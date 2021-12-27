package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

var (
	chanRestartBinanceTradeStream = make(chan bool, 10)
)

func binanceTradeStream() {

	var streamParams []string
	marketListMutex.RLock()
	for _, market := range marketList {
		if market.Status == "enabled" && market.Exchange == "binance" {
			streamParams = append(streamParams, strings.ToLower(market.Pair)+"@aggTrade")
		}
	}
	marketListMutex.RUnlock()

	wsResp := binanceStreamTradeResp{}
	bwConn := binanceWSConnect(streamParams)
	if _, _, err := bwConn.ReadMessage(); err != nil {
		log.Println("err ", err.Error())
	}

	//loop through and read all messages received
	for {
		select {
		case <-chanRestartBinanceTradeStream:

			streamParams = nil
			marketListMutex.RLock()
			for _, market := range marketList {
				if market.Status == "enabled" && market.Exchange == "binance" {
					streamParams = append(streamParams, strings.ToLower(market.Pair)+"@aggTrade")
				}
			}
			marketListMutex.RUnlock()

			bwConn.Close()
			bwConn = binanceWSConnect(streamParams)
			if _, _, err := bwConn.ReadMessage(); err != nil {
				log.Println("err ", err.Error())
			}

		default:
		}

		if err := bwConn.ReadJSON(&wsResp); err != nil {
			log.Println("binanceTradeStream bwCon read error:", err)
			time.Sleep(time.Second * 15)

			select {
			case chanRestartBinanceTradeStream <- true:
			default:
			}
			continue
		}

		marketPair := strings.ToUpper(strings.Split(wsResp.Stream, "@")[0])
		if marketPair == "" {
			continue
		}

		market := getMarket(marketPair, "binance")
		if market.Pair == "" {
			continue
		}

		trade := trades{}
		trade.Pair = market.Pair
		trade.Exchange = market.Exchange
		trade.BaseAsset = market.BaseAsset
		trade.QuoteAsset = market.QuoteAsset

		if wsResp.Data.IsBuyerMaker {
			trade.Side = "Buy"
		} else {
			trade.Side = "Sell"
		}
		trade.TradeID = wsResp.Data.TradeID
		trade.Price, _ = strconv.ParseFloat(wsResp.Data.Price, 64)
		trade.Quantity, _ = strconv.ParseFloat(wsResp.Data.Quantity, 64)
		trade.TradeTime = fmt.Sprintf("%s", time.Unix(int64(wsResp.Data.TradeTime)/1000, 0))
		updateTrade(trade)

		// 	select {
		// 	case wsBroadcastTrade <- trade:
		// 	default:
		// 	}
		// }
		market.LastPrice = market.Price
		market.Price = trade.Price
		updateMarket(market)

		// select {
		// case chanStoplossTakeProfit <- getOrderbook(market.Pair, "binance"):
		// default:
		// }
	}
	//loop through and read all messages received
}
