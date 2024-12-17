package main

import (
	"backpocket/utils"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	strategyList    []strategys
	strategyListMap = make(map[string]int)

	marketRSIBands = make(map[string][]float64)
	bollingerBands = make(map[string][]float64)

	marketRSIBandsMutex = sync.RWMutex{}
	bollingerBandsMutex = sync.RWMutex{}

	strategyListMutex    = sync.RWMutex{}
	strategyListMapMutex = sync.RWMutex{}
	wsConnStrategysMutex = sync.RWMutex{}

	wsConnStrategys     = make(map[*websocket.Conn]bool)
	wsBroadcastStrategy = make(chan interface{}, 10240)

	chanStoplossTakeProfit = make(chan orderbooks, 10240)
)

type strategys struct {
	ID       uint64 `sql:"index"`
	Status   string `sql:"index"`
	Exchange string `sql:"index"`
	Percent  float64
	Symbol   string `sql:"index"`

	// Account     string `sql:"unique index"`
}

func wsHandlerStrategys(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		wsConnStrategysMutex.Lock()
		wsConnStrategys[wsConn] = true
		wsConnStrategysMutex.Unlock()

	}
}

func wsHandlerStrategyBroadcast() {
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			wsConnStrategysMutex.Lock()
			for wsConn := range wsConnStrategys {
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					delete(wsConnStrategys, wsConn)
					wsConn.Close()
				}
			}
			wsConnStrategysMutex.Unlock()
		}
	}()

	go func() {

		for strategy := range wsBroadcastStrategy {
			wsConnStrategysMutex.Lock()
			for wsConn := range wsConnStrategys {
				if err := wsConn.WriteJSON(strategy); err != nil {
					delete(wsConnStrategys, wsConn)
					wsConn.Close()
				}
			}
			wsConnStrategysMutex.Unlock()

		}
	}()
}

