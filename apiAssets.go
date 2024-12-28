package main

import (
	"backpocket/models"
	"backpocket/utils"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	assetList    []models.Asset
	assetListMap = make(map[string]int)

	assetListMutex    = sync.RWMutex{}
	assetListMapMutex = sync.RWMutex{}
	wsConnAssetsMutex = sync.RWMutex{}

	wsConnAssets     = make(map[*websocket.Conn]bool)
	wsBroadcastAsset = make(chan *wsResponseType, 10240)
)

func getAsset(symbol, assetExchange string) (asset models.Asset) {
	assetKey := 0
	assetListMapMutex.RLock()
	for assetID, assetListIndex := range assetListMap {
		if assetID == fmt.Sprintf("%s-%s", symbol, strings.ToLower(assetExchange)) {
			assetKey = assetListIndex
			break
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

		var filteredAssetList []models.Asset
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

			var searchText string
			var searchParams []interface{}

			searchText = "symbol = ?"
			searchParams = append(searchParams, msgReq.Symbol)

			if msgReq.Exchange != "" {
				searchText += " AND exchange = ?"
				searchParams = append(searchParams, msgReq.Exchange)
			}
			orderby := "symbol, exchange"

			var foundAssets []models.Asset
			if err := utils.SqlDB.Where(searchText, searchParams...).Order(orderby).Find(&foundAssets).Error; err != nil {
				log.Println(err.Error())
			}

			wsConnAssetsMutex.Lock()
			if err := wsConn.WriteJSON(&wsResponseType{Action: "searchresult", Result: foundAssets}); err != nil {
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
			if asset.Result == nil {
				continue
			}
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

func updateAsset(asset models.Asset) {
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
		assetListMap[fmt.Sprintf("%s-%s", asset.Symbol, strings.ToLower(asset.Exchange))] = symbolKey
		assetListMapMutex.Unlock()

	} else {
		assetListMutex.Lock()
		assetList[symbolKey-1] = asset
		assetListMutex.Unlock()
	}

	select {
	case wsBroadcastAsset <- &wsResponseType{Action: "balanceupdate", Result: []models.Asset{asset}}:
	default:
	}
}

func saveAsset(asset models.Asset) {
	if err := utils.SqlDB.Model(&asset).Where("symbol = ? and exchange = ?", asset.Symbol, asset.Exchange).Updates(map[string]interface{}{"free": asset.Free, "locked": asset.Locked, "status": asset.Status}).Error; err != nil {
		log.Println(err.Error())
	}
}

func LoadAssetsFromDB() {
	var assets []models.Asset
	if err := utils.SqlDB.Find(&assets).Order("symbol, exchange").Error; err != nil {
		log.Println(err.Error())
		return
	}

	for _, asset := range assets {
		updateAsset(asset)
	}
}
