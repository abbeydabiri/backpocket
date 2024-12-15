package main

import (
	"backpocket/utils"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/x2v3/signalr"
	"github.com/x2v3/signalr/hubs"
)

var (
	chanRestartCrex24Market24HRTickerStream = make(chan bool, 10)
)

func crex24MarketGet() {

	httpClient := http.Client{Timeout: time.Duration(time.Second * 30)}
	httpRequest, _ := http.NewRequest("GET", crex24RestURL+"/v2/public/instruments", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		log.Panic(err.Error())
		return
	}
	defer httpResponse.Body.Close()

	bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		log.Panic(err.Error())
		return
	}

	var exchangeInfo []struct {
		Symbol, BaseCurrency,
		QuoteCurrency, FeeCurrency,
		FeeSchedule, State string

		TickSize, MinPrice, MaxPrice,
		VolumeIncrement, MinVolume,
		MaxVolume, MinQuoteVolume,
		MaxQuoteVolume float64

		SupportedOrderTypes []string
	}

	if err := json.Unmarshal(bodyBytes, &exchangeInfo); err != nil {
		// log.Panic(string(bodyBytes))
		log.Panic(err.Error())
		return
	}

	for _, exchangePair := range exchangeInfo {
		if exchangePair.State != "active" {
			continue
		}

		market := getMarket(exchangePair.Symbol, "crex24")

		if market.Pair != "" {
			continue
		}

		//findkey and create if it does not exist

		//this logic adds a new marketpair
		market.Pair = exchangePair.Symbol
		market.Status = "disabled"
		market.Exchange = "crex24"

		switch exchangePair.Symbol {
		case "FLASH-BTC", "LTC-BTC", "LTC-USDT", "BTC-USDT":
			market.Status = "enabled"
		}

		market.BaseAsset = exchangePair.BaseCurrency
		market.QuoteAsset = exchangePair.QuoteCurrency

		market.MinQty = exchangePair.MinVolume
		market.MaxQty = exchangePair.MaxVolume
		market.StepSize = exchangePair.VolumeIncrement
		market.MinPrice = exchangePair.MinPrice
		market.MaxPrice = exchangePair.MaxPrice
		market.TickSize = exchangePair.TickSize
		market.MinNotional = exchangePair.MinQuoteVolume

		if market.Pair != "" {
			market.ID = sqlTableID()
			//save market to db
			if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(market), reflect.ValueOf(market)); len(sqlParams) > 0 {
				utils.SqlDB.Exec(sqlQuery, sqlParams...)
			}
			//save market to db
		}
	}
}

func crex24Market24hrTicker() {

	for {
		httpClient := http.Client{Timeout: time.Duration(time.Second * 30)}
		httpRequest, _ := http.NewRequest("GET", crex24RestURL+"/v2/public/tickers", nil)
		httpResponse, err := httpClient.Do(httpRequest)
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 15)
			continue
		}
		defer httpResponse.Body.Close()

		bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 15)
			continue
		}

		type ticker24HrType struct {
			Instrument, Timestamp string

			Last, PercentChange,
			Low, High, BaseVolume,
			QuoteVolume, Ask,
			Bid float64
		}

		var ticker24HRList []ticker24HrType
		if err := json.Unmarshal(bodyBytes, &ticker24HRList); err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 15)
			continue
		}

		for _, marketPair := range ticker24HRList {

			market := getMarket(marketPair.Instrument, "crex24")

			//findkey and create if it does not exist
			if market.Pair != "" {

				market.LowPrice = marketPair.Low
				market.HighPrice = marketPair.High
				market.PriceChange = (marketPair.PercentChange / 100) * marketPair.Last
				market.PriceChangePercent = marketPair.PercentChange

				if market.Status != "enabled" {
					market.Price = marketPair.Last
					market.LastPrice = marketPair.Last

					market.AskPrice = marketPair.Ask
					market.BidPrice = marketPair.Bid
				}

				select {
				case wsBroadcastMarket <- market:
				default:
				}
				updateMarket(market)

				// if market.Status != "enabled" {
				// var bidsList []bidAskStruct
				// var asksList []bidAskStruct

				// bidsList = append(bidsList, bidAskStruct{Price: market.Price})
				// asksList = append(asksList, bidAskStruct{Price: market.Price})

				// orderbook := orderbooks{
				// 	Pair: market.Pair, Exchange: market.Exchange, TickSize: market.TickSize,
				// 	QuoteAsset: market.QuoteAsset, BaseAsset: market.BaseAsset,
				// 	Asks: asksList, Bids: bidsList,
				// }

				// select {
				// case chanStoplossTakeProfit <- orderbook:
				// default:
				// }
				// }
			}
		}
		time.Sleep(time.Minute * 3)
	}
}

