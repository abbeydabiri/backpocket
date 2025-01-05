package main

import (
	"backpocket/models"
	"backpocket/utils"
	"fmt"
	"log"
	"strconv"
	"strings"
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

	opportunityMutex = sync.RWMutex{}
	opportunityMap   = make(map[string]notifications)

	chanStoplossTakeProfit = make(chan orderbooks, 10240)
)

func showsReversalPatterns(trend string, pattern utils.SummaryPattern) (match bool) {

	if strings.Contains(pattern.Chart, trend+":") {
		match = true
	}

	if match && strings.Contains(pattern.Candle, trend+":") {
		match = true
	}

	return
}

func findOpportunity(pair, exchange string,
	buyPercentDiff, sellPercentDiff float64) (opportunity string) {
	// Optimal Entry and Exit Points
	// 	1. For Longs:
	// 		Entry: Look for bullish patterns in short-term timeframes (Minutes and Hours) within a neutral-to-bullish overall trend.
	//		(e.g 1m or 5m is neutral or bearish and patterns are bullish)
	//
	// 		Exit: Monitor bearish patterns in higher timeframes (Days and Weeks) within a neutral-to-bullish overall trend.
	//		(e.g 5m or 15m is neutral or bullish and patterns are bearish)
	//
	// 	2. For Shorts:
	// 		Entry: Look for bearish patterns in short-term timeframes within a neutral-to-bearish overall trend.
	//		(e.g 1m or 5m is neutral or bullish and patterns are bearish)
	//
	// 		Exit: Monitor neutral or bullish patterns in higher timeframes.
	//		(e.g 5m or 15m is neutral or bearish and patterns are bullish)

	if pair == "" || exchange == "" {
		return
	}

	market := getMarket(pair, exchange)
	analysis := getAnalysis(pair, exchange)

	if market.Pair != pair || market.Exchange != exchange {
		return
	}

	if analysis.Pair != pair || analysis.Exchange != exchange {
		return
	}

	//lower - higher | 1m - 15m | 5m - 1h | 15m - 4h | 30m - 6h | 1h -12h | 4h - 1d | 6h - 3d

	lowerInterval := analysis.Intervals["1m"]
	higherInterval := analysis.Intervals["15m"]

	lowerRetracement := higherInterval.RetracementLevels["0.786"]
	higherRetracement := higherInterval.RetracementLevels["0.236"]

	isMarketSupport := false
	if lowerInterval.SMA10.Support == lowerInterval.SMA20.Support &&
		lowerInterval.SMA20.Support == lowerInterval.SMA50.Support {
		isMarketSupport = true
	}

	isMarketResistance := false
	if lowerInterval.SMA10.Resistance == lowerInterval.SMA20.Resistance &&
		lowerInterval.SMA20.Resistance == lowerInterval.SMA50.Resistance {
		isMarketResistance = true
	}

	//Check for Long // Buy Opportunity
	if isMarketSupport && lowerInterval.Trend == "Bearish" && showsReversalPatterns("Bullish", lowerInterval.Pattern) &&
		market.Close > lowerInterval.Candle.Open && market.Price > market.LastPrice && market.Close > lowerRetracement &&
		lowerInterval.Candle.Open <= lowerInterval.BollingerBands["lower"] && buyPercentDiff > float64(3) {
		opportunity = "BUY"
	}

	// -- -- --

	//Check for Short // Sell Opportunity
	if isMarketResistance && lowerInterval.Trend == "Bullish" && showsReversalPatterns("Bearish", lowerInterval.Pattern) &&
		market.Close < lowerInterval.Candle.Open && market.Price < market.LastPrice && market.Close < higherRetracement &&
		lowerInterval.Candle.Open >= lowerInterval.BollingerBands["higher"] && sellPercentDiff > float64(3) {
		opportunity = "SELL"
	}

	if market.Closed == 1 {
		opportunityMutex.Lock()
		pairexchange := fmt.Sprintf("%s-%s", pair, exchange)
		opportunityMap[pairexchange] = notifications{Title: "", Message: ""}
		opportunityMutex.Unlock()
	}

	return
}

func apiStrategyStopLossTakeProfit() {

	for orderbook := range chanStoplossTakeProfit {

		var orderbookBidPrice, orderbookAskPrice float64
		var orderBookBidsBaseTotal, orderBookAsksBaseTotal float64

		orderbookMutex.RLock()
		orderbookPair := orderbook.Pair
		orderbookExchange := orderbook.Exchange
		if len(orderbook.Bids) > 3 {
			orderbookBidPrice = orderbook.Bids[0].Price
		}
		if len(orderbook.Asks) > 3 {
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

		sellPercentDifference := utils.TruncateFloat(((orderBookAsksBaseTotal-orderBookBidsBaseTotal)/orderBookAsksBaseTotal)*100, 3)
		buyPercentDifference := utils.TruncateFloat(((orderBookBidsBaseTotal-orderBookAsksBaseTotal)/orderBookBidsBaseTotal)*100, 3)
		opportunityFound := findOpportunity(orderbookPair, orderbookExchange, buyPercentDifference, sellPercentDifference)

		if opportunityFound != "" {
			go func() {
				price := orderbookAskPrice
				opportunity := "BUY or LONG"
				if opportunityFound == "SELL" {
					price = orderbookBidPrice
					opportunity = "SELL or SHORT"
				}

				pairexchange := fmt.Sprintf("%s-%s", orderbookPair, orderbookExchange)
				title := fmt.Sprintf("*%s Exchange", strings.ToTitle(orderbookExchange))
				message := fmt.Sprintf("%s '%s' @ %v", opportunity, orderbookPair, price)
				log.Printf("Opportunity: %s | %s \n", title, message)

				opportunityMutex.Lock()
				if !strings.Contains(opportunityMap[pairexchange].Message, opportunity) {
					opportunityMap[pairexchange] = notifications{
						Title: title, Message: message,
					}
					wsBroadcastNotification <- opportunityMap[pairexchange]
				}
				opportunityMutex.Unlock()
			}()
		}

		// if opportunityFound == "" {
		// 	continue
		// }

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

			switch oldOrder.Side {
			case "BUY": //CHECK TO SELL BACK
				oldOrder.RefSide = "SELL"

				if opportunityFound == "SELL" {
					newTakeprofit := utils.TruncateFloat(((orderbookBidPrice-oldOrder.Price)/oldOrder.Price)*100, 3)
					if newTakeprofit >= oldOrder.Takeprofit && oldOrder.Takeprofit > 0 {
						oldOrder.RefTripped = fmt.Sprintf("> %.3f%% TP: %.8f", newTakeprofit, orderbookBidPrice)
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

				if opportunityFound == "BUY" {
					newTakeprofit := utils.TruncateFloat(((oldOrder.Price-orderbookAskPrice)/oldOrder.Price)*100, 3)
					if newTakeprofit >= oldOrder.Takeprofit && oldOrder.Takeprofit > 0 {
						oldOrder.RefTripped = fmt.Sprintf("< %.3f%% TP: %.8ff", newTakeprofit, orderbookAskPrice)
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
	for _, interval := range analysis.Intervals {
		// if interval.Timeframe == DefaultTimeframe {
		interval.Candle.Close = market.Close
		// }
		interval.Trend = utils.OverallTrend(interval.SMA10.Entry,
			interval.SMA20.Entry, interval.SMA50.Entry, interval.Candle.Close)
	}
	analysis.Trend = utils.TimeframeTrends(analysis.Intervals)
	updateAnalysis(analysis)
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
