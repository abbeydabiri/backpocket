package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TypeKline struct {
	Timestamp      time.Time `json:"timestamp"`
	Open           float64   `json:"open"`
	High           float64   `json:"high"`
	Low            float64   `json:"low"`
	Close          float64   `json:"close"`
	Volume         float64   `json:"volume"`
	QuoteVolume    float64   `json:"quotevolume"`
	NumberOfTrades int       `json:"numberoftrades"`
}

func restHandlerKline(httpRes http.ResponseWriter, httpReq *http.Request) {
	query := httpReq.URL.Query()

	pair := query.Get("pair")
	exchange := query.Get("exchange")
	intervals := query.Get("intervals")
	limit := query.Get("limit")
	startTime := query.Get("starttime")
	endTime := query.Get("endtime")

	if exchange == "" {
		exchange = "binance"
	}

	if intervals == "" {
		intervals = "15m"
	}

	if pair == "" {
		http.Error(httpRes, "Missing parameters pair, intervals", http.StatusBadRequest)
		return
	}

	request, err := prepareKlineRequest(pair, exchange, intervals, limit, startTime, endTime)
	if err != nil {
		http.Error(httpRes, err.Error(), http.StatusInternalServerError)
		return
	}

	candlesticks := make(map[string][]TypeKline)
	switch request.Exchange {
	case "binance":
		candlesticks = binanceKlines(request.Intervals, request.Pair,
			request.StartTime, request.EndTime, request.Limit)
	}

	if len(candlesticks) == 0 {
		err = fmt.Errorf("No data found for pair: %s | exchange: %s", pair, exchange)
		http.Error(httpRes, err.Error(), http.StatusInternalServerError)
		return
	}

	httpRes.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(candlesticks)
	if err != nil {
		http.Error(httpRes, "Error converting to JSON", http.StatusInternalServerError)
		return
	}

	httpRes.Write(jsonResponse)
}

type klineRequest struct {
	Intervals []string
	Pair, Exchange,
	StartTime, EndTime string
	Limit int
}

func prepareKlineRequest(pair, exchange, intervals, limit, starttime, endtime string) (request klineRequest, err error) {

	request.Exchange = exchange
	if request.Exchange == "" {
		request.Exchange = "binance"
	}

	if intervals == "" || pair == "" {
		err = fmt.Errorf("Missing parameters pair, intervals, limit")
		return
	}

	request.Pair = pair
	request.EndTime = endtime
	request.StartTime = starttime

	if limit != "" {
		if request.Limit, err = strconv.Atoi(limit); err != nil {
			err = fmt.Errorf("Invalid limit parameter")
			return
		}
	} else {
		request.Limit = 60
	}

	// Validate klineType
	validKlineTypes := map[string]bool{
		"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
		"1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true,
		"1d": true, "3d": true, "1w": true, "1M": true,
	}

	intervalError := ""
	request.Intervals = strings.Split(intervals, ",")
	for _, interval := range request.Intervals {
		if !validKlineTypes[interval] {
			intervalError += "[" + interval + "] "
		}
	}

	if intervalError != "" {
		err = fmt.Errorf("Invalid kline type " + intervalError)
	}

	return
}