func apiStrategyStopLossTakeProfit() {

	for orderbook := range chanStoplossTakeProfit {

		orderbookPair := ""
		var orderbookBidPrice, orderbookAskPrice, orderBookBidsBaseTotal, orderBookAsksBaseTotal float64

		orderbookMutex.RLock()
		orderbookPair = orderbook.Pair
		if len(orderbook.Bids) > 0 {
			orderbookBidPrice = orderbook.Bids[0].Price
		}
		if len(orderbook.Asks) > 0 {
			orderbookAskPrice = orderbook.Asks[0].Price
		}
		orderBookBidsBaseTotal = orderbook.BidsBaseTotal
		orderBookAsksBaseTotal = orderbook.AsksBaseTotal
		orderbookMutex.RUnlock()

		if orderbookBidPrice == 0 || orderbookAskPrice == 0 {
			log.Println("Skipping Empty orderbooks: ")
			log.Printf("orderbook.Asks %+v \n", orderbook.Asks)
			log.Printf("orderbook.Bids %+v \n", orderbook.Bids)
			log.Printf("orderbook %+v \n", orderbook)
			continue
		}

		var oldOrderList []orders
		var oldPriceList []float64

		//do a mutex RLock loop through orders
		orderListMutex.RLock()
		for _, oldOrder := range orderList {

			if oldOrder.Pair != orderbookPair {
				continue
			}

			//check if order was FILLED
			if oldOrder.Status != "FILLED" {
				continue
			}

			if oldOrder.RefEnabled <= 0 {
				continue
			}

			if oldOrder.Takeprofit <= 0 && oldOrder.Stoploss <= 0 {
				continue
			}

			if len(oldOrder.RefSide) > 0 {
				continue
			}

			if len(oldOrder.RefTripped) > 0 {
				continue
			}

			market := getMarket(oldOrder.Pair, oldOrder.Exchange)
			switch oldOrder.Side {
			case "BUY": //CHECK TO SELL BACK
				oldOrder.RefSide = "SELL"

				// if market.Close < market.Open && market.Price < market.LastPrice {
				// if market.Price < market.LastPrice && market.Close > market.Open {
				// if market.Close < market.Open && market.Price < market.LastPrice && market.LastPrice < market.LowerBand {

				/*
					*Best Time to Sell:*

					Sell at the Upper Band:
						When the Current Price touches or exceeds the Upper Bollinger Band, it signals potential overbought conditions.

					Volume Confirmation:
						Check for decreasing volume or signs of a reversal (e.g., red candles forming after hitting the Upper Band).

					Overbought Signals:
						Use RSI > 70 to confirm overbought conditions.
				*/

				// if market.Pair == "XRPUSDT" && market.RSI > 0 {
				// 	fmt.Printf("\n\n\n")
				// 	fmt.Println("market: ", market.Pair, " - CHECK TO SELL BACK - ",
				// 		market.Close >= market.UpperBand && market.Close < market.Open && market.Price < market.LastPrice && orderBookBidsBaseTotal < orderBookAsksBaseTotal && market.RSI > float64(70))

				// 	fmt.Println("market.Close >= market.UpperBand && market.Close < market.Open && market.Price < market.LastPrice && orderBookBidsBaseTotal < orderBookAsksBaseTotal && market.RSI > float64(70)")
				// 	fmt.Println(market.Close, " > ", market.UpperBand, " && ", market.Close, " < ", market.Open, " && ", market.Price, " < ", market.LastPrice, " && ", orderBookBidsBaseTotal, " < ", orderBookAsksBaseTotal, " && ", market.RSI, " > ", float64(70))
				// 	fmt.Println(market.Close >= market.UpperBand, market.Close < market.Open, market.Price < market.LastPrice, orderBookBidsBaseTotal < orderBookAsksBaseTotal, market.RSI > float64(70))
				// }

				if market.Close >= market.UpperBand && market.Close < market.Open && market.Price < market.LastPrice && orderBookBidsBaseTotal < orderBookAsksBaseTotal && market.RSI > float64(75) {
					newTakeprofit := utils.TruncateFloat(((orderbookBidPrice-oldOrder.Price)/oldOrder.Price)*100, 3)
					// log.Println("TRIGGER SELL: ", oldOrder.OrderID, " [-] Market: ", market.Pair, " [-] newTakeprofit: ", newTakeprofit, " [-] oldTakeprofit: ", oldOrder.Takeprofit)

					if newTakeprofit >= oldOrder.Takeprofit && oldOrder.Takeprofit > 0 {
						oldOrder.RefTripped = fmt.Sprintf("> %.3f%% TP: %.8f @ RSI %.2f", newTakeprofit, orderbookBidPrice, market.RSI)
						oldPriceList = append(oldPriceList, orderbookBidPrice)
						oldOrderList = append(oldOrderList, oldOrder)
					}
				}

				newStoploss := utils.TruncateFloat(((oldOrder.Price-orderbookBidPrice)/oldOrder.Price)*100, 3)
				if newStoploss >= oldOrder.Stoploss && oldOrder.Stoploss > 0 {
					oldOrder.RefTripped = fmt.Sprintf("< %.3f%% SL: %.8f", newStoploss, orderbookBidPrice)
					oldPriceList = append(oldPriceList, orderbookBidPrice)
					oldOrderList = append(oldOrderList, oldOrder)
				}

			case "SELL": //CHECK TO BUY BACK
				oldOrder.RefSide = "BUY"

				// if market.Close > market.Open && market.Price > market.LastPrice {
				// if market.Price > market.LastPrice && market.Close < market.Open {
				// if market.Close > market.Open && market.Price > market.LastPrice && market.LastPrice > market.MiddleBand {

				/*
					*Best Time to Buy:*

					Buy at the Lower Band:
						When the Current Price touches or dips below the Lower Bollinger Band, it signals that the price is potentially oversold.
						Look for confirmation that the price is starting to rebound (e.g., a green candle forming on the next tick).

					Volume Confirmation:
						High volume on the bounce indicates strong buying interest.

					Oversold Signals:
						Use a complementary indicator like RSI (Relative Strength Index) to confirm oversold conditions (e.g., RSI < 30).

					Avoid Buying in a Downtrend:
						If the price continues to hug or break through the Lower Band, wait until it stabilizes above the band before entering.
				*/

				// if market.Pair == "XRPUSDT" && market.RSI > 0 {
				// 	fmt.Printf("\n\n\n")
				// 	fmt.Println("market: ", market.Pair, " - CHECK TO BUY BACK - ",
				// 		market.Close <= market.LowerBand && market.Close > market.Open && market.Price > market.LastPrice && orderBookBidsBaseTotal > orderBookAsksBaseTotal && market.RSI < float64(30))

				// 	fmt.Println("market.Close <= market.LowerBand && market.Close > market.Open && market.Price > market.LastPrice && orderBookBidsBaseTotal > orderBookAsksBaseTotal && market.RSI < float64(30)")
				// 	fmt.Println(market.Close, " <= ", market.LowerBand, " && ", market.Close, " > ", market.Open, " && ", market.Price, " > ", market.LastPrice, " && ", orderBookBidsBaseTotal, " > ", orderBookAsksBaseTotal, " && ", market.RSI, " < ", float64(30))
				// 	fmt.Println(market.Close <= market.LowerBand, market.Close > market.Open, market.Price > market.LastPrice, orderBookBidsBaseTotal > orderBookAsksBaseTotal, market.RSI < float64(30))
				// }

				if market.Close <= market.LowerBand && market.Close > market.Open && market.Price > market.LastPrice && orderBookBidsBaseTotal > orderBookAsksBaseTotal && market.RSI < float64(25) {
					newTakeprofit := utils.TruncateFloat(((oldOrder.Price-orderbookAskPrice)/oldOrder.Price)*100, 3)
					// log.Println("TRIGGER BUY: ", oldOrder.OrderID, " [-] Market: ", market.Pair, " [-] newTakeprofit: ", newTakeprofit, " [-] oldTakeprofit: ", oldOrder.Takeprofit)

					if newTakeprofit >= oldOrder.Takeprofit && oldOrder.Takeprofit > 0 {
						oldOrder.RefTripped = fmt.Sprintf("< %.3f%% TP: %.8f @ RSI %.2f", newTakeprofit, orderbookAskPrice, market.RSI)
						oldPriceList = append(oldPriceList, orderbookAskPrice)
						oldOrderList = append(oldOrderList, oldOrder)
					}
				}

				//experiment buying is always good so far as we sell higher, therefore lets use same take profit for buying higher or lower
				newStoploss := utils.TruncateFloat(((orderbookAskPrice-oldOrder.Price)/oldOrder.Price)*100, 3)
				// if newStoploss >= oldOrder.Stoploss && oldOrder.Stoploss > 0 {
				if newStoploss >= oldOrder.Takeprofit && oldOrder.Stoploss > 0 && oldOrder.Takeprofit > 0 {
					oldOrder.RefTripped = fmt.Sprintf("> %.3f%% SL: %.8f", newStoploss, orderbookAskPrice)
					oldPriceList = append(oldPriceList, orderbookAskPrice)
					oldOrderList = append(oldOrderList, oldOrder)
				}
			}
		}
		orderListMutex.RUnlock()

		for keyID, oldOrder := range oldOrderList {
			updateOrder(oldOrder)

			newOrder := orders{}
			newOrder.Pair = oldOrder.Pair
			newOrder.Side = oldOrder.RefSide
			newOrder.AutoRepeat = oldOrder.AutoRepeat
			newOrder.Price = oldPriceList[keyID]
			newOrder.Quantity = oldOrder.Quantity
			newOrder.Exchange = oldOrder.Exchange
			newOrder.RefOrderID = oldOrder.OrderID

			if newOrder.AutoRepeat > 0 {
				newOrder.AutoRepeat = oldOrder.AutoRepeat - 1
				newOrder.AutoRepeatID = oldOrder.RefOrderID

				if newOrder.Side == "BUY" {
					newOrder.Stoploss = utils.TruncateFloat(oldOrder.Stoploss, 3)
					newOrder.Takeprofit = utils.TruncateFloat(oldOrder.Takeprofit, 3)
				} else {
					newOrder.Stoploss = utils.TruncateFloat(oldOrder.Stoploss, 3)
					newOrder.Takeprofit = utils.TruncateFloat(oldOrder.Takeprofit, 3)
				}
			}

			switch newOrder.Exchange {
			default:
				binanceOrderCreate(newOrder.Pair, newOrder.Side, strconv.FormatFloat(newOrder.Price, 'f', -1, 64), strconv.FormatFloat(newOrder.Quantity, 'f', -1, 64), newOrder.Stoploss, newOrder.Takeprofit, newOrder.AutoRepeat, newOrder.RefOrderID)
			case "crex24":
				crex24OrderCreate(newOrder.Pair, newOrder.Side, newOrder.Price, newOrder.Quantity, newOrder.Stoploss, newOrder.Takeprofit, newOrder.AutoRepeat, newOrder.RefOrderID)
			}
		}

	}
}