func crex24Market24hrTickerStream() {

	updateMarketTicker := func(instrument, lastprice, change, low, high, basevol, quotevol string) {
		market := getMarket(instrument, "crex24")
		if market.Pair == "" {
			return
		}

		market.LowPrice, _ = strconv.ParseFloat(low, 64)
		market.HighPrice, _ = strconv.ParseFloat(high, 64)
		market.LastPrice, _ = strconv.ParseFloat(lastprice, 64)
		market.PriceChangePercent, _ = strconv.ParseFloat(change, 64)
		market.PriceChange = (market.PriceChangePercent / 100) * market.LastPrice
		updateMarket(market)

		// if market.Status != "enabled" {
		bidsList := make([]bidAskStruct, 1)
		asksList := make([]bidAskStruct, 1)

		bidsList = append(bidsList, bidAskStruct{Price: market.LastPrice})
		asksList = append(asksList, bidAskStruct{Price: market.LastPrice})

		orderbook := orderbooks{
			Pair: market.Pair, Exchange: market.Exchange, TickSize: market.TickSize,
			QuoteAsset: market.QuoteAsset, BaseAsset: market.BaseAsset,
			Asks: asksList, Bids: bidsList,
		}

		select {
		case chanStoplossTakeProfit <- orderbook:
		default:
		}
		// }
	}

	//make a connection receive a signalR
	// wsResp := crex24WSTickerResp{}

	wsRespHandler := func(tickerMessage signalr.Message) {
		if len(tickerMessage.M) == 0 {
			return
		}

		crex24Ticker := crex24WSTickerResp{}
		signalrMessage := tickerMessage.M[0]
		json.Unmarshal([]byte(signalrMessage.A[1].(string)), &crex24Ticker)

		for _, tickerResp := range crex24Ticker.LST {
			updateMarketTicker(tickerResp.I, tickerResp.LST, tickerResp.PC, tickerResp.L, tickerResp.H, tickerResp.BV, tickerResp.QV)
		}

		var lastprice, change, low, high, basevol, quotevol string
		for _, tickerResp := range crex24Ticker.U {
			for _, tickerUpdate := range tickerResp.U {
				switch tickerUpdate.N {
				case "LST":
					lastprice = tickerUpdate.V
				case "PC":
					change = tickerUpdate.V
				case "L":
					low = tickerUpdate.V
				case "H":
					high = tickerUpdate.V
				case "BV":
					basevol = tickerUpdate.V
				case "QV":
					quotevol = tickerUpdate.V
				}
			}
			updateMarketTicker(tickerResp.I, lastprice, change, low, high, basevol, quotevol)
			low = ""
			high = ""
			change = ""
			basevol = ""
			quotevol = ""
			lastprice = ""
		}
	}
	logIfErr := func(err error) {
		if err != nil {
			log.Println(err)
			return
		}
	}
	//create the logic function to handle signalR messages
	//start the connection

	channelMethod := "Tickers"
	var signalRClient *signalr.Client
	chanRestartCrex24Market24HRTickerStream <- true

	for range chanRestartCrex24Market24HRTickerStream {
		err = signalRClient.Send(hubs.ClientMsg{
			H: crex24SignalrHub,
			M: "leave" + channelMethod,
		})
		if signalRClient.Close(); err != nil {
			log.Println(err.Error())
		}

		//subscribe to the channels
		signalRClient = crex24WSConnect()
		logIfErr(signalRClient.Run(wsRespHandler, logIfErr))
		signalRClient.Send(hubs.ClientMsg{
			H: crex24SignalrHub,
			M: "join" + channelMethod,
		})
	}

}

