package main

import (
	"backpocket/models"
	"backpocket/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

		newBatchedAssets := []models.Asset{}
		updateBatchedAssets := []models.Asset{}
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

			if asset.Free > 0 || asset.Locked > 0 {
				asset.Status = "enabled"
			} else {
				asset.Status = "disabled"
			}

			if !(asset.ID > 0) {
				//this logic adds a new asset
				asset.ID = models.TableID()
				asset.Symbol = bal.Asset
				asset.State = ""
				asset.Exchange = "binance"

				newBatchedAssets = append(newBatchedAssets, asset)
			} else {
				updateBatchedAssets = append(updateBatchedAssets, asset)
			}

			updateAsset(asset)
		}

		if len(newBatchedAssets) > 0 {
			if err := utils.SqlDB.Transaction(func(tx *gorm.DB) error {
				if err := tx.CreateInBatches(newBatchedAssets, 500).Error; err != nil {
					return err //Rollback
				}
				return nil
			}); err != nil {
				log.Println("Error Creating Batches: ", err.Error())
			}
		}

		if len(updateBatchedAssets) > 0 {
			values := make([]clause.Expr, 0, len(updateBatchedAssets))
			for _, asset := range updateBatchedAssets {
				values = append(values, gorm.Expr("(?::bigint, ?, ?::double precision, ?::double precision) ", asset.ID, asset.Status, asset.Free, asset.Locked))
			}

			batchedValues := make([]clause.Expr, 0, 250)
			for i, v := range values {
				batchedValues = append(batchedValues, v)
				if (i+1)%250 == 0 {
					batchedUpdateQueryAssets(batchedValues)
					batchedValues = make([]clause.Expr, 0, 250)
				}
			}

			if len(batchedValues) > 0 {
				batchedUpdateQueryAssets(batchedValues)
			}
		}
		time.Sleep(time.Minute * 15)
	}
}

func batchedUpdateQueryAssets(batchedValues []clause.Expr) {
	valuesExpr := gorm.Expr("?", batchedValues)
	valuesExpr.WithoutParentheses = true

	if tx := utils.SqlDB.Exec(
		"UPDATE assets SET status = tmp.status, free = tmp.free, locked = tmp.locked, updatedate = NOW() FROM (VALUES ?) tmp(id,status,free,locked) WHERE assets.id = tmp.id",
		valuesExpr,
	); tx.Error != nil {
		log.Printf("Error Creating Batches: %+v \n", tx.Error)
	}
	time.Sleep(time.Millisecond * 100)
}

func binanceAssetStreamKeepAliveListenKey(listenKey string) {
	client := &http.Client{}
	ticker := time.NewTicker(20 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		err := binanceAssetStreamSendKeepAlive(client, listenKey)
		if err != nil {
			log.Printf("Failed to send keep-alive for listenKey: %v \n", err)
		} else {
			log.Println("Successfully sent keep-alive for listenKey")
		}
	}
}

// sendKeepAlive sends a PUT request to keep the listenKey alive
func binanceAssetStreamSendKeepAlive(client *http.Client, listenKey string) error {
	url := binanceRestURL + "/userDataStream?listenKey=" + listenKey
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer([]byte{}))
	if err != nil {
		return err
	}

	req.Header.Set("X-MBX-APIKEY", binanceAPIKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to keep listenKey alive: %s", resp.Status)
	}
	return nil
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

	//make an http get call to binanceRestURL every 30 minutes to keep the stream alive
	go binanceAssetStreamKeepAliveListenKey(respStruct.ListenKey)

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
			time.Sleep(time.Second * 10)

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
				asset.Symbol = wRespBalance.Data.Asset
				asset.State = ""
				asset.Status = "enabled"
				asset.Exchange = "binance"

				if err := utils.SqlDB.Model(&asset).Create(&asset).Error; err != nil {
					log.Println(err.Error())
				}
			}
			// log.Printf("balanceUpdate asset: %+v \n", asset)
			updateAsset(asset)
			go saveAsset(asset)
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
					asset.Symbol = bal.Asset
					asset.State = ""
					asset.Status = "enabled"
					asset.Exchange = "binance"

					if err := utils.SqlDB.Model(&asset).Create(&asset).Error; err != nil {
						log.Println(err.Error())
					}
				}
				// log.Printf("outboundAccountPosition asset: %+v \n", asset)
				updateAsset(asset)
				go saveAsset(asset)
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
			if !(order.ID > 0) {
				order.Side = wRespOrderupdate.Data.Side
				order.Pair = wRespOrderupdate.Data.Symbol
				order.OrderID = wRespOrderupdate.Data.OrderID
				order.Status = wRespOrderupdate.Data.CurrentOrderStatus
				order.Createdate = time.Unix(wRespOrderupdate.Data.CreationTime/1000, 0)

				order.Price, _ = strconv.ParseFloat(wRespOrderupdate.Data.OrderPrice, 64)
				order.Quantity, _ = strconv.ParseFloat(wRespOrderupdate.Data.OrderQuantity, 64)
				order.Total = utils.TruncateFloat(order.Price*order.Quantity, 8)

				if err := utils.SqlDB.Model(&order).Create(&order).Error; err != nil {
					log.Println(err.Error())
				}
			} else {
				order.Status = wRespOrderupdate.Data.CurrentOrderStatus
				order.Price, _ = strconv.ParseFloat(wRespOrderupdate.Data.LastExecutedPrice, 64)
				executedQty, _ := strconv.ParseFloat(wRespOrderupdate.Data.CummulativeFilledQty, 64)
				cummulativeQuoteQty, _ := strconv.ParseFloat(wRespOrderupdate.Data.CummulativeQuoteTransactedQty, 64)

				order.Quantity, _ = strconv.ParseFloat(wRespOrderupdate.Data.OrderQuantity, 64)
				order.Total, _ = strconv.ParseFloat(wRespOrderupdate.Data.QuoteOrderQty, 64)
				if executedQty > 0 && order.Status == "CANCELED" {
					order.Status = "FILLED"
				}

				if order.Status != "CANCELED" && executedQty > 0 && cummulativeQuoteQty > 0 {
					order.Quantity = executedQty
					order.Total = cummulativeQuoteQty
				}

				order.Updatedate = time.Unix(wRespOrderupdate.Data.CreationTime/1000, 0)
				if order.Createdate.IsZero() {
					order.Createdate = order.Updatedate
				}
			}
			updateOrderAndSave(order, true)

			wsBroadcastNotification <- notifications{
				Title:   "*Binance Exchange*",
				Message: fmt.Sprintf("%s limit %s order [%v] for %s %s", order.Status, order.Side, order.OrderID, strconv.FormatFloat(order.Quantity, 'f', -1, 64), order.Pair),
			}

		default:
			log.Printf("Unknown Event: %v - %+v", wsResp.Data.Event, wsResp)
		}
	}

	//loop through and read all messages received
}
