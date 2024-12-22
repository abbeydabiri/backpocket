package main

import (
	"backpocket/models"
	"backpocket/utils"
	"fmt"
	"log"
	"math"
	"strconv"
	"sync"
)

var (
	bollingerBands  = make(map[string][]float64)
	marketRSIPrices = make(map[string][]float64)
	marketRSIValues = make(map[string][]float64)

	marketRSIValuesMutex = sync.RWMutex{}
	marketRSIPricesMutex = sync.RWMutex{}
	bollingerBandsMutex  = sync.RWMutex{}

	chanStoplossTakeProfit = make(chan orderbooks, 10240)
)

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

		var oldOrderList []models.Order
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

				//calculate percentage difference between orderBookAsksBaseTotal and orderBookBidsBaseTotal
				sellPercentDifference := utils.TruncateFloat(((orderBookAsksBaseTotal-orderBookBidsBaseTotal)/orderBookAsksBaseTotal)*100, 3)

				// if market.Pair == "XRPUSDT" && market.RSI > 0 {
				// log.Printf("\n\n\n")
				// log.Println("market: ", market.Pair, " - CHECK TO SELL BACK - ",
				// 	market.Close > market.UpperBand && market.Close < market.Open && market.Price < market.LastPrice && (sellPercentDifference > float64(3) || market.RSI > float64(65)))

				// log.Println("market.Close > market.UpperBand && market.Close < market.Open && market.Price < market.LastPrice && sellPercentDifference > float64(3) || market.RSI > float64(65)")
				// log.Println(market.Close, " > ", market.UpperBand, " && ", market.Close, " < ", market.Open, " && ", market.Price, " < ", market.LastPrice, " && (", sellPercentDifference, " > ", float64(3), " || ", market.RSI, " > ", float64(65), ")")
				// log.Println(market.Close > market.UpperBand, market.Close < market.Open, market.Price < market.LastPrice, sellPercentDifference > float64(3), market.RSI > float64(65))
				// }

				// if market.Close > market.UpperBand && market.Close < market.Open && market.Price < market.LastPrice && (sellPercentDifference > float64(3) || market.RSI > float64(65)) {
				if market.Close >= market.UpperBand && market.Close < market.Open && (sellPercentDifference > float64(5) || market.RSI > float64(65)) {
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

				//calculate percentage difference between orderBookBidsBaseTotal and orderBookAsksBaseTotal
				buyPercentDifference := utils.TruncateFloat(((orderBookBidsBaseTotal-orderBookAsksBaseTotal)/orderBookBidsBaseTotal)*100, 3)

				// if market.Close < market.LowerBand && market.Close > market.Open && market.Price > market.LastPrice && (buyPercentDifference > float64(3) || market.RSI < float64(35)) {
				if market.Close <= market.LowerBand && market.Close > market.Open && (buyPercentDifference > float64(5) || market.RSI < float64(35)) {
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
			updateOrderAndSave(oldOrder, true)

			newOrder := models.Order{}
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

func calculateRSIBands(market *models.Market) {

	marketRSIPricesMutex.RLock()
	rsiPrices := marketRSIPrices[market.Pair]
	marketRSIPricesMutex.RUnlock()

	smoothingLength := 3
	rsiLength := len(rsiPrices) - 1 //for rsiLenght of 9 rsiPrices must have 10 values
	if rsiLength < smoothingLength {
		return
	}

	//calculate RSI
	var avgGain, avgLoss float64
	for i := 1; i < len(rsiPrices); i++ {
		change := rsiPrices[i] - rsiPrices[i-1]
		switch {
		case change > 0:
			avgGain += change
		case change < 0:
			avgLoss += -change
		}
	}

	avgGain /= float64(rsiLength)
	avgLoss /= float64(rsiLength)

	rsiValue := avgGain / avgLoss
	rsiValue = utils.TruncateFloat(100-(100/(1+rsiValue)), 2)
	//calculate RSI

	//Update RSI values for the market
	marketRSIValuesMutex.Lock()
	marketRSIValues[market.Pair] = append(marketRSIValues[market.Pair], rsiValue)
	if len(marketRSIValues[market.Pair]) > smoothingLength {
		marketRSIValues[market.Pair] = marketRSIValues[market.Pair][1:]
	}
	rsiValue = 0
	for _, rsi := range marketRSIValues[market.Pair] {
		rsiValue += rsi
	}
	marketRSIValuesMutex.Unlock()
	//Update RSI values for the market

	//applying RSI smoothing
	rsiValue /= float64(smoothingLength)
	market.RSI = utils.TruncateFloat(rsiValue, 2)
}

func calculateBollingerBands(market *models.Market) {

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
