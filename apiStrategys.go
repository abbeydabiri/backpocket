package main

import (
	"backpocket/models"
	"backpocket/utils"
	"fmt"
	"log"
	"strconv"
	"sync"
)

const (
	DefaultTimeframe = "1m"
)

var (
	bollingerBands  = make(map[string][]float64)
	marketRSIPrices = make(map[string][]float64)

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
			analysis := getAnalysis(oldOrder.Pair, oldOrder.Exchange)
			analysisTimeframe := "5m"
			analysisInterval := utils.Summary{}
			if analysis.Intervals[analysisTimeframe].Timeframe == analysisTimeframe {
				analysisInterval = analysis.Intervals[analysisTimeframe]
			}
			if analysisInterval.Timeframe == "" {
				for _, interval := range analysis.Intervals {
					if interval.Timeframe == "" {
						analysisInterval = interval
						break
					}
				}
			}

			marketRSI := analysisInterval.RSI
			marketUpperBand := analysisInterval.BollingerBands["upper"]
			marketLowerBand := analysisInterval.BollingerBands["lower"]

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
				// 	market.Close > marketUpperBand && market.Close < market.Open && market.Price < market.LastPrice && (sellPercentDifference > float64(3) || marketRSI > float64(65)))

				// log.Println("market.Close > marketUpperBand && market.Close < market.Open && market.Price < market.LastPrice && sellPercentDifference > float64(3) || marketRSI > float64(65)")
				// log.Println(market.Close, " > ", marketUpperBand, " && ", market.Close, " < ", market.Open, " && ", market.Price, " < ", market.LastPrice, " && (", sellPercentDifference, " > ", float64(3), " || ", marketRSI, " > ", float64(65), ")")
				// log.Println(market.Close > marketUpperBand, market.Close < market.Open, market.Price < market.LastPrice, sellPercentDifference > float64(3), marketRSI > float64(65))
				// }

				if market.Close >= marketUpperBand && market.Close < market.Open && sellPercentDifference > float64(2) && marketRSI >= float64(63) {
					newTakeprofit := utils.TruncateFloat(((orderbookBidPrice-oldOrder.Price)/oldOrder.Price)*100, 3)
					// log.Println("TRIGGER SELL: ", oldOrder.OrderID, " [-] Market: ", market.Pair, " [-] newTakeprofit: ", newTakeprofit, " [-] oldTakeprofit: ", oldOrder.Takeprofit)

					if newTakeprofit >= oldOrder.Takeprofit && oldOrder.Takeprofit > 0 {
						oldOrder.RefTripped = fmt.Sprintf("> %.3f%% TP: %.8f @ RSI %.2f", newTakeprofit, orderbookBidPrice, marketRSI)
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

				// if market.Pair == "XRPUSDT" && marketRSI > 0 {
				// 	fmt.Printf("\n\n\n")
				// 	fmt.Println("market: ", market.Pair, " - CHECK TO BUY BACK - ",
				// 		market.Close <= marketLowerBand && market.Close > market.Open && market.Price > market.LastPrice && orderBookBidsBaseTotal > orderBookAsksBaseTotal && marketRSI < float64(30))

				// 	fmt.Println("market.Close <= marketLowerBand && market.Close > market.Open && market.Price > market.LastPrice && orderBookBidsBaseTotal > orderBookAsksBaseTotal && marketRSI < float64(30)")
				// 	fmt.Println(market.Close, " <= ", marketLowerBand, " && ", market.Close, " > ", market.Open, " && ", market.Price, " > ", market.LastPrice, " && ", orderBookBidsBaseTotal, " > ", orderBookAsksBaseTotal, " && ", marketRSI, " < ", float64(30))
				// 	fmt.Println(market.Close <= marketLowerBand, market.Close > market.Open, market.Price > market.LastPrice, orderBookBidsBaseTotal > orderBookAsksBaseTotal, marketRSI < float64(30))
				// }

				//calculate percentage difference between orderBookBidsBaseTotal and orderBookAsksBaseTotal
				buyPercentDifference := utils.TruncateFloat(((orderBookBidsBaseTotal-orderBookAsksBaseTotal)/orderBookBidsBaseTotal)*100, 3)

				// if market.Close < marketLowerBand && market.Close > market.Open && market.Price > market.LastPrice && (buyPercentDifference > float64(3) || marketRSI < float64(35)) {
				if market.Close <= marketLowerBand && market.Close > market.Open && buyPercentDifference > float64(2) && marketRSI <= float64(36) {
					newTakeprofit := utils.TruncateFloat(((oldOrder.Price-orderbookAskPrice)/oldOrder.Price)*100, 3)
					// log.Println("TRIGGER BUY: ", oldOrder.OrderID, " [-] Market: ", market.Pair, " [-] newTakeprofit: ", newTakeprofit, " [-] oldTakeprofit: ", oldOrder.Takeprofit)

					if newTakeprofit >= oldOrder.Takeprofit && oldOrder.Takeprofit > 0 {
						oldOrder.RefTripped = fmt.Sprintf("< %.3f%% TP: %.8f @ RSI %.2f", newTakeprofit, orderbookAskPrice, marketRSI)
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

func calculateBollingerBands(market *models.Market) {
	analysis := getAnalysis(market.Pair, market.Exchange)
	if analysis.Intervals[DefaultTimeframe].Timeframe == DefaultTimeframe {
		market.UpperBand = analysis.Intervals[DefaultTimeframe].BollingerBands["upper"]
		market.MiddleBand = analysis.Intervals[DefaultTimeframe].BollingerBands["middle"]
		market.LowerBand = analysis.Intervals[DefaultTimeframe].BollingerBands["lower"]
	}
}

func calculateRSIBands(market *models.Market) {
	marketRSIPricesMutex.RLock()
	rsimapkey := fmt.Sprintf("%s-%s", market.Pair, market.Exchange)
	rsiPrices := marketRSIPrices[rsimapkey]
	marketRSIPricesMutex.RUnlock()
	market.RSI = 0
	if len(rsiPrices) > 14 {
		market.RSI = utils.CalculateSmoothedRSI(rsiPrices, 14, 5)
	}
}
