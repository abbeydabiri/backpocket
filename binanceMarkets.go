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
)

var (
	chanRestartBinanceOHLCVMarketStream = make(chan bool, 10)
)

func binanceMarketGet() {
	for {
		httpClient := http.Client{Timeout: time.Duration(time.Second * 5)}
		httpRequest, _ := http.NewRequest("GET", binanceRestURL+"/exchangeInfo", nil)
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

		var exchangeInfo struct {
			Symbols []struct {
				Symbol, BaseAsset,
				QuoteAsset string
				Filters []struct {
					FilterType, MinQty, MaxQty,
					MinPrice, MaxPrice,
					StepSize, TickSize,
					MinNotional string
				}
			}
		}

		if err := json.Unmarshal(bodyBytes, &exchangeInfo); err != nil {
			log.Panic(string(bodyBytes))
			log.Panic(err.Error())
			return
		}

		for _, marketPair := range exchangeInfo.Symbols {

			market := getMarket(marketPair.Symbol, "binance")

			//findkey and create if it does not exist
			if market.Pair == "" {
				//this logic adds a new marketpair
				market.Pair = marketPair.Symbol
				market.Status = "disabled"
				market.Exchange = "binance"

				switch marketPair.Symbol {
				case "BNBUSDT":
					market.Status = "enabled"
				}

				market.BaseAsset = marketPair.BaseAsset
				market.QuoteAsset = marketPair.QuoteAsset

				for _, filter := range marketPair.Filters {
					switch filter.FilterType {
					case "MIN_NOTIONAL":
						var err error
						if market.MinNotional, err = strconv.ParseFloat(filter.MinNotional, 64); err != nil {
							log.Println(err.Error())
						}

					case "LOT_SIZE":
						var err error
						if market.MinQty, err = strconv.ParseFloat(filter.MinQty, 64); err != nil {
							log.Println(err.Error())
						}

						if market.MaxQty, err = strconv.ParseFloat(filter.MaxQty, 64); err != nil {
							log.Println(err.Error())
						}

						if market.StepSize, err = strconv.ParseFloat(filter.StepSize, 64); err != nil {
							log.Println(err.Error())
						}
					case "PRICE_FILTER":
						var err error
						if market.MinPrice, _ = strconv.ParseFloat(filter.MinPrice, 64); err != nil {
							log.Println(err.Error())
						}

						if market.MaxPrice, _ = strconv.ParseFloat(filter.MaxPrice, 64); err != nil {
							log.Println(err.Error())
						}

						if market.TickSize, _ = strconv.ParseFloat(filter.TickSize, 64); err != nil {
							log.Println(err.Error())
						}
					}
				}

				if market.Pair != "" {
					//save market to db

					if market.ID > 0 {
						updateMarket(market)
					} else {
						market.ID = sqlTableID()
						if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(market), reflect.ValueOf(market)); len(sqlParams) > 0 {
							_, err := utils.SqlDB.Exec(sqlQuery, sqlParams...)
							if err != nil {
								log.Println(sqlQuery)
								log.Println(sqlParams)
								log.Println(err.Error())
							}
						}
					}
					wsBroadcastMarket <- market
					//save market to db
				}
			}
		}

		// dbSetupMarkets()
		time.Sleep(time.Minute)
	}
}

func binanceMarket24hrTicker() {

	for {
		httpClient := http.Client{Timeout: time.Duration(time.Second * 10)}
		httpRequest, _ := http.NewRequest("GET", binanceRestURL+"/ticker/24hr", nil)
		httpResponse, err := httpClient.Do(httpRequest)
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 10)
			continue
		}
		defer httpResponse.Body.Close()

		bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 10)
			continue
		}

		type ticker24HrType struct {
			Symbol, PriceChange,
			PriceChangePercent,
			HighPrice, LastPrice,
			LowPrice, OpenPrice,
			AskPrice, AskQty,
			BidPrice, BidQty,
			PrevClosePrice string
			Count int
		}

		var ticker24HRList []ticker24HrType
		if err := json.Unmarshal(bodyBytes, &ticker24HRList); err != nil {
			log.Println(err.Error())
			log.Println(string(bodyBytes))
			time.Sleep(time.Second * 10)
			continue
		}

		for _, marketPair := range ticker24HRList {

			market := getMarket(marketPair.Symbol, "binance")

			//findkey and create if it does not exist
			if market.Pair != "" {
				var err error
				if market.LowPrice, _ = strconv.ParseFloat(marketPair.LowPrice, 64); err != nil {
					log.Println(err.Error())
				}

				if market.HighPrice, _ = strconv.ParseFloat(marketPair.HighPrice, 64); err != nil {
					log.Println(err.Error())
				}

				if market.PriceChange, _ = strconv.ParseFloat(marketPair.PriceChange, 64); err != nil {
					log.Println(err.Error())
				}

				if market.PriceChangePercent, _ = strconv.ParseFloat(marketPair.PriceChangePercent, 64); err != nil {
					log.Println(err.Error())
				}

				if market.Status != "enabled" {
					if market.Open, _ = strconv.ParseFloat(marketPair.OpenPrice, 64); err != nil {
						log.Println(err.Error())
					}

					if market.AskPrice, _ = strconv.ParseFloat(marketPair.AskPrice, 64); err != nil {
						log.Println(err.Error())
					}

					if market.AskQty, _ = strconv.ParseFloat(marketPair.AskQty, 64); err != nil {
						log.Println(err.Error())
					}

					if market.BidPrice, _ = strconv.ParseFloat(marketPair.BidPrice, 64); err != nil {
						log.Println(err.Error())
					}

					if market.BidQty, _ = strconv.ParseFloat(marketPair.BidQty, 64); err != nil {
						log.Println(err.Error())
					}

					if market.Close, _ = strconv.ParseFloat(marketPair.LastPrice, 64); err != nil {
						log.Println(err.Error())
					}
					market.NumOfTrades = marketPair.Count
				}
				market.LastPrice = market.Price
				market.Price = market.Close
				updateMarket(market)

			}
		}
		time.Sleep(time.Second * 10)
	}
}

