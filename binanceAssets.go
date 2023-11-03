package main

import (
	"backpocket/utils"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	chanRestartBinanceAssetStream = make(chan bool, 10)
)

func binanceAssetGetBalance(asset string) (free float64) {
	assetListMutex.RLock()
	for _, qAsset := range assetList {
		if strings.ToUpper(asset) == strings.ToUpper(qAsset.Symbol) {
			free = qAsset.Free
			return
		}
	}
	assetListMutex.RUnlock()
	//s
	return
}

func binanceAssetGet() {
	for {
		respBytes := binanceRestAPI("GET", binanceRestURL+"/account?", "")

		var assetBalances struct {
			CanTrade, CanDeposit,
			CanWithdraw bool
			Balances []struct {
				Asset, Free, Locked string
			}
		}
		// assetList = []assets{}
		json.Unmarshal(respBytes, &assetBalances)
		for _, bal := range assetBalances.Balances {

			if strings.HasSuffix(bal.Asset, "UP") || strings.HasSuffix(bal.Asset, "DOWN") ||
				strings.HasSuffix(bal.Asset, "BULL") || strings.HasSuffix(bal.Asset, "BEAR") {
				continue
			}

			asset := getAsset(strings.ToUpper(bal.Asset), "binance")
			asset.Free, _ = strconv.ParseFloat(bal.Free, 64)
			asset.Locked, _ = strconv.ParseFloat(bal.Locked, 64)
			//findkey and create if it does not exist

			if !(asset.ID > 0) {
				//this logic adds a new asset
				asset.ID = sqlTableID()
				asset.Symbol = bal.Asset
				asset.State = ""
				asset.Status = "disabled"
				asset.Exchange = "binance"

				if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(asset), reflect.ValueOf(asset)); len(sqlParams) > 0 {
					if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
						log.Println(err.Error())
					}
				}
			}

			updateAsset(asset)
		}

		time.Sleep(time.Minute * 15)
	}
}

