package main

import (
	"backpocket/utils"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/copier"
	"github.com/x2v3/signalr"
	"github.com/x2v3/signalr/hubs"
)

var (
	chanRestartCrex24OrderBookStream = make(chan bool, 10)
)

func crex24OrderBookRest() {

	// for {
	var instrumentList []string
	marketListMutex.RLock()
	for _, market := range marketList {
		if market.Status == "enabled" && market.Exchange == "crex24" {
			instrumentList = append(instrumentList, strings.ToUpper(market.Pair))
		}
	}
	marketListMutex.RUnlock()

	httpClient := http.Client{Timeout: time.Duration(time.Second * 30)}
	for _, instrument := range instrumentList {
		httpRequest, _ := http.NewRequest("GET", crex24RestURL+"/v2/public/orderBook?instrument="+instrument, nil)
		httpResponse, err := httpClient.Do(httpRequest)
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 10)
			continue
		}
		defer httpResponse.Body.Close()

		bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 10)
			continue
		}

		var crex24book struct {
			BuyLevels []struct {
				Price, Volume float64
			}
			SellLevels []struct {
				Price, Volume float64
			}
		}

		if err := json.Unmarshal(bodyBytes, &crex24book); err != nil {
			log.Println(err.Error())
			continue
		}

		orderbook := getOrderbook(instrument, "crex24")

		orderbookMutex.Lock()
		if orderbook.Pair == "" {
			market := getMarket(instrument, "crex24")
			orderbook.QuoteAsset = market.QuoteAsset
			orderbook.BaseAsset = market.BaseAsset
			orderbook.Exchange = market.Exchange
			orderbook.TickSize = market.TickSize
			orderbook.Pair = market.Pair
		}
		orderbook.Asks = nil
		orderbook.Bids = nil

		var prevBidTotal float64
		for _, bid := range crex24book.BuyLevels {
			prevBidTotal = bid.Volume + prevBidTotal
			orderbook.Bids = append(orderbook.Bids,
				bidAskStruct{Price: bid.Price, Quantity: bid.Volume, Total: prevBidTotal})
		}

		var prevAskTotal float64
		for _, ask := range crex24book.SellLevels {
			prevAskTotal = ask.Volume + prevAskTotal
			orderbook.Asks = append(orderbook.Asks,
				bidAskStruct{Price: ask.Price, Quantity: ask.Volume, Total: prevAskTotal})
		}

		updateOrderbook(orderbook)
		orderbookMutex.Unlock()

		var newOrderbook orderbooks
		copier.Copy(&newOrderbook, &orderbook)

		select {
		case wsBroadcastOrderBook <- orderbook:
		default:
		}

		// select {
		// case chanStoplossTakeProfit <- orderbook:
		// default:
		// }

	}

	// 	time.Sleep(time.Second * 15)
	// }
}