func binanceMarketOHLCVStream() {

	var streamParams []string
	marketListMutex.RLock()
	for _, market := range marketList {
		// streamParams = append(streamParams, strings.ToLower(market.Pair)+"@bookTicker")
		if market.Status == "enabled" && market.Exchange == "binance" {
			streamParams = append(streamParams, strings.ToLower(market.Pair)+"@kline_1m")
		}
	}
	marketListMutex.RUnlock()
	// streamParams = append(streamParams, "!bookTicker")

	wsResp := binanceStreamKlineResp{}
	// wsResp := binanceStreamBookTickerResp{}
	bwConn := binanceWSConnect(streamParams)
	if _, _, err := bwConn.ReadMessage(); err != nil {
		log.Println("err ", err.Error())
	}

	//loop through and read all messages received
	for {
		select {
		case <-chanRestartBinanceOHLCVMarketStream:
			streamParams = nil
			marketListMutex.RLock()
			for _, market := range marketList {
				if market.Status == "enabled" && market.Exchange == "binance" {
					streamParams = append(streamParams, strings.ToLower(market.Pair)+"@kline_1m")
				}
			}
			marketListMutex.RUnlock()

			bwConn.Close()
			bwConn = binanceWSConnect(streamParams)
			if _, _, err := bwConn.ReadMessage(); err != nil {
				log.Println("err ", err.Error())
			}
		default:
		}

		if err := bwConn.ReadJSON(&wsResp); err != nil {
			log.Println("binanceMarketOHLCVStream bwCon read error:", err)
			time.Sleep(time.Second * 15)

			select {
			case chanRestartBinanceOHLCVMarketStream <- true:
			default:
			}
			continue
		}

		marketPair := strings.ToUpper(strings.Split(wsResp.Stream, "@")[0])
		if marketPair == "" {
			continue
		}

		market := getMarket(marketPair, "binance")
		if market.Pair == "" {
			continue
		}

		if wsResp.Data.Kline.Closed {
			market.Closed = 1
		} else {
			market.Closed = 0
		}
		market.NumOfTrades = wsResp.Data.Kline.NumOfTrades
		if market.NumOfTrades == 0 {
			clearTrades(market.Pair, market.Exchange)
		}

		market.LastPrice = market.Price
		market.Price = market.Close
		market.Low, _ = strconv.ParseFloat(wsResp.Data.Kline.Low, 64)
		market.Open, _ = strconv.ParseFloat(wsResp.Data.Kline.Open, 64)
		market.High, _ = strconv.ParseFloat(wsResp.Data.Kline.High, 64)
		market.Close, _ = strconv.ParseFloat(wsResp.Data.Kline.Close, 64)
		market.Volume, _ = strconv.ParseFloat(wsResp.Data.Kline.Volume, 64)
		market.VolumeQuote, _ = strconv.ParseFloat(wsResp.Data.Kline.VolumeQuote, 64)

		//set marketprice and save
		// if market.Price == 0 {
		// 	market.Price = market.Close
		// }
		// if market.LastPrice == 0 {
		// 	market.LastPrice = market.Price
		// }

		bollingerBandsMutex.Lock()
		if wsResp.Data.Kline.Closed || len(bollingerBands[market.Pair]) < 10 {
			bollingerBands[market.Pair] = append(bollingerBands[market.Pair], market.Close)
			if len(bollingerBands[market.Pair]) > 20 {
				bollingerBands[market.Pair] = bollingerBands[market.Pair][1:]
			}
		}
		bollingerBandsMutex.Unlock()
		calculateBollingerBands(&market)
		updateMarket(market)

		// select {
		// case chanStoplossTakeProfit <- getOrderbook(market.Pair, "binance"):
		// default:
		// }

		if wsResp.Data.Kline.Closed {
			updateFields := map[string]bool{
				// "low": true, "open": true, "high": true, "close": true, "volume": true, "volumequote": true, "closed": true,

				"numoftrades": true, "closed": true,
				"minqty": true, "maxqty": true, "stepsize": true,
				"minprice": true, "maxprice": true, "ticksize": true,

				"open": true, "close": true, "high": true, "low": true,
				"volume": true, "volumequote": true,
				"lastprice": true, "price": true,

				"upperband": true, "middleband": true, "lowerband": true,

				"firstbid": true, "secondbid": true, "lastbid": true,
				"firstask": true, "secondask": true, "lastask": true,
				"bidqty": true, "bidprice": true,
				"askqty": true, "askprice": true,

				"pricechange": true, "pricechangepercent": true,
				"highprice": true, "lowprice": true,
			}

			if sqlQuery, sqlParams := sqlTableUpdate(reflect.TypeOf(market), reflect.ValueOf(market), updateFields); len(sqlParams) > 0 {
				utils.SqlDB.Exec(sqlQuery, sqlParams...)
			}
		}

	}
	//loop through and read all messages received
}
