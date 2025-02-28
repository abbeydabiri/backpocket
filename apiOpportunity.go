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
		"3d":  {"3d", "1w", "1M"},
		"1d":  {"1d", "3d", "1w"},
		"4h":  {"4h", "1d", "1w"},
		"2h":  {"2h", "6h", "3d"},
		"1h":  {"1h", "4h", "3d"},
		"30m": {"30m", "2h", "1d"},
		"15m": {"15m", "1h", "4h"},
		"5m":  {"5m", "30m", "1h"},
		"3m":  {"3m", "15m", "30m"},
		"1m":  {"1m", "5m", "15m"},
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

	if len(TimeframeMaps[timeframe]) != 3 {
		timeframe = "1m"
	}

	intervals := strings.Join(TimeframeMaps[timeframe], ",") + ",1m"
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

func analyseOpportunity(analysis analysisType, timeframe string, price float64) (opportunity opportunityType) {
	if analysis.Pair == "" || analysis.Exchange == "" {
		return
	}

	if len(TimeframeMaps[timeframe]) != 3 {
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

	interval1m := analysis.Intervals["1m"]
	lowerInterval := analysis.Intervals[TimeframeMaps[timeframe][0]]
	middleInterval := analysis.Intervals[TimeframeMaps[timeframe][1]]
	upperInterval := analysis.Intervals[TimeframeMaps[timeframe][2]]

	if price == 0 {
		price = interval1m.Candle.Close
	}
	opportunity.Pair = analysis.Pair
	opportunity.Exchange = analysis.Exchange
	opportunity.Timeframe = timeframe
	opportunity.Price = price

	isCheckLong := checkIfLong(price, lowerInterval, middleInterval, upperInterval)

	//Check for Long // Buy Opportunity
	if isCheckLong {
		opportunity.Action = "BUY"
	}

	// -- -- --

	//Check for Short // Sell Opportunity
	isCheckShort := checkIfShort(price, lowerInterval, middleInterval, upperInterval)
	if isCheckShort {
		opportunity.Action = "SELL"
	}

	switch opportunity.Action {
	case "BUY":
		opportunity.Stoploss = utils.TruncateFloat(price*0.98, 8)
		opportunity.Takeprofit = utils.TruncateFloat(price*1.05, 8)
	case "SELL":
		opportunity.Stoploss = utils.TruncateFloat(price*1.02, 8)
		opportunity.Takeprofit = utils.TruncateFloat(price*0.95, 8)
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

func checkIfLong(currentPrice float64, summaryLower, summaryMiddle, summaryUpper utils.Summary) bool {
	checkLong := map[string]bool{
		"rsi":       true,
		"fib":       true,
		"trend":     true,
		"support":   true,
		"bollinger": true,
	}

	if summaryLower.RSI == 0 || summaryUpper.RSI == 0 || summaryMiddle.RSI == 0 {
		return false
	}

	if checkLong["rsi"] {
		checkLong["rsi"] = summaryLower.RSI < 50 &&
			summaryMiddle.RSI < 50 &&
			summaryUpper.RSI < 50
	}
	if checkLong["fib"] {
		checkLong["fib"] = currentPrice < summaryLower.RetracementLevels["0.786"] &&
			currentPrice < summaryMiddle.RetracementLevels["0.786"] &&
			currentPrice < summaryUpper.RetracementLevels["0.786"]
	}
	if checkLong["trend"] {
		checkLong["trend"] = summaryLower.Trend == "Bearish" &&
			summaryMiddle.Trend == "Bearish" &&
			summaryUpper.Trend == "Bearish"
	}

	if checkLong["bollinger"] {
		checkLong["bollinger"] = summaryLower.Candle.Low < summaryLower.BollingerBands["lower"]
	}

	checkLong["support"] =
		summaryLower.SMA50.Support == summaryMiddle.SMA50.Support &&
			summaryLower.SMA50.Support == summaryUpper.SMA50.Support

	return checkLong["fib"] && checkLong["bollinger"] && checkLong["trend"] && checkLong["rsi"] && checkLong["support"]
}

func checkIfShort(currentPrice float64, summaryLower, summaryMiddle, summaryUpper utils.Summary) bool {
	checkShort := map[string]bool{
		"rsi":        true,
		"fib":        true,
		"trend":      true,
		"bollinger":  true,
		"resistance": true,
	}

	if summaryLower.RSI == 0 || summaryUpper.RSI == 0 || summaryMiddle.RSI == 0 {
		return false
	}

	if checkShort["rsi"] {
		checkShort["rsi"] = summaryLower.RSI > 50 &&
			summaryMiddle.RSI > 50 &&
			summaryUpper.RSI > 50
	}
	if checkShort["fib"] {
		checkShort["fib"] = currentPrice > summaryLower.RetracementLevels["0.236"] &&
			currentPrice > summaryMiddle.RetracementLevels["0.236"] &&
			currentPrice > summaryUpper.RetracementLevels["0.236"]
	}
	if checkShort["trend"] {
		checkShort["trend"] = summaryLower.Trend == "Bullish" &&
			summaryMiddle.Trend == "Bullish" &&
			summaryUpper.Trend == "Bullish"
	}

	if checkShort["bollinger"] {
		checkShort["bollinger"] = summaryLower.Candle.High > summaryLower.BollingerBands["upper"]
	}

	checkShort["resistance"] =
		summaryLower.SMA50.Resistance == summaryMiddle.SMA50.Resistance &&
			summaryLower.SMA50.Resistance == summaryUpper.SMA50.Resistance

	return checkShort["fib"] && checkShort["bollinger"] && checkShort["trend"] && checkShort["rsi"] && checkShort["resistance"]
}