func crex24OrderBookStream() {

	// time.Sleep(time.Second * 15)
	// sortSliceStable := func(oldBidAsk []bidAskStruct, reverse bool) []bidAskStruct {
	// 	// sort the oldBids in order of price time priority
	// 	sort.SliceStable(oldBidAsk, func(i, j int) bool {
	// 		if reverse {
	// 			return oldBidAsk[i].Price > oldBidAsk[j].Price
	// 		}

	// 		return oldBidAsk[i].Price < oldBidAsk[j].Price
	// 	})
	// 	// sort the oldBids in order of price time priority
	// 	return oldBidAsk
	// }

	wsRespHandler := func(tickerMessage signalr.Message) {
		if len(tickerMessage.M) == 0 {
			return
		}

		signalrMessage := tickerMessage.M[0]
		crex24OrderBook := crex24StreamBookDepthResp{}
		json.Unmarshal([]byte(signalrMessage.A[1].(string)), &crex24OrderBook)

		if len(crex24OrderBook.B) > 0 && len(crex24OrderBook.S) > 0 {
			log.Println(crex24OrderBook.I, " - New BID LST ", len(crex24OrderBook.B), " - New ASK LST ", len(crex24OrderBook.S))
			log.Println(crex24OrderBook.I, " - UPDATE BID LST ", len(crex24OrderBook.BU), " - UPDATE ASK LST ", len(crex24OrderBook.SU))
		}

		orderbook := getOrderbook(crex24OrderBook.I, "crex24")

		orderbookMutex.Lock()
		if orderbook.Pair == "" {
			market := getMarket(crex24OrderBook.I, "crex24")
			orderbook.QuoteAsset = market.QuoteAsset
			orderbook.BaseAsset = market.BaseAsset
			orderbook.Exchange = market.Exchange
			orderbook.TickSize = market.TickSize
			orderbook.Pair = market.Pair
		}

		// if len(orderbook.Bids) == 0 {
		for _, bid := range crex24OrderBook.B {
			price, _ := strconv.ParseFloat(bid.P, 64)
			quantity, _ := strconv.ParseFloat(bid.V, 64)
			orderbook.Bids = append(orderbook.Bids, bidAskStruct{Price: price, Quantity: quantity})
		}
		// }

		for _, bidUpdate := range crex24OrderBook.BU {

			switch bidUpdate.N {
			case "L":

				price, _ := strconv.ParseFloat(bidUpdate.V.(map[string]interface{})["P"].(string), 64)
				quantity, _ := strconv.ParseFloat(bidUpdate.V.(map[string]interface{})["V"].(string), 64)
				orderbook.Bids = append(orderbook.Bids, bidAskStruct{Price: price, Quantity: quantity})

			case "LU":
				price, _ := strconv.ParseFloat(bidUpdate.V.(map[string]interface{})["P"].(string), 64)
				quantity, _ := strconv.ParseFloat(bidUpdate.V.(map[string]interface{})["V"].(string), 64)

				idFound := -1
				for bidID, oldbid := range orderbook.Bids {
					if oldbid.Price == price {
						idFound = bidID
						oldbid.Quantity = quantity
						orderbook.Bids[bidID] = oldbid
					}
				}
				if idFound == -1 {
					orderbook.Bids = append(orderbook.Bids, bidAskStruct{Price: price, Quantity: quantity})
				}

			case "R":
				price, _ := strconv.ParseFloat(bidUpdate.V.(string), 64)

				idFound := -1
				for bidID, oldbid := range orderbook.Bids {
					if oldbid.Price == price {
						idFound = bidID
						// if len(orderbook.Bids) == 1 {
						// 	orderbook.Bids = nil
						// } else {
						// orderbook.Bids[bidID] = orderbook.Bids[len(orderbook.Bids)-1]
						// orderbook.Bids = orderbook.Bids[:len(orderbook.Bids)-1]
						// }
					}
				}

				if idFound > -1 {
					if len(orderbook.Bids) == 1 {
						orderbook.Bids = nil
					} else {
						orderbook.Bids[idFound] = orderbook.Bids[len(orderbook.Bids)-1]
						orderbook.Bids = orderbook.Bids[:len(orderbook.Bids)-1]
					}
				}

			}
		}

		// sort the oldBids in order of price time priority
		sort.SliceStable(orderbook.Bids, func(i, j int) bool {
			return orderbook.Bids[i].Price > orderbook.Bids[j].Price
		})
		// sort the oldBids in order of price time priority

		// if len(orderbook.Asks) == 0 {
		for _, ask := range crex24OrderBook.S {
			price, _ := strconv.ParseFloat(ask.P, 64)
			quantity, _ := strconv.ParseFloat(ask.V, 64)
			orderbook.Asks = append(orderbook.Asks, bidAskStruct{Price: price, Quantity: quantity})
		}
		// }

		//look for updated bids and handle accordingly
		for _, askUpdate := range crex24OrderBook.SU {

			switch askUpdate.N {
			case "L":
				price, _ := strconv.ParseFloat(askUpdate.V.(map[string]interface{})["P"].(string), 64)
				quantity, _ := strconv.ParseFloat(askUpdate.V.(map[string]interface{})["V"].(string), 64)
				orderbook.Asks = append(orderbook.Asks, bidAskStruct{Price: price, Quantity: quantity})

			case "LU":
				price, _ := strconv.ParseFloat(askUpdate.V.(map[string]interface{})["P"].(string), 64)
				quantity, _ := strconv.ParseFloat(askUpdate.V.(map[string]interface{})["V"].(string), 64)

				idFound := -1
				for askID, oldask := range orderbook.Asks {
					if oldask.Price == price {
						idFound = askID
						oldask.Quantity = quantity
						orderbook.Asks[askID] = oldask
					}
				}
				if idFound == -1 {
					orderbook.Asks = append(orderbook.Asks, bidAskStruct{Price: price, Quantity: quantity})
				}

			case "R":
				price, _ := strconv.ParseFloat(askUpdate.V.(string), 64)

				idFound := -1
				for askID, oldask := range orderbook.Asks {
					if oldask.Price == price {
						idFound = askID
					}
				}

				if idFound > -1 {
					if len(orderbook.Asks) == 1 {
						orderbook.Asks = nil
					} else {
						orderbook.Asks[idFound] = orderbook.Asks[len(orderbook.Asks)-1]
						orderbook.Asks = orderbook.Asks[:len(orderbook.Asks)-1]
					}
				}
			}
		}

		// sort the oldAsks in order of price time priority
		sort.SliceStable(orderbook.Asks, func(i, j int) bool {
			return orderbook.Asks[i].Price < orderbook.Asks[j].Price
		})
		// sort the oldAsks in order of price time priorit

		var prevBidTotal float64
		var prevBidQuoteTotal float64
		for id, bid := range orderbook.Bids {

			bid.QuoteQty = bid.Quantity * bid.Price
			prevBidQuoteTotal += bid.QuoteQty
			bid.QuoteTotal = prevBidQuoteTotal

			prevBidTotal += bid.Quantity
			bid.Total = prevBidTotal

			orderbook.Bids[id] = bid
		}
		orderbook.BidsBaseTotal = prevBidTotal
		orderbook.BidsQuoteTotal = prevBidQuoteTotal
		for id := range orderbook.Bids {
			orderbook.Bids[id].Percentage = utils.TruncateFloat((orderbook.Bids[id].Quantity/orderbook.BidsBaseTotal)*100, 3)
		}

		//
		var prevAskTotal float64
		var prevAskQuoteTotal float64
		for id, ask := range orderbook.Asks {

			ask.QuoteQty = ask.Quantity * ask.Price
			prevAskQuoteTotal += ask.QuoteQty
			ask.QuoteTotal = prevAskQuoteTotal

			prevAskTotal += ask.Quantity
			ask.Total = prevAskTotal

			orderbook.Asks[id] = ask
		}
		orderbook.AsksBaseTotal = prevAskTotal
		orderbook.AsksQuoteTotal = prevAskQuoteTotal
		for id := range orderbook.Asks {
			orderbook.Asks[id].Percentage = utils.TruncateFloat((orderbook.Asks[id].Quantity/orderbook.AsksBaseTotal)*100, 3)
		}

		// orderbook.Asks = oldAsks
		updateOrderbook(orderbook)
		orderbookMutex.Unlock()

		var newOrderbook orderbooks
		copier.Copy(&newOrderbook, &orderbook)

		select {
		case wsBroadcastOrderBook <- orderbook:
		default:
		}

		select {
		case chanStoplossTakeProfit <- orderbook:
		default:
		}

	}
	logIfErr := func(err error) {
		if err != nil {
			log.Println(err)
			return
		}
	}

	channelMethod := "OrderBook"
	chanRestartCrex24OrderBookStream <- true
	for range chanRestartCrex24OrderBookStream {

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