func calculateRSIBands(market *markets) {

	marketRSIBandsMutex.RLock()
	rsiBands := marketRSIBands[market.Pair]
	marketRSIBandsMutex.RUnlock()

	//calculate RSI
	var rsiGain float64
	var rsiLoss float64

	for x, closePrice := range rsiBands {
		if x == 0 {
			continue
		}

		switch {
		case closePrice > rsiBands[x-1]:
			rsiGain += closePrice - rsiBands[x-1]
		case closePrice < rsiBands[x-1]:
			rsiLoss += rsiBands[x-1] - closePrice
		}

	}

	rsiValue := (rsiGain / float64(len(rsiBands))) / (rsiLoss / float64(len(rsiBands)))
	market.RSI = utils.TruncateFloat(100-(100/(1+rsiValue)), 2)
	//calculate RSI
}

func calculateBollingerBands(market *markets) {

	bollingerBandsMutex.RLock()
	marketBands := bollingerBands[market.Pair]
	bollingerBandsMutex.RUnlock()

	if len(marketBands) < 3 {
		return
	}

	//Calculate the simple moving average:
	var sumClosePrice float64
	for _, closePrice := range marketBands {
		sumClosePrice += closePrice
		market.MiddleBand = sumClosePrice / float64(len(marketBands))
	}
	//Calculate the simple moving average:

	//Next, for each close price, subtract average from each close price and square this value
	//e.g 25.5 - 26.6 =	-1.1	squared =	1.21
	//	  26.75 - 26.6 =	0.15	squared =	0.023
	var sumAverageClose float64
	for _, closePrice := range marketBands {
		closeAvgDiff := closePrice - market.MiddleBand
		sumAverageClose += closeAvgDiff * closeAvgDiff
	}
	//Add the above calculated values, divide by size of closes available,
	//and then get the square root of this value to get the deviation value:
	sumAverageCloseSquared := math.Sqrt(sumAverageClose / float64(len(marketBands)))

	market.UpperBand = market.MiddleBand + (2 * sumAverageCloseSquared)
	market.LowerBand = market.MiddleBand - (2 * sumAverageCloseSquared)

}

