package main

import (
	"backpocket/utils"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	assetList    []assets
	assetListMap = make(map[string]int)

	assetListMutex    = sync.RWMutex{}
	assetListMapMutex = sync.RWMutex{}
	wsConnAssetsMutex = sync.RWMutex{}

	wsConnAssets     = make(map[*websocket.Conn]bool)
	wsBroadcastAsset = make(chan interface{}, 10240)
)

type assets struct {
	ID       uint64 `sql:"index"`
	Symbol   string `sql:"index"`
	Status   string `sql:"index"`
	Exchange string `sql:"index"`
	State    string `sql:"index"` //buy or sell
	Address  string `sql:"index"`
	Free     float64
	Locked   float64
}

func getAsset(symbol, assetExchange string) (asset assets) {
	assetKey := 0
	assetListMapMutex.RLock()
	for assetID, assetListIndex := range assetListMap {
		if assetID == fmt.Sprintf("%s-%s", symbol, strings.ToLower(assetExchange)) {
			assetKey = assetListIndex
		}
	}
	assetListMapMutex.RUnlock()

	assetListMutex.RLock()
	if assetKey > 0 && len(assetList) > (assetKey-1) {
		asset = assetList[assetKey-1]
		// copier.Copy(&asset, assetList[assetKey-1])
	}
	assetListMutex.RUnlock()
	return
}

func wsHandlerAssets(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		var filteredAssetList []assets
		assetListMutex.RLock()
		for _, asset := range assetList {
			// if asset.Free > 0 || asset.Locked > 0 {
			switch {
			case strings.HasSuffix(asset.Symbol, "UP"):
			case strings.HasSuffix(asset.Symbol, "DOWN"):
			case strings.HasSuffix(asset.Symbol, "BULL"):
			case strings.HasSuffix(asset.Symbol, "BEAR"):
			default:
				filteredAssetList = append(filteredAssetList, asset)
			}
			// }
		}
		assetListMutex.RUnlock()

		wsConn.WriteJSON(&wsResponseType{Action: "fetchassets", Result: filteredAssetList})

		wsConnAssetsMutex.Lock()
		wsConnAssets[wsConn] = true
		wsConnAssetsMutex.Unlock()

		for {
			var msgReq struct {
				Symbol, Exchange string
			}

			if err := wsConn.ReadJSON(&msgReq); err != nil {
				return
			}

			if msgReq.Symbol == "" {
				continue
			}

			//select from sqlitedb into array of orders
			var sqlParams []interface{}
			sqlSearch := "select * from assets where "

			sqlParams = append(sqlParams, "%"+msgReq.Symbol+"%")
			sqlSearch += fmt.Sprintf(" and symbol like $%v ", len(sqlParams))

			if msgReq.Exchange != "" {
				sqlParams = append(sqlParams, "%"+msgReq.Exchange+"%")
				sqlSearch += fmt.Sprintf(" and exchange like $%v ", len(sqlParams))
			}
			sqlSearch += " order by symbol, exchange"

			var resListAssets []assets
			if err := utils.SqlDB.Select(&resListAssets, sqlSearch, sqlParams...); err != nil {
				log.Println(sqlSearch, sqlParams)
				log.Println(err.Error())
			}

			wsConnAssetsMutex.Lock()
			if err := wsConn.WriteJSON(&wsResponseType{Action: "searchresult", Result: resListAssets}); err != nil {
				log.Println(err.Error())
				return
			}
			wsConnAssetsMutex.Unlock()
		}

	}
}
func wsHandlerAssetBroadcast() {
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			wsConnAssetsMutex.Lock()
			for wsConn := range wsConnAssets {
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					delete(wsConnAssets, wsConn)
					wsConn.Close()
				}
			}
			wsConnAssetsMutex.Unlock()
		}
	}()

	go func() {
		for asset := range wsBroadcastAsset {
			wsConnAssetsMutex.Lock()
			for wsConn := range wsConnAssets {
				if err := wsConn.WriteJSON(asset); err != nil {
					delete(wsConnAssets, wsConn)
					wsConn.Close()
				}
			}
			wsConnAssetsMutex.Unlock()
		}
	}()
}

func updateAsset(asset assets) {
	if asset.Symbol == "" {
		return
	}

	symbolKey := 0
	assetListMapMutex.RLock()

	for symbol, key := range assetListMap {
		if symbol == fmt.Sprintf("%s-%s", asset.Symbol, strings.ToLower(asset.Exchange)) {
			symbolKey = key
		}
	}

	assetListMapMutex.RUnlock()

	if symbolKey == 0 {

		assetListMutex.Lock()
		assetList = append(assetList, asset)
		symbolKey = len(assetList)
		assetListMutex.Unlock()

		assetListMapMutex.Lock()
		assetListMap[asset.Symbol] = symbolKey
		assetListMapMutex.Unlock()

	} else {
		assetListMutex.Lock()
		assetList[symbolKey-1] = asset
		assetListMutex.Unlock()
	}

	select {
	case wsBroadcastAsset <- &wsResponseType{Action: "balanceupdate", Result: []assets{asset}}:
	default:
	}

	updateFields := map[string]bool{"free": true, "locked": true}
	if sqlQuery, sqlParams := sqlTableUpdate(reflect.TypeOf(asset), reflect.ValueOf(asset), updateFields); len(sqlParams) > 0 {
		if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
			log.Println(sqlQuery)
			log.Println(err.Error())
		}

	}
}

func dbSetupAssets() {

	// tablename := ""
	// reflectType := reflect.TypeOf(assets{})
	// sqlTable := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	// if utils.SqlDB.Get(&tablename, sqlTable, "assets"); tablename == "" {
	// 	if !sqlTableCreate(reflectType) { //create table if it is missing
	// 		log.Panicf("Table creation failed for table [%s] \n", reflectType.Name())
	// 	}
	// }

	assetListMutex.Lock()
	// assetList = nil
	err := utils.SqlDB.Select(&assetList, "select * from assets order by symbol, exchange")
	if err != nil {
		log.Println(err.Error())
	}
	assetListMutex.Unlock()

	assetListMapMutex.Lock()
	assetListMap = make(map[string]int)
	for symbolKey, asset := range assetList {
		assetListMap[fmt.Sprintf("%s-%s", asset.Symbol, strings.ToLower(asset.Exchange))] = symbolKey + 1
	}
	assetListMapMutex.Unlock()

}
