package main

import (
	"backpocket/models"
	"backpocket/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var (
	// "1m":  []string{"1m", "5m", "15m"},
	// "5m":  []string{"5m", "15m", "30m"},
	// "15m": []string{"15m", "30m", "1h"},
	// "30m": []string{"30m", "1h", "4h"},
	// "1h":  []string{"1h", "4h", "6h"},
	// "4h":  []string{"4h", "6h", "12h"},
	// "6h":  []string{"6h", "12h", "1d"},
	// "12h": []string{"12h", "1d", "3d"},
	TimeframeMaps = map[string][]string{
		// "1M": {"1M", "1w", "1d", "4h"},
		// "1w": {"1w", "1d", "4h", "15m"},
		"3d": {"3d", "12h", "1h", "5m"},
		"1d": {"1d", "4h", "30m", "3m"},
		"6h": {"6h", "1h", "15m", "1m"},
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
		timeframe = "1d"
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
func restHandlerSearchOpportunity(httpRes http.ResponseWriter, httpReq *http.Request) {
	query := httpReq.URL.Query()

	pair := query.Get("pair")
	action := query.Get("action")
	exchange := query.Get("exchange")
	timeframe := query.Get("timeframe")

	starttime := query.Get("starttime")
	endtime := query.Get("endtime")

	var searchText string
	var searchParams []interface{}

	if pair != "" {
		searchText = " pair like ? "
		searchParams = append(searchParams, pair)
	}

	if action != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText = " action like ? "
		searchParams = append(searchParams, action)
	}

	if exchange != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText = " exchange like ? "
		searchParams = append(searchParams, exchange)
	}

	if timeframe != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText = " timeframe like ? "
		searchParams = append(searchParams, timeframe)
	}

	if starttime != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " createdate >= ?::timestamp "
		searchParams = append(searchParams, starttime)
	}

	if endtime != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " createdate <= ?::timestamp "
		searchParams = append(searchParams, endtime)
	}

	orderby := "createdate desc"

	var filteredOrderList []models.Opportunity
	if err := utils.SqlDB.Where(searchText, searchParams...).Order(orderby).Find(&filteredOrderList).Error; err != nil {
		http.Error(httpRes, err.Error(), http.StatusInternalServerError)
	}

	httpRes.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(filteredOrderList)
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

