package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

//https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#klinecandlestick-data
// sample response
// [
//     1499040000000,      // Kline open time
//     "0.01634790",       // Open price
//     "0.80000000",       // High price
//     "0.01575800",       // Low price
//     "0.01577100",       // Close price
//     "148976.11427815",  // Volume
//     1499644799999,      // Kline Close time
//     "2434.19055334",    // Quote asset volume
//     308,                // Number of trades
//     "1756.87402397",    // Taker buy base asset volume
//     "28.46694368",      // Taker buy quote asset volume
//     "0"                 // Unused field, ignore.
//   ]

func binanceKlines(intervals []string, symbol, startTime, endTime string, limit int) (candlesticks map[string][]TypeKline) {

	candlesticks = make(map[string][]TypeKline)
	if symbol == "" || len(intervals) == 0 {
		return
	}

	if limit == 0 {
		limit = 250
	}

	// timeZoneOffset := time.Now().UTC().Unix() - time.Now().Unix()
	// timeZone := fmt.Sprintf("%+03d", timeZoneOffset/3600)
	// timeZone := "+1:00"

	loc, _ := time.LoadLocation("CET")

	startTimeInt := 0
	var parsedTime time.Time
	var err error
	if startTime != "" {
		parsedTime, err = time.ParseInLocation(time.DateTime, startTime, loc)
		if err != nil {
			log.Println(err.Error())
			return
		}
		startTimeInt = int(parsedTime.UnixNano() / int64(time.Millisecond))
	}

	endTimeInt := 0
	if endTime != "" {
		parsedTime, err = time.ParseInLocation(time.DateTime, endTime, loc)
		if err != nil {
			log.Println(err.Error())
			return
		}
		endTimeInt = int(parsedTime.UnixNano() / int64(time.Millisecond))
	}

	paramsQuery := fmt.Sprintf("symbol=%s&limit=%d", symbol, limit)

	if startTimeInt > 0 {
		paramsQuery += fmt.Sprintf("&startTime=%d", startTimeInt)
	}

	if endTimeInt > 0 {
		paramsQuery += fmt.Sprintf("&endTime=%d", endTimeInt)
	}

	httpClient := http.Client{Timeout: time.Duration(time.Second * 30)}

	for _, interval := range intervals {
		var klines []TypeKline

		paramsQueryInterval := fmt.Sprintf("%s&interval=%s", paramsQuery, interval)

		httpRequest, _ := http.NewRequest("GET", binanceRestURL+"/klines?"+paramsQueryInterval, nil)
		httpResponse, err := httpClient.Do(httpRequest)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		defer httpResponse.Body.Close()

		bodyBytes, err := io.ReadAll(httpResponse.Body)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		var klinesData [][]interface{}
		if err := json.Unmarshal(bodyBytes, &klinesData); err != nil {
			log.Println("paramsQuery: ", paramsQuery)
			log.Println(string((bodyBytes)))
			log.Println(err.Error())
			return
		}

		for _, kline := range klinesData {
			var k TypeKline

			if openTime, ok := kline[0].(float64); ok {
				k.Timestamp = time.Unix(0, int64(openTime)*int64(time.Millisecond))
			}

			if openPrice, ok := kline[1].(string); ok {
				k.Open, _ = strconv.ParseFloat(openPrice, 64)
			}

			if highPrice, ok := kline[2].(string); ok {
				k.High, _ = strconv.ParseFloat(highPrice, 64)
			}

			if lowPrice, ok := kline[3].(string); ok {
				k.Low, _ = strconv.ParseFloat(lowPrice, 64)
			}

			if closePrice, ok := kline[4].(string); ok {
				k.Close, _ = strconv.ParseFloat(closePrice, 64)
			}

			if volume, ok := kline[5].(string); ok {
				k.Volume, _ = strconv.ParseFloat(volume, 64)
			}

			if quoteVolume, ok := kline[7].(string); ok {
				k.QuoteVolume, _ = strconv.ParseFloat(quoteVolume, 64)
			}

			if numberOfTrades, ok := kline[8].(float64); ok {
				k.NumberOfTrades = int(numberOfTrades)
			}

			klines = append(klines, k)
		}
		candlesticks[interval] = klines
	}
	return
}