func crex24MarketOHLCVStream() {
	for {

		var feedParams []string
		marketListMutex.RLock()
		for _, market := range marketList {
			if market.Status == "enabled" && market.Exchange == "crex24" {
				feedParams = append(feedParams, strings.ToUpper(market.Pair))
			}
		}
		marketListMutex.RUnlock()

		for _, instrument := range feedParams {

			//=--> OHCLV DATA - CANDLESTICK DATA PER MINUTE
			httpClient := http.Client{Timeout: time.Duration(time.Second * 30)}
			httpRequest, _ := http.NewRequest("GET", crex24RestURL+"/v2/public/ohlcv?granularity=1m&limit=1&instrument="+instrument, nil)
			httpResponse, err := httpClient.Do(httpRequest)
			if err != nil {
				log.Println(err.Error())
				continue
			}
			defer httpResponse.Body.Close()

			bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
			if err != nil {
				log.Println(err.Error())
				continue
			}

			type crex24OHLCVType struct {
				Timestamp string
				Volume, Open, High,
				Low, Close float64
			}

			var ohclvData []crex24OHLCVType
			if err := json.Unmarshal(bodyBytes, &ohclvData); err != nil {
				log.Println(err.Error())
				time.Sleep(time.Second * 15)
				continue
			}

			ohclv := ohclvData[0]
			market := getMarket(instrument, "crex24")

			if market.Pair != "" {
				if ohclv.Open == ohclv.Close {
					market.Closed = 1
					market.NumOfTrades = 0
				} else {
					market.Closed = 0
				}

				market.Low = ohclv.Low
				market.Open = ohclv.Open
				market.High = ohclv.High
				market.Close = ohclv.Close
				market.Price = ohclv.Close
				market.Volume = ohclv.Volume
				market.VolumeQuote = ohclv.Volume * ohclv.Close

				// set marketprice and save
				if market.Price == 0.0 {
					market.Price = market.Close
				}

				bollingerBandsMutex.Lock()
				if market.Closed == 1 || len(bollingerBands[market.Pair]) < 20 {
					bollingerBands[market.Pair] = append(bollingerBands[market.Pair], market.Close)
					if len(bollingerBands[market.Pair]) > 20 {
						bollingerBands[market.Pair] = bollingerBands[market.Pair][1:]
					}
				}
				marketRSIBands[market.Pair] = append(marketRSIBands[market.Pair], market.Close)
				if len(marketRSIBands[market.Pair]) > 14 {
					marketRSIBands[market.Pair] = marketRSIBands[market.Pair][1:]
				}
				bollingerBandsMutex.Unlock()
				calculateBollingerBands(&market)

				marketRSIBandsMutex.Lock()
				if market.Closed == 1 || len(marketRSIBands[market.Pair]) < 14 {
					marketRSIBands[market.Pair] = append(marketRSIBands[market.Pair], market.Price)
					if len(marketRSIBands[market.Pair]) > 14 {
						marketRSIBands[market.Pair] = marketRSIBands[market.Pair][1:]
					}
				}
				marketRSIBandsMutex.Unlock()
				calculateRSIBands(&market)
				updateMarket(market)

				if market.Status == "enabled" {
					select {
					case wsBroadcastMarket <- market:
					default:
					}
				}

				if market.Closed == 1 {
					updateFields := map[string]bool{
						"low": true, "open": true, "high": true, "close": true, "volume": true, "volumequote": true, "closed": true,
					}

					if sqlQuery, sqlParams := sqlTableUpdate(reflect.TypeOf(market), reflect.ValueOf(market), updateFields); len(sqlParams) > 0 {
						utils.SqlDB.Exec(sqlQuery, sqlParams...)
					}
				}
			}
			// OHCLV DATA - CANDLESTICK DATA PER MINUTE
		}

		time.Sleep(time.Second * 15)
	}
}
