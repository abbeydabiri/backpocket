package main

import (
	"backpocket/models"
	"backpocket/utils"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type analysisType struct {
	Pair      string
	Trend     string
	Exchange  string
	Intervals map[string]utils.Summary
}

var (
	analysisList    []analysisType
	analysisListMap = make(map[string]int)

	analysisListMutex    = sync.RWMutex{}
	analysisListMapMutex = sync.RWMutex{}
	wsConnAnalysisMutex  = sync.RWMutex{}

	wsConnAnalysis      = make(map[*websocket.Conn]bool)
	wsBroadcastAnalysis = make(chan analysisType, 10240)
)

func getAnalysis(analysisPair, analysisExchange string) (analysis analysisType) {
	analysisKey := 0
	analysisListMapMutex.RLock()
	for pair, key := range analysisListMap {
		if pair == fmt.Sprintf("%s-%s", analysisPair, strings.ToLower(analysisExchange)) {
			analysisKey = key
		}
	}
	analysisListMapMutex.RUnlock()

	analysisListMutex.RLock()
	if analysisKey > 0 && len(analysisList) > (analysisKey-1) {
		analysis = analysisList[analysisKey-1]
		// copier.Copy(&analysis, analysisList[analysisKey-1])
	}
	analysisListMutex.RUnlock()
	return
}

func wsHandlerAnalysis(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		analysisListMutex.RLock()
		for _, analysis := range analysisList {
			wsConn.WriteJSON(analysis)
		}
		analysisListMutex.RUnlock()

		wsConnAnalysisMutex.Lock()
		wsConnAnalysis[wsConn] = true
		wsConnAnalysisMutex.Unlock()

	}
}

func wsHandlerAnalysisBroadcast() {
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			wsConnAnalysisMutex.Lock()
			for wsConn := range wsConnAnalysis {
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					delete(wsConnAnalysis, wsConn)
					wsConn.Close()
				}
			}
			wsConnAnalysisMutex.Unlock()
		}
	}()

	go func() {
		ticker := time.NewTicker(time.Second * 15)
		defer ticker.Stop()
		for ; true; <-ticker.C {
			analysisListMutex.RLock()
			for _, analysis := range analysisList {
				wsBroadcastAnalysis <- analysis
			}
			analysisListMutex.RUnlock()
		}
	}()

	go func() {
		for analysis := range wsBroadcastAnalysis {
			wsConnAnalysisMutex.Lock()
			for wsConn := range wsConnAnalysis {
				if err := wsConn.WriteJSON(analysis); err != nil {
					delete(wsConnAnalysis, wsConn)
					wsConn.Close()
				}
			}
			wsConnAnalysisMutex.Unlock()
		}
	}()
}

func updateAnalysis(analysis analysisType) {
	if analysis.Pair == "" {
		return
	}

	pairKey := 0
	analysisListMapMutex.RLock()
	for pair, key := range analysisListMap {
		if pair == fmt.Sprintf("%s-%s", analysis.Pair, strings.ToLower(analysis.Exchange)) {
			pairKey = key
		}
	}
	analysisListMapMutex.RUnlock()

	if pairKey == 0 {
		analysisListMutex.Lock()
		analysisList = append(analysisList, analysis)
		pairKey = len(analysisList)
		analysisListMutex.Unlock()

		analysisListMapMutex.Lock()
		analysisListMap[fmt.Sprintf("%s-%s", analysis.Pair, strings.ToLower(analysis.Exchange))] = pairKey
		analysisListMapMutex.Unlock()
	} else {
		analysisListMutex.Lock()
		analysisList[pairKey-1] = analysis
		analysisListMutex.Unlock()
	}
}

func GoFetchEnabledMarketsAnalysis() {

	var analysisMarkets []models.Market

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for ; true; <-ticker.C {

		marketListMutex.RLock()
		analysisMarkets = []models.Market{}
		for _, market := range marketList {
			if market.Status == "enabled" {
				analysisMarkets = append(analysisMarkets, market)
			}
		}
		marketListMutex.RUnlock()

		for _, market := range analysisMarkets {
			analysis, err := retrieveMarketPairAnalysis(market.Pair, market.Exchange, "", "", "", "")
			if err != nil {
				log.Println(err.Error())
				continue
			}
			updateAnalysis(analysis)
		}
	}

}

func restHandlerAnalysis(httpRes http.ResponseWriter, httpReq *http.Request) {
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

	if pair == "" {
		http.Error(httpRes, "Missing pair parameter", http.StatusBadRequest)
		return
	}

	analysis, err := retrieveMarketPairAnalysis(pair, exchange, limit, endTime, startTime, intervals)
	if err != nil {
		http.Error(httpRes, err.Error(), http.StatusInternalServerError)
		return
	}

	httpRes.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(analysis)
	if err != nil {
		http.Error(httpRes, "Error converting to JSON", http.StatusInternalServerError)
		return
	}

	httpRes.Write(jsonResponse)
}

func retrieveMarketPairAnalysis(pair, exchange, limit, endTime, startTime, intervals string) (analysis analysisType, err error) {
	if intervals == "" {
		intervals = "1m,3m,5m,15m,30m,4h,1d"
	}

	analysis.Pair = pair
	analysis.Exchange = exchange

	request, errSub := prepareKlineRequest(pair, exchange, intervals, limit, startTime, endTime)
	if errSub != nil {
		err = fmt.Errorf("Error preparing request: for pair: %s | exchange: %s -> %v", pair, exchange, errSub)
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
		return
	}

	analysis.Intervals = make(map[string]utils.Summary)
	for interval, klines := range candlesticks {
		data := utils.MarketData{}
		for _, kline := range klines {
			data.Close = append(data.Close, kline.Close)
			data.High = append(data.High, kline.High)
			data.Low = append(data.Low, kline.Low)
			data.Open = append(data.Open, kline.Open)
			data.Volume = append(data.Volume, kline.Volume)
		}

		summary, errSub := utils.TradingSummary(pair, interval, data)
		if errSub != nil {
			log.Println(err.Error())
			continue
		}
		analysis.Intervals[interval] = summary

		if DefaultTimeframe == interval {
			marketRSIPricesMutex.Lock()
			rsimapkey := fmt.Sprintf("%s-%s", pair, exchange)
			marketRSIPrices[rsimapkey] = data.Close
			marketRSIPricesMutex.Unlock()
		}
	}

	//calculate overall time frame trends
	//
	closePrice := analysis.Intervals[DefaultTimeframe].Candle.Close
	analysis.Trend = utils.TimeframeTrends(analysis.Intervals, closePrice)

	return
}