func dbStupStrategys() {

	// tablename := ""
	// sqlTable := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	// //create tables if they are missing
	// if utils.SqlDB.Get(&tablename, sqlTable, "strategys"); tablename == "" {
	// 	if reflectType := reflect.TypeOf(strategys{}); !sqlTableCreate(reflectType) {
	// 		log.Panicf("Table creation failed for table [%s] \n", reflectType.Name())
	// 	}

	// 	var defaultStrategys []strategys
	// 	defaultStrategys = append(defaultStrategys, strategys{Symbol: "BTC", Percent: 50, Status: "enabled", Exchange: "binance"})
	// 	defaultStrategys = append(defaultStrategys, strategys{Symbol: "ETH", Percent: 50, Status: "enabled", Exchange: "binance"})

	// 	for _, strategy := range defaultStrategys {
	// 		utils.SqlDB.Exec("INSERT INTO strategys (symbol, percent, status, exchange) VALUES (?, ?, ?, ?)",
	// 			strategy.Symbol, strategy.Percent, strategy.Status, strategy.Exchange)
	// 	}
	// 	// <-time.After(time.Second)
	// }

	strategyListMutex.Lock()
	utils.SqlDB.Select(&strategyList, "select * from strategys order by status, exchange, symbold")
	strategyListMutex.Unlock()

	strategyListMapMutex.Lock()
	strategyListMap = make(map[string]int)
	for id, strategy := range strategyList {
		strategyListMap[strings.ToUpper(strategy.Symbol)] = id + 1
	}
	strategyListMapMutex.Unlock()
}
