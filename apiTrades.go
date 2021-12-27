package main

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	tradeList = make(map[string][]trades)

	tradeListMutex    = sync.RWMutex{}
	wsConnTradesMutex = sync.RWMutex{}

	wsConnTrades     = make(map[*websocket.Conn]bool)
	wsBroadcastTrade = make(chan interface{}, 102400)
)

type trades struct {
	BaseAsset, QuoteAsset, Pair,
	Exchange, Event, Side,
	EventTime, TradeTime string

	TradeID, BuyerOrdID,
	SellerOrdID uint

	Price, Quantity float64
	IsBuyerMaker,
	IsSent bool
}

func wsHandlerTrades(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		//check for enabled markets
		enabledMarketList := make(map[string]bool)
		marketListMutex.RLock()
		for _, market := range marketList {
			if market.Status == "enabled" {
				enabledMarketList[fmt.Sprintf("%s-%s", market.Pair, market.Exchange)] = true
			}
		}
		marketListMutex.RUnlock()
		//check for enabled markets

		var unlockedTradeList []trades
		tradeListMutex.RLock()
		for marketTradeKey := range enabledMarketList {
			unlockedTradeList = append(unlockedTradeList, tradeList[marketTradeKey]...)
		}
		tradeListMutex.RUnlock()

		sort.SliceStable(unlockedTradeList, func(i, j int) bool {
			return unlockedTradeList[i].TradeID < unlockedTradeList[j].TradeID
		})

		for _, trade := range unlockedTradeList {
			wsConn.WriteJSON(trade)
		}

		wsConnTradesMutex.Lock()
		wsConnTrades[wsConn] = true
		wsConnTradesMutex.Unlock()
	}
}

func wsHandlerTradeBroadcast() {

	// loop through enabled markets and send updates down
	go func() {
		limiter := time.Tick(time.Minute)
		var enabledMarketList map[string]bool

		// tickerMarkets := time.Tick(time.Minute * 3)
		// //check for enabled markets
		// marketListMutex.RLock()
		// for _, market := range marketList {
		// 	if market.Status == "enabled" {
		// 		enabledMarketList[fmt.Sprintf("%s-%s", market.Pair, market.Exchange)] = true
		// 	}
		// }
		// marketListMutex.RUnlock()
		// //check for enabled markets

		for {
			<-limiter

			//check for enabled markets
			enabledMarketList = make(map[string]bool)
			marketListMutex.RLock()
			for _, market := range marketList {
				if market.Status == "enabled" {
					enabledMarketList[fmt.Sprintf("%s-%s", market.Pair, market.Exchange)] = true
				}
			}
			marketListMutex.RUnlock()

			var unlockedTradeList []trades
			tradeListMutex.RLock()
			for marketTradeKey := range enabledMarketList {
				unlockedTradeList = append(unlockedTradeList, tradeList[marketTradeKey]...)
			}
			tradeListMutex.RUnlock()

			sort.SliceStable(unlockedTradeList, func(i, j int) bool {
				return unlockedTradeList[i].TradeID < unlockedTradeList[j].TradeID
			})

			for _, trade := range unlockedTradeList {
				if trade.IsSent {
					continue
				}
				select {
				case wsBroadcastTrade <- trade:
				default:
				}
				trade.IsSent = true
				updateTrade(trade)
			}
		}
	}()
	// loop through enabled markets and send updates down

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			wsConnTradesMutex.Lock()
			for wsConn := range wsConnTrades {
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					delete(wsConnTrades, wsConn)
					wsConn.Close()
				}
			}
			wsConnTradesMutex.Unlock()
		}
	}()

	go func() {
		for trade := range wsBroadcastTrade {
			wsConnTradesMutex.Lock()
			for wsConn := range wsConnTrades {
				if err := wsConn.WriteJSON(trade); err != nil {
					delete(wsConnTrades, wsConn)
					wsConn.Close()
				}
			}
			wsConnTradesMutex.Unlock()
		}
	}()
}

func clearTrades(pair, exchange string) {
	if pair == "" && exchange == "" {
		return
	}
	tradeKey := fmt.Sprintf("%s-%s", pair, exchange)

	tradeListMutex.Lock()
	tradeList[tradeKey] = []trades{}
	tradeListMutex.Unlock()
}

func updateTrade(trade trades) {
	if trade.Pair == "" && trade.Exchange == "" {
		return
	}
	tradeKey := fmt.Sprintf("%s-%s", trade.Pair, trade.Exchange)

	// if pairKey == 0 {
	tradeListMutex.Lock()
	if tradeList[tradeKey] == nil {
		tradeList[tradeKey] = []trades{}
	}

	tradeList[tradeKey] = append(tradeList[tradeKey], trade)
	if len(tradeList[tradeKey]) > 50 {
		tradeList[tradeKey] = tradeList[tradeKey][1:]
	}
	tradeListMutex.Unlock()
}
