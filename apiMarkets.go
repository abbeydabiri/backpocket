package main

import (
	"backpocket/utils"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	marketList    []markets
	marketListMap map[string]int

	marketListMutex    = sync.RWMutex{}
	marketListMapMutex = sync.RWMutex{}
	wsConnMarketsMutex = sync.RWMutex{}

	wsConnMarkets     = make(map[*websocket.Conn]bool)
	wsBroadcastMarket = make(chan interface{}, 10240)
)

type markets struct {
	ID       uint64 `sql:"index"`
	Pair     string `sql:"unique index"`
	Status   string `sql:"index"`
	Exchange string `sql:"index"`

	NumOfTrades,
	Closed int

	BaseAsset, QuoteAsset string

	TakeProfit, StopLoss,

	MinNotional,
	MinQty, MaxQty, StepSize,
	MinPrice, MaxPrice, TickSize,
	Open, Close, High, Low,
	Volume, VolumeQuote,
	LastPrice, Price,

	UpperBand, MiddleBand, LowerBand,

	FirstBid, SecondBid, LastBid,
	FirstAsk, SecondAsk, LastAsk,
	BidQty, BidPrice,
	AskQty, AskPrice,

	PriceChange,
	PriceChangePercent,
	HighPrice, LowPrice,
	RSI float64
}

func getMarket(marketPair, marketExchange string) (market markets) {
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

		var unlockedMarketList []markets
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
				return
			}

			if msg.Pair == "" {
				continue
			}

			switch msg.Action {
			case "restart":

				var unlockedMarketList []markets
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
						updateFields := map[string]bool{"status": true}
						if sqlQuery, sqlParams := sqlTableUpdate(reflect.TypeOf(market), reflect.ValueOf(market), updateFields); len(sqlParams) > 0 {
							utils.SqlDB.Exec(sqlQuery, sqlParams...)
						}
					}
				}()

			case "enable":
				oldMarket := getMarket(msg.Pair, msg.Exchange)
				oldMarket.Status = "enabled"
				updateMarket(oldMarket)
				restartMarkets(msg.Exchange)
				wsBroadcastMarket <- oldMarket
				updateFields := map[string]bool{"status": true}
				if sqlQuery, sqlParams := sqlTableUpdate(reflect.TypeOf(oldMarket), reflect.ValueOf(oldMarket), updateFields); len(sqlParams) > 0 {
					utils.SqlDB.Exec(sqlQuery, sqlParams...)
				}

			case "disable":
				oldMarket := getMarket(msg.Pair, msg.Exchange)
				oldMarket.Status = "disabled"
				updateMarket(oldMarket)
				restartMarkets(msg.Exchange)
				wsBroadcastMarket <- oldMarket
				updateFields := map[string]bool{"status": true}
				if sqlQuery, sqlParams := sqlTableUpdate(reflect.TypeOf(oldMarket), reflect.ValueOf(oldMarket), updateFields); len(sqlParams) > 0 {
					utils.SqlDB.Exec(sqlQuery, sqlParams...)
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
			wsConnMarketsMutex.Lock()
			for wsConn := range wsConnMarkets {
				if err := wsConn.WriteJSON(market); err != nil {
					delete(wsConnMarkets, wsConn)
					wsConn.Close()
				}
			}
			wsConnMarketsMutex.Unlock()
		}
	}()
}

func updateMarket(market markets) {
	if market.Pair == "" {
		return
	}

	// go func() {
	// 	updateFields := map[string]bool{
	// 		"numoftrades": true, "closed": true,

	// 		"minqty": true, "maxqty": true, "stepsize": true,
	// 		"minprice": true, "maxprice": true, "ticksize": true,

	// 		"open": true, "close": true, "high": true, "low": true,
	// 		"volume": true, "volumequote": true,
	// 		"lastprice": true, "price": true,

	// 		"upperband": true, "middleband": true, "lowerband": true,

	// 		"firstbid": true, "secondbid": true, "lastbid": true,
	// 		"firstask": true, "secondask": true, "lastask": true,
	// 		"bidqty": true, "bidprice": true,
	// 		"askqty": true, "askprice": true,

	// 		"pricechange": true, "pricechangepercent": true,
	// 		"highprice": true, "lowprice": true,
	// 	}

	// 	if sqlQuery, sqlParams := sqlTableUpdate(reflect.TypeOf(market), reflect.ValueOf(market), updateFields); len(sqlParams) > 0 {
	// 		utils.SqlDB.Exec(sqlQuery, sqlParams...)
	// 	}
	// }()

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

func dbSetupMarkets() {

	// tablename := ""
	// sqlTable := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	// //create tables if they are missing
	// if utils.SqlDB.Get(&tablename, sqlTable, "markets"); tablename == "" {
	// 	if reflectType := reflect.TypeOf(markets{}); !sqlTableCreate(reflectType) {
	// 		log.Panicf("Table creation failed for table [%s] \n", reflectType.Name())
	// 	}
	// }

	marketListMutex.Lock()
	// marketList = nil
	err := utils.SqlDB.Select(&marketList, "select * from markets order by status, exchange, pair ")
	if err != nil {
		println(err.Error())
	}
	marketListMutex.Unlock()

	marketListMapMutex.Lock()
	marketListMap = make(map[string]int)
	for pairKey, marketPair := range marketList {
		marketListMap[fmt.Sprintf("%s-%s", marketPair.Pair, strings.ToLower(marketPair.Exchange))] = pairKey + 1
	}
	marketListMapMutex.Unlock()

}
