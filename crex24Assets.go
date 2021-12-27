package main

import (
	"backpocket/utils"
	"encoding/json"
	"log"
	"reflect"
	"strings"
	"time"
)

var (
	chanRestartCrex24AssetStream = make(chan bool, 10)
)

func crex24AssetGetBalance(asset string) (free float64) {
	// assetListMutex.RLock()
	for _, qAsset := range assetList {
		if strings.ToUpper(asset) == strings.ToUpper(qAsset.Symbol) {
			free = qAsset.Free
			return
		}
	}
	// assetListMutex.RUnlock()
	return
}

func crex24AssetGet() {
	crex24Keys()
	for {
		respBytes := crex24RestAPI("GET", "/v2/account/balance?nonZeroOnly=false", nil)

		var assetBalances []struct {
			Currency            string
			Available, Reserved float64
		}

		// assetList = []assets{}
		json.Unmarshal(respBytes, &assetBalances)
		for _, assetBal := range assetBalances {

			asset := getAsset(strings.ToUpper(assetBal.Currency), "crex24")
			asset.Free = assetBal.Available
			asset.Locked = assetBal.Reserved
			//findkey and create if it does not exist

			if !(asset.ID > 0) {
				//this logic adds a new asset
				asset.ID = sqlTableID()
				asset.Symbol = assetBal.Currency
				asset.State = ""
				asset.Status = "disabled"
				asset.Exchange = "crex24"

				if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(asset), reflect.ValueOf(asset)); len(sqlParams) > 0 {
					if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
						log.Println(err.Error())
					}
				}

			}

			updateAsset(asset)
			//send asset down websocket
		}
		time.Sleep(time.Minute * 5)
	}
}

func crex24AssetCryptoDepositAddress(currency string) {
	crex24Keys()
	respBytes := crex24RestAPI("GET", "/v2/account/depositAddress?currency="+currency, nil)

	var depositAddressResponse struct {
		Currency, Address,
		PaymentID string
	}

	// assetList = []assets{}
	asset := getAsset(currency, "crex24")

	if asset.ID > 0 {
		json.Unmarshal(respBytes, &depositAddressResponse)
		asset.Address = depositAddressResponse.Address
		updateAsset(asset)
	}
}

func crex24AssetStream() {
	//

	// crex24Keys()
	// var streamParams []string
	// var respStruct struct {
	// 	ListenKey string `json:"listenKey"`
	// }

	// httpClient := http.Client{Timeout: time.Duration(time.Second * 10)}
	// httpRequest, _ := http.NewRequest("POST", crex24RestURL+"/userDataStream", nil)
	// httpRequest.Header.Set("X-MBX-APIKEY", crex24APIKey)

	// httpResponse, err := httpClient.Do(httpRequest)
	// if err != nil {
	// 	log.Panicf(err.Error())
	// 	return
	// }

	// respBytes, err := ioutil.ReadAll(httpResponse.Body)
	// if err != nil {
	// 	log.Panicf(err.Error())
	// 	return
	// }
	// httpResponse.Body.Close()

	// json.Unmarshal(respBytes, &respStruct)
	// streamParams = append(streamParams, respStruct.ListenKey)

	// if respStruct.ListenKey == "" {
	// 	log.Panicf("Could not retrieve Listenkey, error: %s", respBytes)
	// 	return
	// }

	// bwConn := crex24WSConnect()
	// if _, _, err := bwConn.ReadMessage(); err != nil {
	// 	log.Println("err ", err.Error())
	// }
	/*
		//loop through and read all messages received
		for {
			wsResp := crex24StreamAssetResp{}
			_, wsRespBytes, _ := bwConn.ReadMessage()
			if err := json.Unmarshal(wsRespBytes, &wsResp); err != nil {
				log.Println("readBytes:", string(wsRespBytes))
				log.Println("read:", err)
				continue
			}

			select {
			case <-chanRestartCrex24AssetStream:
				//bwConn.Close()
				bwConn = crex24WSConnect(streamParams)
				if _, _, err := bwConn.ReadMessage(); err != nil {
					log.Println("err ", err.Error())
				}

			default:
			}

			switch wsResp.Data.Event {
			case "balanceUpdate":
				wRespBalance := crex24BalanceUpdate{}

				if err := json.Unmarshal(wsRespBytes, &wRespBalance); err != nil {
					log.Println("read:", err)
					log.Println("wsRespBytes:", string(wsRespBytes))
					return
				}
				//find symbol
				asset := getAsset(strings.ToUpper(wRespBalance.Data.Asset), "crex24")
				asset.Free, _ = strconv.ParseFloat(wRespBalance.Data.BalanceDelta, 64)

				if !(asset.ID > 0) {
					//this logic adds a new asset
					asset.ID = sqlTableID()
					asset.Symbol = wRespBalance.Data.Asset
					asset.State = ""
					asset.Status = "enabled"
					asset.Exchange = "crex24"

					if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(asset), reflect.ValueOf(asset)); len(sqlParams) > 0 {
						if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
							log.Println(err.Error())
						}
					}

				}

				select {
				case wsBroadcastAsset <- &wsResponseType{Action: "balanceupdate", Result: []assets{asset}}:
				default:
				}

				updateAsset(asset)



			case "outboundAccountPosition":
				wRespOutboundPosition := crex24OutboundAccountPosition{}
				if err := json.Unmarshal(wsRespBytes, &wRespOutboundPosition); err != nil {
					log.Println("read:", err)
					log.Println("wsRespBytes:", string(wsRespBytes))
					return
				}

				for _, bal := range wRespOutboundPosition.Data.Balances {

					//find symbol
					asset := getAsset(strings.ToUpper(bal.Asset), "crex24")
					asset.Free, _ = strconv.ParseFloat(bal.Free, 64)
					asset.Locked, _ = strconv.ParseFloat(bal.Locked, 64)
					//findkey and create if it does not exist

					if !(asset.ID > 0) {
						//this logic adds a new asset
						asset.ID = sqlTableID()
						asset.Symbol = bal.Asset
						asset.State = ""
						asset.Status = "enabled"
						asset.Exchange = "crex24"

						if sqlQuery, sqlParams := sqlTableInsert(reflect.TypeOf(asset), reflect.ValueOf(asset)); len(sqlParams) > 0 {
							if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
								log.Println(err.Error())
							}
						}
					}

					select {
					case wsBroadcastAsset <- &wsResponseType{Action: "outboundaccountposition", Result: []assets{asset}}:
					default:
					}

					updateAsset(asset)
				}

			}

		}
	*/
	//loop through and read all messages received
}