func binanceAssetStream() {

	var streamParams []string
	var respStruct struct {
		ListenKey string `json:"listenKey"`
	}

	httpClient := http.Client{Timeout: time.Duration(time.Second * 30)}
	httpRequest, _ := http.NewRequest("POST", binanceRestURL+"/userDataStream", nil)
	httpRequest.Header.Set("X-MBX-APIKEY", binanceAPIKey)

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		log.Panicf(err.Error())
		return
	}

	respBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		log.Panicf(err.Error())
		return
	}
	httpResponse.Body.Close()

	json.Unmarshal(respBytes, &respStruct)
	streamParams = append(streamParams, respStruct.ListenKey)

	if respStruct.ListenKey == "" {
		log.Panicf("Could not retrieve Listenkey, error: %s", respBytes)
		return
	}

	bwConn := binanceWSConnect(streamParams)
	if _, _, err := bwConn.ReadMessage(); err != nil {
		log.Println("err ", err.Error())
	}

	//loop through and read all messages received
	for {
		select {
		case <-chanRestartBinanceAssetStream:
			bwConn.Close()
			bwConn = binanceWSConnect(streamParams)
			if _, _, err := bwConn.ReadMessage(); err != nil {
				log.Println("err ", err.Error())
			}
		default:
		}

		wsResp := binanceStreamAssetResp{}
		_, wsRespBytes, _ := bwConn.ReadMessage()
		if err := json.Unmarshal(wsRespBytes, &wsResp); err != nil {
			log.Println("binanceAssetStream bwCon read error:", err)
			log.Println("wsRespBytes:", string(wsRespBytes))
			time.Sleep(time.Second * 15)

			select {
			case chanRestartBinanceAssetStream <- true:
			default:
			}
			continue
		}

		// log.Println(string(wsRespBytes))
		switch wsResp.Data.Event {
		case "balanceUpdate":
			wRespBalance := binanceBalanceUpdate{}
			// log.Println(string(wsRespBytes))

			if err := json.Unmarshal(wsRespBytes, &wRespBalance); err != nil {
				log.Println("read:", err)
				log.Println("wsRespBytes:", string(wsRespBytes))
				continue
			}
			//find symbol

			asset := getAsset(strings.ToUpper(wRespBalance.Data.Asset), "binance")
			asset.Free, _ = strconv.ParseFloat(wRespBalance.Data.BalanceDelta, 64)

			if !(asset.ID > 0) {
				//this logic adds a new asset
				asset.ID = sqlTableID()
				asset.Symbol = wRespBalance.Data.Asset
				asset.State = ""
				asset.Status = "enabled"
				asset.Exchange = "binance"

				if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(asset), reflect.ValueOf(asset)); len(sqlParams) > 0 {
					if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
						log.Println(err.Error())
					}
				}
			}
			// log.Printf("balanceUpdate asset: %+v \n", asset)
			updateAsset(asset)
			// select {
			// case wsBroadcastAsset <- &wsResponseType{Action: "balanceupdate", Result: []assets{asset}}:
			// default:
			// }

		case "outboundAccountPosition":
			wRespOutboundPosition := binanceOutboundAccountPosition{}
			// log.Println(string(wsRespBytes))

			if err := json.Unmarshal(wsRespBytes, &wRespOutboundPosition); err != nil {
				log.Println("read:", err)
				log.Println("wsRespBytes:", string(wsRespBytes))
				continue
			}

			for _, bal := range wRespOutboundPosition.Data.Balances {

				asset := getAsset(strings.ToUpper(bal.Asset), "binance")
				asset.Free, _ = strconv.ParseFloat(bal.Free, 64)
				asset.Locked, _ = strconv.ParseFloat(bal.Locked, 64)
				//findkey and create if it does not exist

				if !(asset.ID > 0) {
					//this logic adds a new asset
					asset.ID = sqlTableID()
					asset.Symbol = bal.Asset
					asset.State = ""
					asset.Status = "enabled"
					asset.Exchange = "binance"

					if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(asset), reflect.ValueOf(asset)); len(sqlParams) > 0 {
						if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
							log.Println(err.Error())
						}
					}
				}
				// log.Printf("outboundAccountPosition asset: %+v \n", asset)
				updateAsset(asset)
				// select {
				// case wsBroadcastAsset <- &wsResponseType{Action: "outboundaccountposition", Result: []assets{asset}}:
				// default:
				// }

			}

		case "executionReport":
			wRespOrderupdate := binanceExecutionReport{}
			if err := json.Unmarshal(wsRespBytes, &wRespOrderupdate); err != nil {
				log.Println("read:", err)
				log.Println("wsRespBytes:", string(wsRespBytes))
				continue
			}

			//find order
			order := getOrder(wRespOrderupdate.Data.OrderID, "binance")

			order.Exchange = "binance"
			if !(order.OrderID > 0) {
				order.Side = wRespOrderupdate.Data.Side
				order.Pair = wRespOrderupdate.Data.Symbol
				order.OrderID = wRespOrderupdate.Data.OrderID
				order.Status = wRespOrderupdate.Data.CurrentOrderStatus
				order.Created = time.Unix(wRespOrderupdate.Data.CreationTime/1000, 0)

				order.Price, _ = strconv.ParseFloat(wRespOrderupdate.Data.OrderPrice, 64)
				order.Quantity, _ = strconv.ParseFloat(wRespOrderupdate.Data.OrderQuantity, 64)
				order.Total = order.Price * order.Quantity

				order.ID = sqlTableID()

				ordersTableMutex.Lock()
				if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(order), reflect.ValueOf(order)); len(sqlParams) > 0 {
					if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
						log.Println(err.Error())
					}
				}
				ordersTableMutex.Unlock()
			} else {
				order.Status = wRespOrderupdate.Data.CurrentOrderStatus
				order.Updated = time.Unix(wRespOrderupdate.Data.CreationTime/1000, 0)
				if order.Created.IsZero() {
					order.Created = order.Updated
				}
			}
			updateOrder(order)

			wsBroadcastNotification <- notifications{
				Title:   "*Binance Exchange*",
				Message: fmt.Sprintf("%s limit %s order [%v] for %v %s", order.Status, order.Side, order.OrderID, order.Quantity, order.Pair),
			}
		}
	}

	//loop through and read all messages received
}
