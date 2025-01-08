package main

import (
	"backpocket/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var (
	TimeframeMaps = map[string][]string{
		"1m":  []string{"1m", "5m", "15m"},
		"5m":  []string{"5m", "15m", "30m"},
		"15m": []string{"15m", "30m", "1h"},
		"30m": []string{"30m", "1h", "4h"},
		"1h":  []string{"1h", "4h", "6h"},
		"4h":  []string{"4h", "6h", "12h"},
		"6h":  []string{"6h", "12h", "1d"},
		"12h": []string{"12h", "1d", "3d"},
	}
)

func restHandlerOpportunity(httpRes http.ResponseWriter, httpReq *http.Request) {
	query := httpReq.URL.Query()

	pair := query.Get("pair")
	exchange := query.Get("exchange")
	timeframe := query.Get("intervals")
	limit := query.Get("limit")
	startTime := query.Get("starttime")
	endTime := query.Get("endtime")
	marketPriceVar := query.Get("marketprice")

	marketPrice, err := strconv.ParseFloat(marketPriceVar, 64)
	if err != nil {
		marketPrice = 0
	}

	if exchange == "" {
		exchange = "binance"
	}

	if pair == "" {
		http.Error(httpRes, "Missing pair parameter", http.StatusBadRequest)
		return
	}

	if TimeframeMaps[timeframe] == nil {
		timeframe = "1m"
	}
	intervals := strings.Join(TimeframeMaps[timeframe], ",")
	analysis, err := retrieveMarketPairAnalysis(pair, exchange, limit, endTime, startTime, intervals)
	if err != nil {
		http.Error(httpRes, err.Error(), http.StatusInternalServerError)
		return
	}
	opportunity := analyseOpportunity(analysis, timeframe, marketPrice)

	httpRes.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(opportunity)
	if err != nil {
		http.Error(httpRes, "Error converting to JSON", http.StatusInternalServerError)
		return
	}

	httpRes.Write(jsonResponse)
}

type opportunityType struct {
	Pair       string
	Action     string
	Price      float64
	Timeframe  string
	Exchange   string
	Stoploss   float64
	Takeprofit float64
	Analysis   map[string]interface{}
}

