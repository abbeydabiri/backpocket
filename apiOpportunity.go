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

	retracement0236 := lowerInterval.RetracementLevels["0.236"]
	retracement0786 := lowerInterval.RetracementLevels["0.786"]

	isMarketSupport := false
	if (lowerInterval.SMA10.Support == middleInterval.SMA10.Support &&
		middleInterval.SMA20.Support == higherInterval.SMA10.Support) ||
		(lowerInterval.SMA10.Support == lowerInterval.SMA50.Support &&
			middleInterval.SMA10.Support == middleInterval.SMA50.Support &&
			higherInterval.SMA10.Support == higherInterval.SMA50.Support) {
		isMarketSupport = true
	}

	isMarketResistance := false
	if (lowerInterval.SMA10.Resistance == middleInterval.SMA10.Resistance &&
		middleInterval.SMA20.Resistance == higherInterval.SMA10.Resistance) ||
		(lowerInterval.SMA10.Resistance == lowerInterval.SMA50.Resistance &&
			middleInterval.SMA10.Resistance == middleInterval.SMA50.Resistance &&
			higherInterval.SMA10.Resistance == higherInterval.SMA50.Resistance) {
		isMarketResistance = true
	}

	//Check for Long // Buy Opportunity
	if isMarketSupport && lowerInterval.Trend != "Bullish" &&

		((showsReversalPatterns("Bullish", lowerInterval.Pattern) && showsReversalPatterns("Bullish", middleInterval.Pattern)) ||
			(showsReversalPatterns("Bullish", lowerInterval.Pattern) && showsReversalPatterns("Bullish", higherInterval.Pattern)) ||
			(showsReversalPatterns("Bullish", middleInterval.Pattern) && showsReversalPatterns("Bullish", higherInterval.Pattern))) &&

		opportunity.Price <= middleInterval.BollingerBands["middle"] &&
		higherInterval.Candle.Open < opportunity.Price &&
		opportunity.Price > lowerInterval.Candle.Open &&
		opportunity.Price >= retracement0236 &&
		lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry &&
		lowerInterval.RSI < 50 {
		opportunity.Action = "BUY"
		opportunity.Stoploss = lowerInterval.SMA50.Support
		opportunity.Takeprofit = middleInterval.SMA50.Resistance
	}
	buyAnalysis := []string{}

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("isMarketSupport : %v", isMarketSupport))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.Trend != 'Bullish' : %v", lowerInterval.Trend != "Bullish"))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("showsReversalPatterns(Bullish, lowerInterval.Pattern) : %v = %v", showsReversalPatterns("Bullish", lowerInterval.Pattern), lowerInterval.Pattern.Chart))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("showsReversalPatterns(Bullish, middleInterval.Pattern) : %v = %v", showsReversalPatterns("Bullish", middleInterval.Pattern), middleInterval.Pattern.Chart))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("showsReversalPatterns(Bullish, higherInterval.Pattern) : %v = %v", showsReversalPatterns("Bullish", higherInterval.Pattern), higherInterval.Pattern.Chart))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price <= middleInterval.BollingerBands[middle]  : %v | %v - %v", opportunity.Price <= middleInterval.BollingerBands["middle"], opportunity.Price, middleInterval.BollingerBands["middle"]))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("higherInterval.Candle.Open < opportunity.Price : %v | %v - %v", higherInterval.Candle.Open < opportunity.Price, higherInterval.Candle.Open, opportunity.Price))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price > lowerInterval.Candle.Open : %v | %v - %v", opportunity.Price > lowerInterval.Candle.Open, opportunity.Price, lowerInterval.Candle.Open))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price >= retracement0236 : %v | %v - %v", opportunity.Price >= retracement0236, opportunity.Price, retracement0236))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry:  %v | %v - %v", lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry, lowerInterval.Candle.Open, lowerInterval.SMA20.Entry))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.Pattern.Candle : %v", lowerInterval.Pattern.Candle))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.RSI %v < 50 : %v", lowerInterval.RSI, lowerInterval.RSI < 50))

	// -- -- --

	//Check for Short // Sell Opportunity
	if isMarketResistance && lowerInterval.Trend != "Bearish" &&

		((showsReversalPatterns("Bearish", lowerInterval.Pattern) && showsReversalPatterns("Bearish", middleInterval.Pattern)) ||
			(showsReversalPatterns("Bearish", lowerInterval.Pattern) && showsReversalPatterns("Bearish", higherInterval.Pattern)) ||
			(showsReversalPatterns("Bearish", middleInterval.Pattern) && showsReversalPatterns("Bearish", higherInterval.Pattern))) &&

		opportunity.Price >= middleInterval.BollingerBands["middle"] &&
		higherInterval.Candle.Open > opportunity.Price &&
		opportunity.Price < lowerInterval.Candle.Open &&
		opportunity.Price <= retracement0786 &&
		lowerInterval.Candle.Open >= lowerInterval.SMA10.Entry &&
		lowerInterval.RSI > 50 {
		opportunity.Action = "SELL"
		opportunity.Stoploss = lowerInterval.SMA50.Resistance
		opportunity.Takeprofit = lowerInterval.SMA50.Support
	}
	sellAnalysis := []string{}
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("isMarketResistance : %v", isMarketResistance))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.Trend != 'Bearish' : %v", lowerInterval.Trend != "Bearish"))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("showsReversalPatterns(Bearish, lowerInterval.Pattern) : %v = %v", showsReversalPatterns("Bearish", lowerInterval.Pattern), lowerInterval.Pattern.Chart))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("showsReversalPatterns(Bearish, middleInterval.Pattern) : %v = %v", showsReversalPatterns("Bearish", middleInterval.Pattern), middleInterval.Pattern.Chart))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("showsReversalPatterns(Bearish, higherInterval.Pattern) : %v = %v", showsReversalPatterns("Bearish", higherInterval.Pattern), higherInterval.Pattern.Chart))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price >= middleInterval.BollingerBands[middle] :  %v | %v - %v", opportunity.Price >= middleInterval.BollingerBands["middle"], opportunity.Price, middleInterval.BollingerBands["middle"]))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("higherInterval.Candle.Open > opportunity.Price && : %v | %v - %v", higherInterval.Candle.Open > opportunity.Price, higherInterval.Candle.Open, opportunity.Price))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price < lowerInterval.Candle.Open : %v | %v - %v", opportunity.Price < lowerInterval.Candle.Open, opportunity.Price, lowerInterval.Candle.Open))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price <= retracement0786 : %v | %v - %v", opportunity.Price <= retracement0786, opportunity.Price, retracement0786))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.Candle.Open >= lowerInterval.SMA10.Entry : %v | %v - %v", lowerInterval.Candle.Open >= lowerInterval.SMA10.Entry, lowerInterval.Candle.Open, lowerInterval.SMA10.Entry))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.Pattern.Candle : %v", lowerInterval.Pattern.Candle))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.RSI %v > 50 : %v", lowerInterval.RSI, lowerInterval.RSI > 50))

	opportunity.Analysis = map[string]interface{}{
		"Buy":  buyAnalysis,
		"Sell": sellAnalysis,
	}

	if market.Closed == 1 {
		opportunityMutex.Lock()
		pairexchange := fmt.Sprintf("%s-%s", analysis.Pair, analysis.Exchange)
		opportunityMap[pairexchange] = notifications{Title: "", Message: ""}
		opportunityMutex.Unlock()
	}

	return
}
