package main

import (
	"backpocket/models"
	"backpocket/utils"
	"encoding/json"

	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	orderList    []models.Order
	orderListMap = make(map[string]int)

	orderListMutex    = sync.RWMutex{}
	orderListMapMutex = sync.RWMutex{}
	wsConnOrdersMutex = sync.RWMutex{}

	wsConnOrders     = make(map[*websocket.Conn]bool)
	wsBroadcastOrder = make(chan []models.Order, 10240)
)

type orderMsgType struct {
	Action, Start,
	Stop string
	Order models.Order
}

func getOrder(orderID uint64, orderExchange string) (order models.Order) {
	orderKey := 0
	orderListMapMutex.RLock()
	for mapID, orderListIndex := range orderListMap {
		if mapID == fmt.Sprintf("%v-%s", orderID, strings.ToLower(orderExchange)) {
			orderKey = orderListIndex
		}
	}
	orderListMapMutex.RUnlock()

	orderListMutex.RLock()
	if orderKey > 0 && len(orderList) > (orderKey-1) {
		order = orderList[orderKey-1]
		// copier.Copy(&order, orderList[orderKey-1])
	}
	orderListMutex.RUnlock()
	return
}

func wsHandlerOrders(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		wsConnOrdersMutex.Lock()
		wsConnOrders[wsConn] = true
		wsConnOrdersMutex.Unlock()

		for {
			var msg = orderMsgType{}
			if err := wsConn.ReadJSON(&msg); err != nil {
				log.Println("wsConn.ReadJSON: ", err)
				return
			}

			if reflect.DeepEqual(msg, orderMsgType{}) {
				continue
			}

			switch msg.Action {
			case "refenable":
				msg.Order.RefEnabled = 1
				updateOrderAndSave(msg.Order, true)

			case "refdisable":
				msg.Order.RefEnabled = 0
				updateOrderAndSave(msg.Order, true)

			case "list":
				var searchMsg = searchOrderMsgType{
					Pair:  msg.Order.Pair,
					Stop:  msg.Stop,
					Start: msg.Start,
				}
				filteredOrderList := searchOrderSQL(searchMsg)
				select {
				case wsBroadcastOrder <- filteredOrderList:
				default:
				}

				go func() { //not needed as this causes race errors and data upate issues
					switch msg.Order.Exchange {
					default:
						//convert msg.Start to a valid time.Time
						startTime, err := time.Parse("2006-01-02 15:04:05", msg.Start)
						if startTime.IsZero() || err != nil {
							startTime = time.Now().AddDate(0, -3, 0) //go back 3 months by default
						}
						startTimeStamp := startTime.UnixNano() / int64(time.Millisecond)
						binanceAllOrders(msg.Order.Pair, startTimeStamp)
					case "crex24":
						crex24AllOrders(msg.Order.Pair)
					}
				}() //not needed as this causes race errors and data upate issues

			case "query":
				switch msg.Order.Exchange {
				default:
					binanceOrderQuery(msg.Order.Pair, msg.Order.OrderID)
				case "crex24":
					crex24OrderQuery(msg.Order.OrderID)
				}

			case "cancel":
				switch msg.Order.Exchange {
				default:
					binanceOrderCancel(msg.Order.Pair, msg.Order.OrderID)
				case "crex24":
					crex24OrderCancel(msg.Order.OrderID)
				}

			case "create":

				// TakeProfit, StopLoss

				//Get particular Market.
				switch msg.Order.Exchange {
				default:
					binanceOrderCreate(msg.Order.Pair, msg.Order.Side, strconv.FormatFloat(msg.Order.Price, 'f', -1, 64), strconv.FormatFloat(msg.Order.Quantity, 'f', -1, 64), msg.Order.Stoploss, msg.Order.Takeprofit, msg.Order.AutoRepeat, 0)
				case "crex24":
					crex24OrderCreate(msg.Order.Pair, msg.Order.Side, msg.Order.Price, msg.Order.Quantity, msg.Order.Stoploss, msg.Order.Takeprofit, msg.Order.AutoRepeat, 0)
				}

			}
		}
	}
}

func wsHandlerOrderBroadcast() {
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			wsConnOrdersMutex.Lock()
			for wsConn := range wsConnOrders {
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					delete(wsConnOrders, wsConn)
					wsConn.Close()
				}
			}
			wsConnOrdersMutex.Unlock()
		}
	}()

	go func() {
		for order := range wsBroadcastOrder {
			if len(order) == 0 {
				continue
			}
			wsConnOrdersMutex.Lock()
			for wsConn := range wsConnOrders {
				if err := wsConn.WriteJSON(order); err != nil {
					delete(wsConnOrders, wsConn)
					wsConn.Close()
				}
			}
			wsConnOrdersMutex.Unlock()
		}
	}()
}

func updateOrderAndSave(order models.Order, save bool) {
	if order.OrderID == 0 {
		return
	}

	orderKey := 0
	orderListMapMutex.RLock()
	for orderID, key := range orderListMap {
		if orderID == fmt.Sprintf("%v-%s", order.OrderID, strings.ToLower(order.Exchange)) {
			orderKey = key
		}
	}
	orderListMapMutex.RUnlock()

	if orderKey == 0 {
		orderListMutex.Lock()
		orderList = append(orderList, order)
		orderKey = len(orderList)
		orderListMutex.Unlock()

		orderListMapMutex.Lock()
		orderListMap[fmt.Sprintf("%v-%s", order.OrderID, strings.ToLower(order.Exchange))] = orderKey
		orderListMapMutex.Unlock()
	} else {
		orderListMutex.Lock()
		orderList[orderKey-1] = order
		orderListMutex.Unlock()
	}

	if save {
		go saveOrder(order)

		select {
		case wsBroadcastOrder <- []models.Order{order}:
		default:
		}
	}

}

func saveOrder(order models.Order) {

	//convert order to a map interface using json
	orderMap := make(map[string]interface{})
	orderJSON, err := json.Marshal(order)
	if err != nil {
		log.Println("Error marshalling order to JSON:", err)
		return
	}
	if err := json.Unmarshal(orderJSON, &orderMap); err != nil {
		log.Println("Error unmarshalling JSON to map:", err)
		return
	}

	if err := utils.SqlDB.Model(&order).Where("pair = ? and exchange = ? and orderid = ?", order.Pair, order.Exchange, order.OrderID).Updates(orderMap).Error; err != nil {
		log.Println(err.Error())
	}

	// sql := utils.SqlDB.ToSQL(func(tx *gorm.DB) *gorm.DB {
	// 	return tx.Model(&order).Where("pair = ? and exchange = ? and orderid = ?", order.Pair, order.Exchange, order.OrderID).Updates(orderMap)
	// })
	// log.Println("Query: ", sql)
}

func LoadOrdersFromDB() {
	var orders []models.Order
	if err := utils.SqlDB.Find(&orders).Error; err != nil {
		log.Println(err.Error())
		return
	}

	for _, order := range orders {
		updateOrderAndSave(order, false)
	}
}
