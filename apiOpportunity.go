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
		"5m":  []string{"1m", "5m", "15m", "30m"},
		"15m": []string{"1m", "15m", "30m", "1h"},
		"30m": []string{"1m", "30m", "1h", "4h"},
		"1h":  []string{"1m", "1h", "4h", "6h"},
		"4h":  []string{"1m", "4h", "6h", "12h"},
		"6h":  []string{"1m", "6h", "12h", "1d"},
		"12h": []string{"1m", "12h", "1d", "3d"},
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
	Pair      string
	Action    string
	Price     float64
	Timeframe string
	Exchange  string
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
		if market.Price == 0 {
			price = utils.TruncateFloat((analysis.Intervals["1m"].Candle.Close+analysis.Intervals["1m"].Candle.Open)/2, 8)
		}
	}

	opportunity.Pair = analysis.Pair
	opportunity.Exchange = analysis.Exchange
	opportunity.Timeframe = timeframe
	opportunity.Price = price

	// log.Printf("\n\n---1m Candle----: %+v", analysis.Intervals["1m"].Pattern)

	lowerInterval := analysis.Intervals[TimeframeMaps[timeframe][0]]
	middleInterval := analysis.Intervals[TimeframeMaps[timeframe][1]]
	higherInterval := analysis.Intervals[TimeframeMaps[timeframe][2]]

	retracement0236 := lowerInterval.RetracementLevels["0.236"]
	retracement0786 := lowerInterval.RetracementLevels["0.786"]

	isMarketSupport := false
	if lowerInterval.SMA10.Support == lowerInterval.SMA20.Support &&
		lowerInterval.SMA10.Support == lowerInterval.SMA50.Support &&

		(middleInterval.SMA10.Support == middleInterval.SMA20.Support ||
			middleInterval.SMA20.Support == middleInterval.SMA50.Support) &&

		(higherInterval.SMA10.Support == middleInterval.SMA20.Support ||
			higherInterval.SMA20.Support == middleInterval.SMA50.Support ||
			higherInterval.SMA10.Support == higherInterval.SMA50.Support) {
		isMarketSupport = true
	}

	isMarketResistance := false
	if lowerInterval.SMA10.Resistance == lowerInterval.SMA20.Resistance &&
		lowerInterval.SMA10.Resistance == lowerInterval.SMA50.Resistance &&

		(middleInterval.SMA10.Resistance == middleInterval.SMA20.Resistance ||
			middleInterval.SMA20.Resistance == middleInterval.SMA50.Resistance) &&

		(higherInterval.SMA10.Resistance == middleInterval.SMA20.Resistance ||
			higherInterval.SMA20.Resistance == middleInterval.SMA50.Resistance ||
			higherInterval.SMA10.Resistance == higherInterval.SMA50.Resistance) {
		isMarketResistance = true
	}

	//Check for Long // Buy Opportunity
	if isMarketSupport && lowerInterval.Trend != "Bullish" &&
		showsReversalPatterns("Bullish", lowerInterval.Pattern) &&
		showsReversalPatterns("Bullish", middleInterval.Pattern) &&
		showsReversalPatterns("Bullish", higherInterval.Pattern) &&
		opportunity.Price <= lowerInterval.SMA10.Entry &&
		opportunity.Price >= lowerInterval.Candle.Open &&
		(lowerInterval.Candle.Open >= retracement0786 ||
			opportunity.Price <= retracement0236) &&
		lowerInterval.Candle.Open <= lowerInterval.BollingerBands["middle"] && lowerInterval.RSI < 44 {
		opportunity.Action = "BUY"
	}
	// log.Println("--------")
	// log.Println("--------")
	// log.Println("--------")
	// log.Println("Action:", opportunity.Action)
	// log.Println("BUY ANALYSIS: ", analysis.Pair, " @ ", analysis.Exchange)
	// log.Println("isMarketSupport:", isMarketSupport)
	// log.Println("lowerInterval.Trend != Bullish:", lowerInterval.Trend != "Bullish")
	// log.Println("showsReversalPatterns(Bullish, lowerInterval.Pattern):", showsReversalPatterns("Bullish", lowerInterval.Pattern))
	// log.Println("showsReversalPatterns(Bullish, middleInterval.Pattern):", showsReversalPatterns("Bullish", middleInterval.Pattern))
	// log.Println("showsReversalPatterns(Bullish, higherInterval.Pattern):", showsReversalPatterns("Bullish", higherInterval.Pattern))

	// log.Println("opportunity.Price <= lowerInterval.SMA10.Entry:", opportunity.Price <= lowerInterval.SMA10.Entry, opportunity.Price, lowerInterval.SMA10.Entry)
	// log.Println("opportunity.Price >= lowerInterval.Candle.Open:", opportunity.Price >= lowerInterval.Candle.Open, opportunity.Price, lowerInterval.Candle.Open)
	// log.Println("lowerInterval.Candle.Open >= retracement0786:", lowerInterval.Candle.Open >= retracement0786, lowerInterval.Candle.Open, retracement0786)
	// log.Println("opportunity.Price <= retracement0236:", opportunity.Price <= retracement0236, opportunity.Price, retracement0236)

	// log.Println("lowerInterval.Candle.Open <= lowerInterval.BollingerBands[middle]:", lowerInterval.Candle.Open <= lowerInterval.BollingerBands["middle"], lowerInterval.Candle.Open, lowerInterval.BollingerBands["middle"])
	// log.Println("lowerInterval.RSI < 40:", lowerInterval.RSI)

	// -- -- --

	//Check for Short // Sell Opportunity
	if isMarketResistance && lowerInterval.Trend != "Bearish" &&
		showsReversalPatterns("Bearish", lowerInterval.Pattern) &&
		showsReversalPatterns("Bearish", middleInterval.Pattern) &&
		showsReversalPatterns("Bearish", higherInterval.Pattern) &&
		opportunity.Price >= lowerInterval.SMA10.Entry &&
		opportunity.Price <= lowerInterval.Candle.Open &&
		(lowerInterval.Candle.Open <= retracement0236 ||
			opportunity.Price <= retracement0786) &&
		lowerInterval.Candle.Open >= lowerInterval.BollingerBands["middle"] && lowerInterval.RSI > 55 {
		opportunity.Action = "SELL"
	}
	// log.Println("--------")
	// log.Println("--------")
	// log.Println("--------")
	// log.Println("Action:", opportunity.Action)
	// log.Println("SELL ANALYSIS: ", analysis.Pair, " @ ", analysis.Exchange)
	// log.Println("isMarketResistance:", isMarketResistance)
	// log.Println("lowerInterval.Trend != Bearish:", lowerInterval.Trend != "Bearish")
	// log.Println("showsReversalPatterns(Bearish, lowerInterval.Pattern):", showsReversalPatterns("Bearish", lowerInterval.Pattern))
	// log.Println("showsReversalPatterns(Bearish, middleInterval.Pattern):", showsReversalPatterns("Bearish", middleInterval.Pattern))
	// log.Println("showsReversalPatterns(Bearish, higherInterval.Pattern):", showsReversalPatterns("Bearish", higherInterval.Pattern))

	// log.Println("opportunity.Price >= lowerInterval.SMA10.Entry:", opportunity.Price >= lowerInterval.SMA10.Entry, opportunity.Price, lowerInterval.SMA10.Entry)
	// log.Println("opportunity.Price <= lowerInterval.Candle.Open:", opportunity.Price <= lowerInterval.Candle.Open, opportunity.Price, lowerInterval.Candle.Open)
	// log.Println("lowerInterval.Candle.Open <= retracement0236:", lowerInterval.Candle.Open <= retracement0236, lowerInterval.Candle.Open, retracement0236)
	// log.Println("opportunity.Price <= retracement0786:", opportunity.Price <= retracement0786, opportunity.Price, retracement0786)

	// log.Println("lowerInterval.Candle.Open >= lowerInterval.BollingerBands[middle]:", lowerInterval.Candle.Open >= lowerInterval.BollingerBands["middle"], lowerInterval.Candle.Open, lowerInterval.BollingerBands["middle"])
	// log.Println("lowerInterval.RSI > 55:", lowerInterval.RSI)

	if market.Closed == 1 {
		opportunityMutex.Lock()
		pairexchange := fmt.Sprintf("%s-%s", analysis.Pair, analysis.Exchange)
		opportunityMap[pairexchange] = notifications{Title: "", Message: ""}
		opportunityMutex.Unlock()
	}

	return
}
