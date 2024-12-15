package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/x2v3/signalr"
	"github.com/x2v3/signalr/hubs"
)

var (
	chanRestartCrex24TradeStream = make(chan bool, 10)
)

func crex24TradeStream() {

	updateMarketTrade := func(instrument, price, volume, side string, tradetime uint, lUpdate bool) {
		market := getMarket(instrument, "crex24")
		if market.Pair == "" {
			return
		}

		trade := trades{}
		trade.Pair = market.Pair
		trade.Exchange = market.Exchange
		trade.BaseAsset = market.BaseAsset
		trade.QuoteAsset = market.QuoteAsset

		if side == "b" {
			trade.Side = "Buy"
		} else {
			trade.Side = "Sell"
		}
		trade.Price, _ = strconv.ParseFloat(price, 64)
		trade.Quantity, _ = strconv.ParseFloat(volume, 64)
		trade.TradeID = tradetime
		trade.TradeTime = fmt.Sprintf("%s", time.Unix(int64(tradetime), 0))
		updateTrade(trade)

		market.NumOfTrades++
		market.Price = trade.Price
		if market.LastPrice == 0 {
			market.LastPrice = market.Price
		}

		updateMarket(market)

		if lUpdate {
			if market.Status == "enabled" {
				// select {
				// case wsBroadcastTrade <- trade:
				// default:
				// }
				select {
				case wsBroadcastMarket <- market:
				default:
				}

				var bidsList []bidAskStruct
				var asksList []bidAskStruct

				bidsList = append(bidsList, bidAskStruct{Price: market.Price})
				asksList = append(asksList, bidAskStruct{Price: market.Price})

				orderbook := orderbooks{
					Pair: market.Pair, Exchange: market.Exchange, TickSize: market.TickSize,
					QuoteAsset: market.QuoteAsset, BaseAsset: market.BaseAsset,
					Asks: asksList,
					Bids: bidsList,
				}
				select {
				case chanStoplossTakeProfit <- orderbook:
				default:
				}
			}
		}
	}

	wsRespHandler := func(tickerMessage signalr.Message) {
		if len(tickerMessage.M) == 0 {
			return
		}

		signalrMessage := tickerMessage.M[0]
		crex24Trade := crex24StreamTradeResp{}
		json.Unmarshal([]byte(signalrMessage.A[1].(string)), &crex24Trade)

		if len(crex24Trade.NT) > 0 {
			log.Println("Received updates of Trade History ", crex24Trade.I)
		}

		for id, tradeResp := range crex24Trade.LST {
			lUpdate := true
			if id > 30 {
				continue
			}
			go updateMarketTrade(crex24Trade.I, tradeResp.P, tradeResp.V, tradeResp.S, tradeResp.T, lUpdate)
			lUpdate = false
		}

		for _, tradeResp := range crex24Trade.NT {
			go updateMarketTrade(crex24Trade.I, tradeResp.P, tradeResp.V, tradeResp.S, tradeResp.T, true)
		}
	}
	logIfErr := func(err error) {
		if err != nil {
			log.Println(err)
			return
		}
	}

	channelMethod := "TradeHistory"
	chanRestartCrex24TradeStream <- true
	for range chanRestartCrex24TradeStream {

		var instrumentList []string
		marketListMutex.RLock()
		for _, market := range marketList {
			if market.Status == "enabled" && market.Exchange == "crex24" {
				instrumentList = append(instrumentList, strings.ToUpper(market.Pair))
			}
		}
		marketListMutex.RUnlock()

		for _, instrument := range instrumentList {
			go func(instrument string) {
				signalRClient := crex24WSConnect()
				logIfErr(signalRClient.Run(wsRespHandler, logIfErr))
				signalRClient.Send(hubs.ClientMsg{
					H: crex24SignalrHub,
					M: "join" + channelMethod,
					A: []interface{}{instrument},
				})
			}(instrument)
		}

	}
}
