package main

import (
	"backpocket/models"
	"backpocket/utils"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	chanRestartBinanceOHLCVMarketStream = make(chan bool, 10)
)

func binanceGetExistingMarkets(wg *sync.WaitGroup) {
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

		bodyBytes, err := io.ReadAll(httpResponse.Body)
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

		newBatchedMarkets := []models.Market{}
		for _, marketPair := range exchangeInfo.Symbols {

			market := getMarket(marketPair.Symbol, "binance")

			//this logic adds a new marketpair
			market.Pair = marketPair.Symbol

			market.Exchange = "binance"
			if market.ID == 0 {
				market.Status = "disabled"
			} else {
				switch marketPair.Symbol {
				case "BNBUSDT":
					market.Status = "enabled"
				}
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

			if market.ID == 0 {
				market.ID = models.TableID()
				market.Createdate = time.Now()
				market.Updatedate = time.Now()
				newBatchedMarkets = append(newBatchedMarkets, market)
			}
			updateMarket(market)
		}

		if len(newBatchedMarkets) > 0 {
			if err := utils.SqlDB.Transaction(func(tx *gorm.DB) error {
				if err := tx.CreateInBatches(newBatchedMarkets, 500).Error; err != nil {
					return err //Rollback
				}
				return nil
			}); err != nil {
				log.Println("Error Creating Batches: ", err.Error())
			}
		}

		if lFirstRun {
			wg.Done()
			lFirstRun = false
			log.Println("Markets First Run Completed")
		}

		time.Sleep(time.Hour * 6)
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

		bodyBytes, err := io.ReadAll(httpResponse.Body)
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

		updateBatchedMarkets := []models.Market{}
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

					if market.Close, err = strconv.ParseFloat(marketPair.LastPrice, 64); err != nil {
						log.Println(err.Error())
					}
					market.NumOfTrades = marketPair.Count
				}
				market.LastPrice = market.Price
				market.Price = market.Close
				updateMarket(market)
				wsBroadcastMarket <- market
				updateBatchedMarkets = append(updateBatchedMarkets, market)
			}
		}

		if len(updateBatchedMarkets) > 0 {
			values := make([]clause.Expr, 0, len(updateBatchedMarkets))
			for _, market := range updateBatchedMarkets {
				values = append(values, gorm.Expr("(?::bigint, ?::integer, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision, ?::double precision) ", market.ID, market.NumOfTrades, market.Open, market.Close, market.High, market.Low, market.Volume, market.VolumeQuote, market.LastPrice, market.Price, market.UpperBand, market.MiddleBand, market.LowerBand, market.PriceChange, market.PriceChangePercent, market.HighPrice, market.LowPrice))
			}

			batchedValues := make([]clause.Expr, 0, 500)
			for i, v := range values {
				batchedValues = append(batchedValues, v)
				if (i+1)%500 == 0 {
					batchedUpdateQueryMarkets(batchedValues)
					batchedValues = make([]clause.Expr, 0, 500)
				}
			}

			if len(batchedValues) > 0 {
				batchedUpdateQueryMarkets(batchedValues)
			}
		}

		time.Sleep(time.Minute * 5)
	}
}

func batchedUpdateQueryMarkets(batchedValues []clause.Expr) {
	valuesExpr := gorm.Expr("?", batchedValues)
	valuesExpr.WithoutParentheses = true

	if tx := utils.SqlDB.Exec(
		"UPDATE markets SET numoftrades = tmp.numoftrades, open = tmp.open, close = tmp.close, high = tmp.high, low = tmp.low, volume = tmp.volume, volumequote = tmp.volumequote, lastprice = tmp.lastprice, price = tmp.price, upperband = tmp.upperband, middleband = tmp.middleband, lowerband = tmp.lowerband, pricechange = tmp.pricechange, pricechangepercent = tmp.pricechangepercent, highprice = tmp.highprice, lowprice = tmp.lowprice, updatedate = NOW() FROM (VALUES ?) tmp(id, numoftrades, open, close, high, low, volume, volumequote, lastprice, price, upperband, middleband, lowerband, pricechange, pricechangepercent, highprice, lowprice) WHERE markets.id = tmp.id",
		valuesExpr,
	); tx.Error != nil {
		log.Printf("Error Creating Batches: %+v \n", tx.Error)
	}
	time.Sleep(time.Millisecond * 100)
}

func binanceMarketOHLCVStream() {

	var streamParams []string
	marketListMutex.RLock()
	for _, market := range marketList {
		// streamParams = append(streamParams, strings.ToLower(market.Pair)+"@bookTicker")
		if market.Status == "enabled" && market.Exchange == "binance" {
			streamParams = append(streamParams, strings.ToLower(market.Pair)+"@kline_"+DefaultTimeframe)
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
					streamParams = append(streamParams, strings.ToLower(market.Pair)+"@kline_"+DefaultTimeframe)
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
		calculateBollingerBands(&market)

		marketRSIPricesMutex.Lock()
		rsimapkey := fmt.Sprintf("%s-%s", market.Pair, market.Exchange)
		rsimaplenght := len(marketRSIPrices[rsimapkey])
		if rsimaplenght > 1 {
			marketRSIPrices[rsimapkey][rsimaplenght-1] = market.Price
		}
		marketRSIPricesMutex.Unlock()
		calculateRSIBands(&market)
		updateMarket(market)

		if wsResp.Data.Kline.Closed {
			if err := utils.SqlDB.Model(&market).Where("pair = ? and exchange = ?", market.Pair, market.Exchange).Updates(&market).Error; err != nil {
				log.Println(err.Error())
			}
		}

	}
	//loop through and read all messages received
}