func oldAnalyseOpportunity(analysis analysisType, timeframe string, price float64) (opportunity opportunityType) {
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

	isAllMarketSupport := (lowerInterval.SMA10.Support ==
		lowerInterval.SMA50.Support &&
		lowerInterval.SMA20.Support == middleInterval.SMA10.Support)

	isSameMarketSupport := (lowerInterval.SMA20.Support == lowerInterval.SMA50.Support &&
		middleInterval.SMA20.Support == middleInterval.SMA50.Support &&
		higherInterval.SMA10.Support == higherInterval.SMA20.Support)

	isLowerMiddleSupport := (lowerInterval.SMA20.Support == middleInterval.SMA20.Support)

	isMarketSupport := false
	if isAllMarketSupport || isSameMarketSupport || isLowerMiddleSupport {
		isMarketSupport = true
	}

	isAllMarketResistance := (lowerInterval.SMA10.Resistance ==
		lowerInterval.SMA50.Resistance &&
		lowerInterval.SMA20.Resistance == middleInterval.SMA10.Resistance)

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
		showsReversalPatterns("Bullish", lowerInterval.Pattern) &&

		(strings.Contains(lowerInterval.Pattern.Candle, "Bullish") ||
			strings.Contains(middleInterval.Pattern.Candle, "Bullish") ||
			strings.Contains(higherInterval.Pattern.Candle, "Bullish")) &&

		lowerInterval.Candle.Close > lowerInterval.PrevCandle.High &&

		opportunity.Price <= lowerInterval.BollingerBands["middle"] &&
		opportunity.Price > lowerInterval.Candle.Open &&
		opportunity.Price >= retracement0618 &&
		lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry &&
		lowerInterval.RSI < 50 {
		opportunity.Action = "BUY"
	}

	buyAnalysis := []string{}
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("isMarketSupport : %v", isMarketSupport))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.Trend != 'Bullish' : %v - %v", lowerInterval.Trend != "Bullish", lowerInterval.Trend))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("showsReversalPatterns(Bullish, lowerInterval.Pattern) : %v - %v",
		showsReversalPatterns("Bullish", lowerInterval.Pattern), lowerInterval.Pattern.Chart))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.Pattern.Candle' : %v", lowerInterval.Pattern.Candle))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("middleInterval.Pattern.Candle' : %v", middleInterval.Pattern.Candle))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("higherInterval.Pattern.Candle' : %v", higherInterval.Pattern.Candle))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price <= lowerInterval.BollingerBands[middle]  : %v | %v - %v", opportunity.Price <= lowerInterval.BollingerBands["middle"], opportunity.Price, middleInterval.BollingerBands["middle"]))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price > lowerInterval.Candle.Open : %v | %v - %v", opportunity.Price > lowerInterval.Candle.Open, opportunity.Price, lowerInterval.Candle.Open))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("opportunity.Price >= retracement0618 : %v | %v - %v", opportunity.Price >= retracement0618, opportunity.Price, retracement0618))
	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry:  %v | %v - %v", lowerInterval.Candle.Open <= lowerInterval.SMA20.Entry, lowerInterval.Candle.Open, lowerInterval.SMA20.Entry))

	buyAnalysis = append(buyAnalysis, fmt.Sprintf("lowerInterval.RSI %v < 50 : %v", lowerInterval.RSI, lowerInterval.RSI < 50))

	// -- -- --

	//Check for Short // Sell Opportunity
	if isMarketResistance && lowerInterval.Trend != "Bearish" &&
		showsReversalPatterns("Bearish", lowerInterval.Pattern) &&

		(strings.Contains(lowerInterval.Pattern.Candle, "Bearish") ||
			strings.Contains(middleInterval.Pattern.Candle, "Bearish") ||
			strings.Contains(higherInterval.Pattern.Candle, "Bearish")) &&

		lowerInterval.Candle.Close < lowerInterval.PrevCandle.Low &&

		opportunity.Price >= lowerInterval.BollingerBands["middle"] &&
		opportunity.Price < lowerInterval.Candle.Open &&
		opportunity.Price <= retracement0382 &&
		lowerInterval.Candle.Open >= lowerInterval.SMA20.Entry &&
		lowerInterval.RSI > 50 {
		opportunity.Action = "SELL"
	}

	sellAnalysis := []string{}
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("isMarketResistance : %v", isMarketResistance))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.Trend != 'Bearish' : %v - %v", lowerInterval.Trend != "Bearish", lowerInterval.Trend))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("showsReversalPatterns(Bearish, lowerInterval.Pattern) : %v - %v",
		showsReversalPatterns("Bearish", lowerInterval.Pattern), lowerInterval.Pattern.Chart))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.Pattern.Candle' : %v", lowerInterval.Pattern.Candle))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("middleInterval.Pattern.Candle' : %v", middleInterval.Pattern.Candle))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("higherInterval.Pattern.Candle' : %v", higherInterval.Pattern.Candle))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price >= lowerInterval.BollingerBands[middle] :  %v | %v - %v", opportunity.Price >= lowerInterval.BollingerBands["middle"], opportunity.Price, middleInterval.BollingerBands["middle"]))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price < lowerInterval.Candle.Open : %v | %v - %v", opportunity.Price < lowerInterval.Candle.Open, opportunity.Price, lowerInterval.Candle.Open))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("opportunity.Price <= retracement0382 : %v | %v - %v", opportunity.Price <= retracement0382, opportunity.Price, retracement0382))
	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.Candle.Open >= lowerInterval.SMA20.Entry : %v | %v - %v", lowerInterval.Candle.Open >= lowerInterval.SMA20.Entry, lowerInterval.Candle.Open, lowerInterval.SMA20.Entry))

	sellAnalysis = append(sellAnalysis, fmt.Sprintf("lowerInterval.RSI %v > 50 : %v", lowerInterval.RSI, lowerInterval.RSI > 50))

	switch opportunity.Action {
	case "BUY":
		opportunity.Stoploss = utils.TruncateFloat(opportunity.Price*0.997, 8)
		opportunity.Takeprofit = utils.TruncateFloat(opportunity.Price*1.023, 8)
	case "SELL":
		opportunity.Stoploss = utils.TruncateFloat(opportunity.Price*1.003, 8)
		opportunity.Takeprofit = utils.TruncateFloat(opportunity.Price*0.977, 8)
	}

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

