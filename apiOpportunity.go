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

	TimeframeMaps = map[string]bool{
		// "3d": {"3d", "12h", "1h", "15m"},
		// "1d": {"1d", "4h", "30m", "5m"},
		// "6h": {"6h", "1h", "15m", "1m"},
		"1m":  true,
		"5m":  true,
		"15m": true,
		"30m": true,
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

	if !TimeframeMaps[timeframe] {
		timeframe = "1m"
	}
	// intervals := strings.Join(TimeframeMaps[timeframe], ",")
	intervals := timeframe
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

	if !TimeframeMaps[timeframe] {
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

	interval := analysis.Intervals[timeframe]

	if price == 0 {
		price = utils.TruncateFloat((interval.Candle.Open+interval.Candle.Close)/2, 8)
	}
	opportunity.Pair = analysis.Pair
	opportunity.Exchange = analysis.Exchange
	opportunity.Timeframe = timeframe
	opportunity.Price = price

	isCheckLong := checkIfLong(price, interval)

	//Check for Long // Buy Opportunity
	if isCheckLong && strings.Contains(interval.Pattern.Candle, "Bullish") {
		opportunity.Action = "BUY"
	}

	// -- -- --

	//Check for Short // Sell Opportunity
	isCheckShort := checkIfShort(price, interval)
	if isCheckShort && strings.Contains(interval.Pattern.Candle, "Bearish") {
		opportunity.Action = "SELL"
	}

	switch opportunity.Action {
	case "BUY":
		opportunity.Stoploss = interval.SMA20.Support
		opportunity.Takeprofit = interval.SMA20.Resistance
	case "SELL":
		opportunity.Stoploss = interval.SMA20.Resistance
		opportunity.Takeprofit = interval.SMA20.Support
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

func checkIfLong(currentPrice float64, summary utils.Summary) bool {
	checkLong := map[string]bool{
		"rsi":     true,
		"fib":     true,
		"trend":   true,
		"candle":  true,
		"pattern": true,
		"support": true,
	}

	if summary.RSI == 0 {
		return false
	}

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

	return checkLong["trend"] && checkLong["rsi"] && checkLong["fib"] && checkLong["candle"] && checkLong["pattern"] && checkLong["support"]
}

func checkIfShort(currentPrice float64, summary utils.Summary) bool {
	checkShort := map[string]bool{
		"rsi":        true,
		"fib":        true,
		"trend":      true,
		"candle":     true,
		"pattern":    true,
		"resistance": true,
	}

	if summary.RSI == 0 {
		return false
	}

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

	return checkShort["trend"] && checkShort["rsi"] && checkShort["fib"] && checkShort["candle"] && checkShort["pattern"] && checkShort["resistance"]
}
