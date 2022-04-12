package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	orderbookList    []orderbooks
	orderbookListMap = make(map[string]int)

	orderbookMutex        = sync.RWMutex{}
	orderbookListMutex    = sync.RWMutex{}
	orderbookListMapMutex = sync.RWMutex{}
	wsConnOrderbooksMutex = sync.RWMutex{}

	wsConnOrderbooks     = make(map[*websocket.Conn]bool)
	wsBroadcastOrderBook = make(chan interface{}, 10240)
)

type bidAskStruct struct {
	Price, Quantity,
	Percentage,
	Total float64
}

// func (ba bidASkStruct) MarshalJSON() ([]byte, error) {

// 	ba.Price =

// 	return json.Marshal(ba)
// }

type orderbooks struct {
	BaseAsset, QuoteAsset,
	Pair, Exchange string

	BidsQuoteTotal, AsksQuoteTotal,
	BidsBaseTotal, AsksBaseTotal,
	TickSize float64

	Bids, Asks []bidAskStruct
}

func getOrderbook(orderbookPair, orderbookExchange string) (orderbook orderbooks) {
	orderbookKey := 0
	orderbookListMapMutex.RLock()
	for pair, key := range orderbookListMap {
		if pair == fmt.Sprintf("%s-%s", orderbookPair, strings.ToLower(orderbookExchange)) {
			orderbookKey = key
		}
	}
	orderbookListMapMutex.RUnlock()

	orderbookListMutex.RLock()
	if orderbookKey > 0 && len(orderbookList) > (orderbookKey-1) {
		orderbook = orderbookList[orderbookKey-1]
		// copier.Copy(&orderbook, orderbookList[orderbookKey-1])
	}
	orderbookListMutex.RUnlock()
	return orderbook
}

func wsHandlerOrderbooks(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		var unlockedOrderbookList []orderbooks
		orderbookListMutex.RLock()
		for _, orderbook := range orderbookList {
			unlockedOrderbookList = append(unlockedOrderbookList, orderbook)
		}
		orderbookListMutex.RUnlock()

		for _, orderbook := range unlockedOrderbookList {
			wsConn.WriteJSON(orderbook)
		}

		wsConnOrderbooksMutex.Lock()
		wsConnOrderbooks[wsConn] = true
		wsConnOrderbooksMutex.Unlock()

		/*


			for {

				var msg struct {
					Action, Pair,
					Exchange string
				}

				if err := wsConn.ReadJSON(&msg); err != nil {
					return
				}

				switch msg.Action {
				case "restart":
					// switch msg.Exchange {
					// case "crex24":
					// 	select {
					// 	case chanRestartCrex24TradeStream <- true:
					// 	default:
					// 	}

					// 	select {
					// 	case chanRestartCrex24OrderBookStream <- true:
					// 	default:
					// 	}

					// 	select {
					// 	case chanRestartCrex24OrderBookStream <- true:
					// 	default:
					// 	}

					// case "binance":
					// 	select {
					// 	case chanRestartBinanceTradeStream <- true:
					// 	default:
					// 	}

					// 	select {
					// 	case chanRestartBinanceOrderBookStream <- true:
					// 	default:
					// 	}

					// 	select {
					// 	case chanRestartBinanceOrderBookStream <- true:
					// 	default:
					// 	}
					// }
				}
			}
		*/
	}
}

func wsHandlerOrderbookBroadcast() {
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			wsConnOrderbooksMutex.Lock()
			for wsConn := range wsConnOrderbooks {
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					delete(wsConnOrderbooks, wsConn)
					wsConn.Close()
				}
			}
			wsConnOrderbooksMutex.Unlock()
		}
	}()

	go func() {
		for orderbook := range wsBroadcastOrderBook {
			wsConnOrderbooksMutex.Lock()
			for wsConn := range wsConnOrderbooks {
				orderbookMutex.RLock()
				if err := wsConn.WriteJSON(orderbook); err != nil {
					delete(wsConnOrderbooks, wsConn)
					wsConn.Close()
				}
				orderbookMutex.RUnlock()
			}
			wsConnOrderbooksMutex.Unlock()
		}
	}()
}

func updateOrderbook(orderbook orderbooks) {
	if orderbook.Pair == "" {
		return
	}

	pairKey := 0
	orderbookListMapMutex.RLock()
	for pair, key := range orderbookListMap {
		if pair == fmt.Sprintf("%s-%s", orderbook.Pair, strings.ToLower(orderbook.Exchange)) {
			pairKey = key
		}
	}
	orderbookListMapMutex.RUnlock()

	if pairKey == 0 {
		orderbookListMutex.Lock()
		orderbookList = append(orderbookList, orderbook)
		pairKey = len(orderbookList)
		orderbookListMutex.Unlock()

		orderbookListMapMutex.Lock()
		orderbookListMap[fmt.Sprintf("%s-%s", orderbook.Pair, strings.ToLower(orderbook.Exchange))] = pairKey
		orderbookListMapMutex.Unlock()
	} else {
		orderbookListMutex.Lock()
		orderbookList[pairKey-1] = orderbook
		orderbookListMutex.Unlock()
	}
}
