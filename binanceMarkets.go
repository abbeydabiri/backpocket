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
	"sync"
	"time"
)

var (
	chanRestartBinanceOHLCVMarketStream = make(chan bool, 10)
)

func binanceMarketGet(wg *sync.WaitGroup) {
	lFirstRun := true
	for {
		httpClient := http.Client{Timeout: time.Duration(time.Second * 5)}
		httpRequest, _ := http.NewRequest("GET", binanceRestURL+"/exchangeInfo", nil)
		httpResponse, err := httpClient.Do(httpRequest)
		if err != nil {
			log.Printf(err.Error())
			time.Sleep(time.Minute * 5)
			continue
		}
		defer httpResponse.Body.Close()

		bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			log.Printf(err.Error())
			time.Sleep(time.Minute * 5)
			continue
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

		sqlBatchMarkets := []markets{}
		// sqlBatchInsert := `insert into markets (id,pair,status,exchange,numoftrades,closed,baseasset,quoteasset,takeprofit,stoploss,minnotional,minqty,maxqty,stepsize,minprice,maxprice,ticksize,open,close,high,low,volume,volumequote,lastprice,price,upperband,middleband,lowerband,firstbid,secondbid,lastbid,firstask,secondask,lastask,bidqty,bidprice,askqty,askprice,pricechange,pricechangepercent,highprice,lowprice)
		// values (:id, :pair, :status, :exchange, :num_of_trades, :closed, :base_asset, :quote_asset  ,:take_profit, :stop_loss, :min_notional, :min_qty, :max_qty, :step_size, :min_price, :max_price, :tick_size, :open, :close, :high, :low, :volume, :volume_quote, :last_price, :price, :upper_band, :middle_band, :lower_band, :first_bid, :second_bid, :last_bid, :first_ask, :second_ask, :last_ask, :bid_qty, :bid_price, :ask_qty, :ask_price, :price_change, :price_change_percent, :high_price, :low_price )`
		sqlBatchInsert := `insert into markets (id,pair,status,exchange,numoftrades,closed,baseasset,quoteasset,takeprofit,stoploss,minnotional,minqty,maxqty,stepsize,minprice,maxprice,ticksize,open,close,high,low,volume,volumequote,lastprice,price,upperband,middleband,lowerband,firstbid,secondbid,lastbid,firstask,secondask,lastask,bidqty,bidprice,askqty,askprice,pricechange,pricechangepercent,highprice,lowprice)
		values (:id, :pair, :status, :exchange, :numoftrades, :closed, :baseasset, :quoteasset  ,:takeprofit, :stoploss, :minnotional, :minqty, :maxqty, :stepsize, :minprice, :maxprice, :ticksize, :open, :close, :high, :low, :volume, :volumequote, :lastprice, :price, :upperband, :middleband, :lowerband, :firstbid, :secondbid, :lastbid, :firstask, :secondask, :lastask, :bidqty, :bidprice, :askqty, :askprice, :pricechange, :pricechangepercent, :highprice, :lowprice )`

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
					if market.ID == 0 {
						if !lFirstRun {
							log.Printf("Market is Missing ID after First Run: %+v /n", market)
						}

						market.ID = sqlTableID()
						sqlBatchMarkets = append(sqlBatchMarkets, market)
					}
					updateMarket(market)
					wsBroadcastMarket <- market
					//save market to db
				}
			}
		}

		if lFirstRun {
			sqlTempBatch := []markets{}
			for _, market := range sqlBatchMarkets {
				sqlTempBatch = append(sqlTempBatch, market)
				if len(sqlTempBatch) == 100 {
					go func() {
						if _, err := utils.SqlDB.NamedExec(sqlBatchInsert, sqlTempBatch); err != nil {
							log.Println("sqlBatchMarkets Error:", err.Error())
						}
					}()
					time.Sleep(time.Millisecond * 200)
					sqlTempBatch = nil
				}
			}
			wg.Done()
		}
		lFirstRun = false

		time.Sleep(time.Minute * 5)
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
				if market.LowPrice, err = strconv.ParseFloat(marketPair.LowPrice, 64); err != nil {
					log.Println(err.Error())
				}

				if market.HighPrice, err = strconv.ParseFloat(marketPair.HighPrice, 64); err != nil {
					log.Println(err.Error())
				}

				if market.PriceChange, err = strconv.ParseFloat(marketPair.PriceChange, 64); err != nil {
					log.Println(err.Error())
				}

				if market.PriceChangePercent, err = strconv.ParseFloat(marketPair.PriceChangePercent, 64); err != nil {
					log.Println(err.Error())
				}

				if market.Status != "enabled" {
					if market.Open, err = strconv.ParseFloat(marketPair.OpenPrice, 64); err != nil {
						log.Println(err.Error())
					}

					if market.AskPrice, err = strconv.ParseFloat(marketPair.AskPrice, 64); err != nil {
						log.Println(err.Error())
					}

					if market.AskQty, err = strconv.ParseFloat(marketPair.AskQty, 64); err != nil {
						log.Println(err.Error())
					}

					if market.BidPrice, err = strconv.ParseFloat(marketPair.BidPrice, 64); err != nil {
						log.Println(err.Error())
					}

					if market.BidQty, err = strconv.ParseFloat(marketPair.BidQty, 64); err != nil {
						log.Println(err.Error())
					}

					if market.Close, err = strconv.ParseFloat(marketPair.LastPrice, 64); err != nil {
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

		_, wsRespBytes, _ := bwConn.ReadMessage()
		if err := json.Unmarshal(wsRespBytes, &wsResp); err != nil {
			// if err := bwConn.ReadJSON(&wsResp); err != nil {
			log.Println("binanceMarketOHLCVStream bwCon read error:", err)
			log.Println("wsRespBytes:", string(wsRespBytes))
			time.Sleep(time.Second * 10)

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
		if wsResp.Data.Kline.Closed || len(bollingerBands[market.Pair]) < 20 {
			bollingerBands[market.Pair] = append(bollingerBands[market.Pair], market.Close)
			if len(bollingerBands[market.Pair]) > 30 {
				bollingerBands[market.Pair] = bollingerBands[market.Pair][1:]
			}
		}
		bollingerBandsMutex.Unlock()
		calculateBollingerBands(&market)

		bollingerBandsMutex.Lock()
		marketRSIBands[market.Pair] = append(marketRSIBands[market.Pair], market.Price)
		if len(marketRSIBands[market.Pair]) > 14 {
			marketRSIBands[market.Pair] = marketRSIBands[market.Pair][1:]
		}
		bollingerBandsMutex.Unlock()
		calculateRSIBands(&market)
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
