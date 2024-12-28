package main

import (
	"backpocket/models"
	"backpocket/utils"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	marketList    []models.Market
	marketListMap = make(map[string]int)

	marketListMutex    = sync.RWMutex{}
	marketListMapMutex = sync.RWMutex{}
	wsConnMarketsMutex = sync.RWMutex{}

	wsConnMarkets     = make(map[*websocket.Conn]bool)
	wsBroadcastMarket = make(chan models.Market, 10240)
)

func getMarket(marketPair, marketExchange string) (market models.Market) {
	marketKey := 0
	marketListMapMutex.RLock()
	for pair, key := range marketListMap {
		if pair == fmt.Sprintf("%s-%s", marketPair, strings.ToLower(marketExchange)) {
			marketKey = key
		}
	}
	marketListMapMutex.RUnlock()

	marketListMutex.RLock()
	if marketKey > 0 && len(marketList) > (marketKey-1) {
		market = marketList[marketKey-1]
		// copier.Copy(&market, marketList[marketKey-1])
	}
	marketListMutex.RUnlock()
	return
}

func restartMarkets(exchange string) {
	// switch exchange {
	// case "crex24":
	// 	select {
	// 	case chanRestartCrex24Market24HRTickerStream <- true:
	// 	default:
	// 	}

	// 	select {
	// 	case chanRestartCrex24TradeStream <- true:
	// 	default:
	// 	}

	// 	select {
	// 	case chanRestartCrex24OrderBookStream <- true:
	// 	default:
	// 	}

	// case "binance":

	select {
	case chanRestartBinanceAssetStream <- true:
	default:
	}

	select {
	case chanRestartBinanceOrderStream <- true:
	default:
	}

	select {
	case chanRestartBinanceTradeStream <- true:
	default:
	}

	select {
	case chanRestartBinanceOrderBookStream <- true:
	default:
	}

	select {
	case chanRestartBinanceOHLCVMarketStream <- true:
	default:
	}
	// }

}

func wsHandlerMarkets(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		var unlockedMarketList []models.Market
		marketListMutex.RLock()
		for _, market := range marketList {
			unlockedMarketList = append(unlockedMarketList, market)
		}
		marketListMutex.RUnlock()

		wsConnMarketsMutex.Lock()
		for _, market := range unlockedMarketList {
			wsConn.WriteJSON(market)
		}
		wsConnMarkets[wsConn] = true
		wsConnMarketsMutex.Unlock()

		for {

			var msg struct {
				Action, Pair,
				Exchange string
			}

			if err := wsConn.ReadJSON(&msg); err != nil {
				log.Println("wsConn.ReadJSON: ", err)
				// wsConn.Close()
				return
			}

			if msg.Pair == "" {
				continue
			}

			switch msg.Action {
			/*
				case "restart":

					var unlockedMarketList []models.Market
					marketListMutex.Lock()
					for id, market := range marketList {
						marketList[id].Status = "disabled"
						unlockedMarketList = append(unlockedMarketList, market)
					}
					marketListMutex.Unlock()
					restartMarkets(msg.Exchange)

					go func() {
						for _, market := range unlockedMarketList {
							wsBroadcastMarket <- market
							if err := utils.SqlDB.Model(&market).Where("pair = ? and exchange = ?", market.Pair, market.Exchange).Updates(map[string]interface{}{"status": "disabled"}).Error; err != nil {
								log.Println(err.Error())
							}
						}
					}()
			*/
			case "enable":
				oldMarket := getMarket(msg.Pair, msg.Exchange)
				oldMarket.Status = "enabled"
				updateMarket(oldMarket)
				restartMarkets(msg.Exchange)
				wsBroadcastMarket <- oldMarket
				if err := utils.SqlDB.Model(&oldMarket).Where("pair = ? and exchange = ?", oldMarket.Pair, oldMarket.Exchange).Updates(
					map[string]interface{}{"status": "enabled"}).Error; err != nil {
					log.Println(err.Error())
				}

			case "disable":
				oldMarket := getMarket(msg.Pair, msg.Exchange)
				oldMarket.Status = "disabled"
				updateMarket(oldMarket)
				restartMarkets(msg.Exchange)
				wsBroadcastMarket <- oldMarket
				if err := utils.SqlDB.Model(&oldMarket).Where("pair = ? and exchange = ?", oldMarket.Pair, oldMarket.Exchange).Updates(
					map[string]interface{}{"status": "disabled"}).Error; err != nil {
					log.Println(err.Error())
				}
			}
		}
	}
}

func wsHandlerMarketBroadcast() {
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			wsConnMarketsMutex.Lock()
			for wsConn := range wsConnMarkets {
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					delete(wsConnMarkets, wsConn)
					wsConn.Close()
				}
			}
			wsConnMarketsMutex.Unlock()
		}
	}()

	go func() {
		for market := range wsBroadcastMarket {
			if market.Pair == "" {
				continue
			}
			wsConnMarketsMutex.Lock()
			for wsConn := range wsConnMarkets {
				if err := wsConn.WriteJSON(market); err != nil {
					if err.Error() != "websocket: close sent" {
						log.Printf("market: %+v", market)
						log.Println("error writing market json: ", err.Error())
					} else {
						delete(wsConnMarkets, wsConn)
						wsConn.Close()
					}
				}
			}
			wsConnMarketsMutex.Unlock()
		}
	}()
}

func updateMarket(market models.Market) {
	if market.Pair == "" {
		return
	}

	pairKey := 0
	marketListMapMutex.RLock()
	for pair, key := range marketListMap {
		if pair == fmt.Sprintf("%s-%s", market.Pair, strings.ToLower(market.Exchange)) {
			pairKey = key
		}
	}
	marketListMapMutex.RUnlock()

	if pairKey == 0 {
		marketListMutex.Lock()
		marketList = append(marketList, market)
		pairKey = len(marketList)
		marketListMutex.Unlock()

		marketListMapMutex.Lock()
		marketListMap[fmt.Sprintf("%s-%s", market.Pair, strings.ToLower(market.Exchange))] = pairKey
		marketListMapMutex.Unlock()
	} else {
		marketListMutex.Lock()
		marketList[pairKey-1] = market
		marketListMutex.Unlock()
	}
}

func LoadMarketsFromDB() {
	var markets []models.Market
	if err := utils.SqlDB.Find(&markets).Error; err != nil {
		log.Println(err.Error())
		return
	}

	for _, market := range markets {
		updateMarket(market)
	}
}