func analyseOpportunity(analysis analysisType, timeframe string, price float64) (opportunity opportunityType) {
	if analysis.Pair == "" || analysis.Exchange == "" {
		return
	}

	if len(TimeframeMaps[timeframe]) < 4 {
		return
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

	interval0 := analysis.Intervals[TimeframeMaps[timeframe][0]]
	interval1 := analysis.Intervals[TimeframeMaps[timeframe][1]]
	interval2 := analysis.Intervals[TimeframeMaps[timeframe][2]]
	interval3 := analysis.Intervals[TimeframeMaps[timeframe][3]]

	if price == 0 {
		price = utils.TruncateFloat((interval3.Candle.Open+interval3.Candle.Close)/2, 8)
	}
	opportunity.Pair = analysis.Pair
	opportunity.Exchange = analysis.Exchange
	opportunity.Timeframe = timeframe
	opportunity.Price = price

	isCheckLong := checkIfLong(price, []utils.Summary{interval0, interval1, interval2, interval3})

	//Check for Long // Buy Opportunity
	if isCheckLong && strings.Contains(interval3.Pattern.Candle, "Bullish") {
		opportunity.Action = "BUY"
	}

	// -- -- --

	//Check for Short // Sell Opportunity
	isCheckShort := checkIfShort(price, []utils.Summary{interval0, interval1, interval2, interval3})
	if isCheckShort && strings.Contains(interval3.Pattern.Candle, "Bearish") {
		opportunity.Action = "SELL"
	}

	switch opportunity.Action {
	case "BUY":
		opportunity.Stoploss = interval3.SMA20.Support
		opportunity.Takeprofit = interval2.SMA10.Resistance
	case "SELL":
		opportunity.Stoploss = interval3.SMA20.Resistance
		opportunity.Takeprofit = interval2.SMA10.Support
	}

	// opportunity.Analysis = map[string]interface{}{
	// 	"Buy":  buyAnalysis,
	// 	"Sell": sellAnalysis,
	// }

	if market.Closed == 1 {
		opportunityMutex.Lock()
		pairexchange := fmt.Sprintf("%s-%s", analysis.Pair, analysis.Exchange)
		opportunityMap[pairexchange] = notifications{Title: "", Message: ""}
		opportunityMutex.Unlock()
	}

	return
}

func checkIfLong(currentPrice float64, intervals []utils.Summary) bool {
	checkLong := map[string]bool{
		"rsi":     true,
		"fib":     true,
		"trend":   true,
		"candle":  true,
		"pattern": true,
		"support": true,
	}

	for index, summary := range intervals {
		if summary.RSI == 0 {
			checkLong["rsi"] = false
			checkLong["fib"] = false
			checkLong["trend"] = false
			checkLong["candle"] = false
			checkLong["pattern"] = false
			checkLong["support"] = false
			continue
		}

		switch index {
		case 3:
			if checkLong["candle"] {
				checkLong["candle"] = currentPrice > summary.Candle.Open
			}
			if checkLong["trend"] {
				checkLong["trend"] = summary.Trend != "Bullish"
			}
			if checkLong["pattern"] {
				checkLong["pattern"] = strings.Contains(summary.Pattern.Chart, "Bullish")
			}
			if checkLong["rsi"] {
				checkLong["rsi"] = summary.RSI < 40
			}
			if checkLong["fib"] {
				checkLong["fib"] = currentPrice < summary.RetracementLevels["0.236"] && currentPrice > summary.RetracementLevels["0.786"]
			}
			if checkLong["support"] {
				checkLong["support"] = summary.SMA20.Support == summary.SMA50.Support
			}
		}
	}

	return checkLong["trend"] && checkLong["rsi"] && checkLong["fib"] && checkLong["candle"] && checkLong["pattern"] && checkLong["support"]
}

func checkIfShort(currentPrice float64, intervals []utils.Summary) bool {
	checkShort := map[string]bool{
		"rsi":        true,
		"fib":        true,
		"trend":      true,
		"candle":     true,
		"pattern":    true,
		"resistance": true,
	}

	for index, summary := range intervals {
		if summary.RSI == 0 {
			checkShort["rsi"] = false
			checkShort["fib"] = false
			checkShort["trend"] = false
			checkShort["candle"] = false
			checkShort["pattern"] = false
			checkShort["resistance"] = false
			continue
		}

		switch index {
		case 3:
			if checkShort["candle"] {
				checkShort["candle"] = currentPrice < summary.Candle.Open
			}
			if checkShort["trend"] {
				checkShort["trend"] = summary.Trend != "Bearish"
			}
			if checkShort["pattern"] {
				checkShort["pattern"] = strings.Contains(summary.Pattern.Chart, "Bearish")
			}
			if checkShort["rsi"] {
				checkShort["rsi"] = summary.RSI > 60
			}
			if checkShort["fib"] {
				checkShort["fib"] = currentPrice < summary.RetracementLevels["0.236"] && currentPrice > summary.RetracementLevels["0.786"]
			}
			if checkShort["resistance"] {
				checkShort["resistance"] = summary.SMA20.Resistance == summary.SMA50.Resistance
			}
		}
	}

	return checkShort["trend"] && checkShort["rsi"] && checkShort["fib"] && checkShort["candle"] && checkShort["pattern"] && checkShort["resistance"]
}