func analyseOpportunity(analysis analysisType, timeframe string, price float64) (opportunity opportunityType) {
	if analysis.Pair == "" || analysis.Exchange == "" {
		return
	}

	if len(TimeframeMaps[timeframe]) < 3 {
		timeframe = "1m"
	}

	market := getMarket(analysis.Pair, analysis.Exchange)
	if price == 0 {
		price = market.Price
	}

	for _, interval := range analysis.Intervals {
		interval.Candle.Close = price
		interval.Trend = utils.OverallTrend(interval.SMA10.Entry,
			interval.SMA20.Entry, interval.SMA50.Entry, interval.Candle.Close)
	}

	// log.Printf("\n\n---1m Candle----: %+v", analysis.Intervals["1m"].Pattern)

	lowerInterval := analysis.Intervals[TimeframeMaps[timeframe][0]]
	middleInterval := analysis.Intervals[TimeframeMaps[timeframe][1]]
	higherInterval := analysis.Intervals[TimeframeMaps[timeframe][2]]

	if price == 0 {
		price = utils.TruncateFloat((lowerInterval.Candle.Open+lowerInterval.Candle.Close)/2, 8)
	}
	opportunity.Pair = analysis.Pair
	opportunity.Exchange = analysis.Exchange
	opportunity.Timeframe = timeframe
	opportunity.Price = price

	retracement0618 := lowerInterval.RetracementLevels["0.618"]
	retracement0382 := lowerInterval.RetracementLevels["0.382"]

	isAllMarketSupport := (lowerInterval.SMA10.Support == lowerInterval.SMA50.Support &&
		middleInterval.SMA10.Support == middleInterval.SMA50.Support &&
		higherInterval.SMA10.Support == higherInterval.SMA50.Support)

	isSameMarketSupport := (lowerInterval.SMA20.Support == lowerInterval.SMA50.Support &&
		middleInterval.SMA20.Support == middleInterval.SMA50.Support &&
		higherInterval.SMA20.Support == higherInterval.SMA50.Support)

	isLowerMiddleSupport := (lowerInterval.SMA20.Support == middleInterval.SMA20.Support)

	isMarketSupport := false
	if isAllMarketSupport || isSameMarketSupport || isLowerMiddleSupport {
		isMarketSupport = true
	}

	isAllMarketResistance := (lowerInterval.SMA10.Resistance == lowerInterval.SMA50.Resistance &&
		middleInterval.SMA10.Resistance == middleInterval.SMA50.Resistance &&
		higherInterval.SMA10.Resistance == higherInterval.SMA50.Resistance)

	isSameMarketResistance := (lowerInterval.SMA20.Resistance == lowerInterval.SMA50.Resistance &&
		middleInterval.SMA20.Resistance == middleInterval.SMA50.Resistance &&
		higherInterval.SMA20.Resistance == higherInterval.SMA50.Resistance)

	isLowerMiddleResistance := (lowerInterval.SMA20.Resistance == middleInterval.SMA20.Resistance)

	isMarketResistance := false
	if isAllMarketResistance || isSameMarketResistance || isLowerMiddleResistance {
		isMarketResistance = true
	}

	//Check for Long // Buy Opportunity
	if isMarketSupport && lowerInterval.Trend != "Bullish" &&
		(middleInterval.Trend != "Bullish" && higherInterval.Trend != "Bullish") &&
		((showsReversalPatterns("Bullish", lowerInterval.Pattern) && showsReversalPatterns("Bullish", middleInterval.Pattern)) ||
			(showsReversalPatterns("Bullish", lowerInterval.Pattern) && showsReversalPatterns("Bullish", higherInterval.Pattern)) ||
			(showsReversalPatterns("Bullish", middleInterval.Pattern) && showsReversalPatterns("Bullish", higherInterval.Pattern)) ||
			(strings.Contains(middleInterval.Pattern.Candle, "Bullish") || strings.Contains(higherInterval.Pattern.Candle, "Bullish"))) &&

		opportunity.Price <= middleInterval.BollingerBands["middle"] &&
		opportunity.Price > lowerInterval.Candle.Open &&
		opportunity.Price >= retracement0618 &&
		lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry &&
		lowerInterval.RSI < 35 {
		opportunity.Action = "BUY"
	}

	buyAnalysis := []string{}
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("isMarketSupport : %v", isMarketSupport))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.Trend != 'Bullish' : %v - %v", lowerInterval.Trend != "Bullish", lowerInterval.Trend))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("middleInterval.Trend != 'Bullish' : %v - %v", middleInterval.Trend != "Bullish", middleInterval.Trend))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("higherInterval.Trend != 'Bullish' : %v - %v", higherInterval.Trend != "Bullish", higherInterval.Trend))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("middleInterval.Pattern.Candle' : %v", middleInterval.Pattern.Candle))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("higherInterval.Pattern.Candle' : %v", higherInterval.Pattern.Candle))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("showsReversalPatterns(Bullish, lowerInterval.Pattern) : %v = %v", showsReversalPatterns("Bullish", lowerInterval.Pattern), lowerInterval.Pattern.Chart))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("showsReversalPatterns(Bullish, middleInterval.Pattern) : %v = %v", showsReversalPatterns("Bullish", middleInterval.Pattern), middleInterval.Pattern.Chart))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("showsReversalPatterns(Bullish, higherInterval.Pattern) : %v = %v", showsReversalPatterns("Bullish", higherInterval.Pattern), higherInterval.Pattern.Chart))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price <= middleInterval.BollingerBands[middle]  : %v | %v - %v", opportunity.Price <= middleInterval.BollingerBands["middle"], opportunity.Price, middleInterval.BollingerBands["middle"]))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price > lowerInterval.Candle.Open : %v | %v - %v", opportunity.Price > lowerInterval.Candle.Open, opportunity.Price, lowerInterval.Candle.Open))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price >= retracement0618 : %v | %v - %v", opportunity.Price >= retracement0618, opportunity.Price, retracement0618))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry:  %v | %v - %v", lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry, lowerInterval.Candle.Open, lowerInterval.SMA20.Entry))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.RSI %v < 35 : %v", lowerInterval.RSI, lowerInterval.RSI < 35))

	// -- -- --

	//Check for Short // Sell Opportunity
	if isMarketResistance && lowerInterval.Trend != "Bearish" &&
		(middleInterval.Trend != "Bearish" || higherInterval.Trend != "Bearish") &&
		((showsReversalPatterns("Bearish", lowerInterval.Pattern) && showsReversalPatterns("Bearish", middleInterval.Pattern)) ||
			(showsReversalPatterns("Bearish", lowerInterval.Pattern) && showsReversalPatterns("Bearish", higherInterval.Pattern)) ||
			(showsReversalPatterns("Bearish", middleInterval.Pattern) && showsReversalPatterns("Bearish", higherInterval.Pattern)) ||
			(strings.Contains(middleInterval.Pattern.Candle, "Bearish") || strings.Contains(higherInterval.Pattern.Candle, "Bearish"))) &&

		opportunity.Price >= middleInterval.BollingerBands["middle"] &&
		opportunity.Price < lowerInterval.Candle.Open &&
		opportunity.Price <= retracement0382 &&
		lowerInterval.Candle.Open >= lowerInterval.SMA20.Entry &&
		lowerInterval.RSI > 65 {
		opportunity.Action = "SELL"
	}

	sellAnalysis := []string{}
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("isMarketResistance : %v", isMarketResistance))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.Trend != 'Bearish' : %v - %v", lowerInterval.Trend != "Bearish", lowerInterval.Trend))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("middleInterval.Trend != 'Bearish' : %v - %v", middleInterval.Trend != "Bearish", middleInterval.Trend))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("higherInterval.Trend != 'Bearish' : %v - %v", higherInterval.Trend != "Bearish", higherInterval.Trend))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("middleInterval.Pattern.Candle' : %v", middleInterval.Pattern.Candle))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("higherInterval.Pattern.Candle' : %v", higherInterval.Pattern.Candle))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("showsReversalPatterns(Bearish, lowerInterval.Pattern) : %v = %v", showsReversalPatterns("Bearish", lowerInterval.Pattern), lowerInterval.Pattern.Chart))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("showsReversalPatterns(Bearish, middleInterval.Pattern) : %v = %v", showsReversalPatterns("Bearish", middleInterval.Pattern), middleInterval.Pattern.Chart))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("showsReversalPatterns(Bearish, higherInterval.Pattern) : %v = %v", showsReversalPatterns("Bearish", higherInterval.Pattern), higherInterval.Pattern.Chart))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price >= middleInterval.BollingerBands[middle] :  %v | %v - %v", opportunity.Price >= middleInterval.BollingerBands["middle"], opportunity.Price, middleInterval.BollingerBands["middle"]))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price < lowerInterval.Candle.Open : %v | %v - %v", opportunity.Price < lowerInterval.Candle.Open, opportunity.Price, lowerInterval.Candle.Open))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price <= retracement0382 : %v | %v - %v", opportunity.Price <= retracement0382, opportunity.Price, retracement0382))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.Candle.Open >= lowerInterval.SMA20.Entry : %v | %v - %v", lowerInterval.Candle.Open >= lowerInterval.SMA20.Entry, lowerInterval.Candle.Open, lowerInterval.SMA20.Entry))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.RSI %v > 65 : %v", lowerInterval.RSI, lowerInterval.RSI > 65))

	opportunity.Analysis = map[string]interface{}{
		"Buy":  buyAnalysis,
		"Sell": sellAnalysis,
	}

	if opportunity.Action == "BUY" && strings.Contains(analysis.Trend, "Bearish") {
		opportunity.Action = "SELL"
	}

	if opportunity.Action == "SELL" && strings.Contains(analysis.Trend, "Bullish") {
		opportunity.Action = "BUY"
	}

	switch opportunity.Action {
	case "BUY":
		opportunity.Stoploss = utils.TruncateFloat(opportunity.Price*0.99, 8)
		opportunity.Takeprofit = utils.TruncateFloat(opportunity.Price*1.03, 8)
	case "SELL":
		opportunity.Stoploss = utils.TruncateFloat(opportunity.Price*1.01, 8)
		opportunity.Takeprofit = utils.TruncateFloat(opportunity.Price*0.97, 8)
	}

	if market.Closed == 1 {
		opportunityMutex.Lock()
		pairexchange := fmt.Sprintf("%s-%s", analysis.Pair, analysis.Exchange)
		opportunityMap[pairexchange] = notifications{Title: "", Message: ""}
		opportunityMutex.Unlock()
	}

	return
}
