package main

import (
	"backpocket/utils"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"
)

var (
	chanRestartBinanceOrderBookStream = make(chan bool, 10)
)

func binanceOrderBookStream() {
	log.Println("Connecting binanceOrderBookStream")
	var streamParams []string
	marketListMutex.RLock()
	for _, market := range marketList {
		if market.Status == "enabled" && market.Exchange == "binance" {
			streamParams = append(streamParams, strings.ToLower(market.Pair)+"@depth20@1000ms")
		}
	}
	marketListMutex.RUnlock()

	wsResp := binanceStreamBookDepthResp{}
	bwConn := binanceWSConnect(streamParams)
	if _, _, err := bwConn.ReadMessage(); err != nil {
		log.Println("err ", err.Error())
	}
	log.Println("Connected binanceOrderBookStream")

	//loop through and read all messages received
	for {

		select {
		case <-chanRestartBinanceOrderBookStream:
			streamParams = nil
			marketListMutex.RLock()
			for _, market := range marketList {
				if market.Status == "enabled" && market.Exchange == "binance" {
					streamParams = append(streamParams, strings.ToLower(market.Pair)+"@depth20@1000ms")
				}
			}
			marketListMutex.RUnlock()

			bwConn.Close()
			bwConn = binanceWSConnect(streamParams)
			if _, _, err := bwConn.ReadMessage(); err != nil {
				log.Println("err ", err.Error())
			}
			log.Println("Restarted binanceOrderBookStream")

		default:
		}

		_, wsRespBytes, _ := bwConn.ReadMessage()
		if err := json.Unmarshal(wsRespBytes, &wsResp); err != nil {
			// if err := bwConn.ReadJSON(&wsResp); err != nil {
			log.Println("binanceOrderBookStream bwCon read error:", err)
			log.Println("wsRespBytes:", string(wsRespBytes))
			time.Sleep(time.Second * 10)

			select {
			case chanRestartBinanceOrderBookStream <- true:
			default:
			}
			continue
		}

		marketPair := strings.ToUpper(strings.Split(wsResp.Stream, "@")[0])
		if marketPair == "" {
			continue
		}

		market := getMarket(marketPair, "binance")
		orderbook := getOrderbook(marketPair, "binance")

		orderbookMutex.Lock()
		if orderbook.Pair == "" {
			orderbook.QuoteAsset = market.QuoteAsset
			orderbook.BaseAsset = market.BaseAsset
			orderbook.Exchange = market.Exchange
			orderbook.TickSize = market.TickSize
			orderbook.Pair = market.Pair
		}

		if orderbook.Pair == "" {
			continue
		}

		orderbook.Bids = nil
		var prevBidTotal float64
		var bidBaseTotal float64
		for _, bid := range wsResp.Data.Bids {
			price, _ := strconv.ParseFloat(bid[0], 64)
			quantity, _ := strconv.ParseFloat(bid[1], 64)
			orderbook.Bids = append(orderbook.Bids, bidAskStruct{Price: price, Quantity: quantity, Total: prevBidTotal + quantity})
			prevBidTotal += quantity
			bidBaseTotal += quantity * price
		}
		orderbook.BidsBaseTotal = bidBaseTotal
		orderbook.BidsQuoteTotal = prevBidTotal
		// bidsQuoteAverage := orderbook.BidsQuoteTotal / float64(len(orderbook.Bids))
		for id := range orderbook.Bids {
			orderbook.Bids[id].Percentage = utils.TruncateFloat((orderbook.Bids[id].Total/orderbook.BidsQuoteTotal)*100.00, 3)
		}

		//

		orderbook.Asks = nil
		var prevAskTotal float64
		var askBaseTotal float64
		for _, ask := range wsResp.Data.Asks {
			price, _ := strconv.ParseFloat(ask[0], 64)
			quantity, _ := strconv.ParseFloat(ask[1], 64)
			orderbook.Asks = append(orderbook.Asks, bidAskStruct{Price: price, Quantity: quantity, Total: prevAskTotal + quantity})
			prevAskTotal += quantity
			askBaseTotal += quantity * price
		}
		orderbook.AsksBaseTotal = askBaseTotal
		orderbook.AsksQuoteTotal = prevAskTotal
		// asksQuoteAverage := orderbook.AsksQuoteTotal / float64(len(orderbook.Asks))
		for id := range orderbook.Asks {
			orderbook.Asks[id].Percentage = utils.TruncateFloat((orderbook.Asks[id].Total/orderbook.AsksQuoteTotal)*100.00, 3)
		}

		updateOrderbook(orderbook)
		orderbookMutex.Unlock()

		select {
		case wsBroadcastMarket <- market:
		default:
		}

		// select {
		// case wsBroadcastOrderBook <- orderbook:
		// default:
		// }

		select {
		case chanStoplossTakeProfit <- orderbook:
		default:
		}

	}
	//loop through and read all messages received
}
