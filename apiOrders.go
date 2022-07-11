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
	orderList    []orders
	orderListMap = make(map[string]int)

	orderListMutex    = sync.RWMutex{}
	ordersTableMutex  = sync.RWMutex{}
	orderListMapMutex = sync.RWMutex{}
	wsConnOrdersMutex = sync.RWMutex{}

	wsConnOrders     = make(map[*websocket.Conn]bool)
	wsBroadcastOrder = make(chan interface{}, 10240)
)

type orders struct {
	ID       uint64 `sql:"index"`
	Pair     string `sql:"index"`
	Status   string `sql:"index"` // (pending|buy|sell|done)
	Exchange string `sql:"index"`

	Side string `sql:"index"`
	Type string `sql:"index"`

	OrderID uint64 `sql:"index"`

	RefSide      string `sql:"index"`
	AutoRepeat   int    `sql:"index"`
	RefTripped   string `sql:"index"`
	AutoRepeatID uint64 `sql:"index"`
	RefOrderID   uint64 `sql:"index"`
	RefEnabled   int    `sql:"index"`

	Price, Quantity,
	Total, Stoploss,
	Takeprofit float64

	Created time.Time `sql:"index"`
	Updated time.Time `sql:"index"`
}

type orderMsgType struct {
	Action, Start,
	Stop string
	Order orders
}

func getOrder(orderID uint64, orderExchange string) (order orders) {
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
				updateOrder(msg.Order)

			case "refdisable":
				msg.Order.RefEnabled = 0
				updateOrder(msg.Order)

			case "list":
				var searchMsg = searchOrderMsgType{
					Stop:  msg.Stop,
					Start: msg.Start,
					Pair:  msg.Order.Pair,
				}
				filteredOrderList := searchOrderSQL(searchMsg)
				select {
				case wsBroadcastOrder <- filteredOrderList:
				default:
				}

				go func() {
					switch msg.Order.Exchange {
					default:
						binanceAllOrders(msg.Order.Pair)
					case "crex24":
						crex24AllOrders(msg.Order.Pair)
					}
				}()

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
					binanceOrderCreate(msg.Order.Pair, msg.Order.Side, fmt.Sprintf("%.8f", msg.Order.Price), msg.Order.Quantity, msg.Order.Stoploss, msg.Order.Takeprofit, msg.Order.AutoRepeat, 0)
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

func updateOrder(order orders) {
	if updateOrderOnly(order) {
		select {
		case wsBroadcastOrder <- []orders{order}:
		default:
		}
	}
}

func updateOrderOnly(order orders) bool {
	if order.OrderID == 0 {
		return false
	}

	orderKey := 0
	orderListMapMutex.RLock()
	for orderID, orderListIndex := range orderListMap {
		if orderID == fmt.Sprintf("%v-%s", order.OrderID, strings.ToLower(order.Exchange)) {
			orderKey = orderListIndex
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

	updateFields := map[string]bool{
		"id": true, "pair": true, "status": true, "exchange": true, "side": true, "type": true, "orderid": true,
		"refside": true, "autorepeat": true, "reftripped": true, "autorepeatid": true, "reforderid": true, "refenabled": true,
		"price": true, "quantity": true, "total": true, "stoploss": true, "takeprofit": true, "created": true, "updated": true,
	}

	go func() {
		ordersTableMutex.Lock()
		if sqlQuery, sqlParams := sqlTableUpdate(reflect.TypeOf(order), reflect.ValueOf(order), updateFields); len(sqlParams) > 0 {
			if _, err := utils.SqlDB.Exec(sqlQuery, sqlParams...); err != nil {
				log.Println(sqlQuery)
				log.Println(sqlParams)
				log.Println(err.Error())
			}

		}
		ordersTableMutex.Unlock()
	}()

	return true
}

func dbSetupOrders() {

	// tablename := ""
	// reflectType := reflect.TypeOf(orders{})
	// sqlTable := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	// if utils.SqlDB.Get(&tablename, sqlTable, "orders"); tablename == "" {
	// 	if !sqlTableCreate(reflectType) { //create table if it is missing
	// 		log.Panicf("Table creation failed for table [%s] \n", reflectType.Name())
	// 	}
	// }

	orderListMutex.Lock()
	// orderList = nil
	utils.SqlDB.Select(&orderList, "select * from orders order by pair, exchange, orderid desc")
	orderListMutex.Unlock()

	orderListMapMutex.Lock()
	orderListMap = make(map[string]int)
	for pairKey, orderPair := range orderList {
		orderListMap[fmt.Sprintf("%v-%s", orderPair.OrderID, strings.ToLower(orderPair.Exchange))] = pairKey + 1
	}
	orderListMapMutex.Unlock()
}
