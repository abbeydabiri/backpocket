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
			marketTrend := analysisInterval.Trend
			marketUpperBand := analysisInterval.BollingerBands["upper"]
			marketLowerBand := analysisInterval.BollingerBands["lower"]
			lowestRetracement := analysisInterval.RetracementLevels["0.786"]
			highestRetracement := analysisInterval.RetracementLevels["0.236"]

			switch oldOrder.Side {
			case "BUY": //CHECK TO SELL BACK
				oldOrder.RefSide = "SELL"

				//calculate percentage difference between orderBookAsksBaseTotal and orderBookBidsBaseTotal
				sellPercentDifference := utils.TruncateFloat(((orderBookAsksBaseTotal-orderBookBidsBaseTotal)/orderBookAsksBaseTotal)*100, 3)

				if market.Open >= marketUpperBand && market.Close < market.Open && sellPercentDifference > float64(2) &&
					marketRSI > float64(70) && marketTrend == "Strong Bullish" && market.Price > highestRetracement {
					newTakeprofit := utils.TruncateFloat(((orderbookBidPrice-oldOrder.Price)/oldOrder.Price)*100, 3)

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

				//calculate percentage difference between orderBookBidsBaseTotal and orderBookAsksBaseTotal
				buyPercentDifference := utils.TruncateFloat(((orderBookBidsBaseTotal-orderBookAsksBaseTotal)/orderBookBidsBaseTotal)*100, 3)

				if market.Open <= marketLowerBand && market.Close > market.Open && buyPercentDifference > float64(2) &&
					marketRSI < float64(30) && marketTrend == "Strong Bearish" && market.Price < lowestRetracement {
					newTakeprofit := utils.TruncateFloat(((oldOrder.Price-orderbookAskPrice)/oldOrder.Price)*100, 3)

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
