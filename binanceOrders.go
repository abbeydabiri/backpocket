package main

import (
	"backpocket/utils"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"time"
)

/*
	Binance Order Status:
		NEW, FILLED, CANCELLED, PARTIALLY_FILLED
*/

var (
	chanRestartBinanceOrderStream = make(chan bool, 10)
)

type binanceOrderType struct {
	OrderID, RefOrderID  int
	Takeprofit, Stoploss float64

	Time, TransactTime int64
	Price, OrigQty, ExecutedQty,
	Symbol, Status, Side string
}

// func binanceOrderBookStream() {

// for {
//loop through enabled markets.

//get the binanceOrders linked to the order

// time.After(time.Minute * 15)
// }
// }

func binanceAllOrders(pair string) {
	queryParams := fmt.Sprintf(binanceListOrdersParams, pair)
	respBytes := binanceRestAPI("GET", binanceRestURL+"/allOrders?", queryParams)

	var binanceOrderList []binanceOrderType
	json.Unmarshal(respBytes, &binanceOrderList)
	for _, binanceOrder := range binanceOrderList {
		binanceUpdateOrder(binanceOrder)
	}
}

func binanceOrderQuery(pair string, orderid int) {
	queryParams := fmt.Sprintf(binanceOrderQueryParams, pair, orderid)
	respBytes := binanceRestAPI("GET", binanceRestURL+"/order?", queryParams)

	binanceOrder := binanceOrderType{}
	json.Unmarshal(respBytes, &binanceOrder)
	time.Sleep(time.Millisecond * 100)

	newOrder := getOrder(orderid, "binance")
	newOrder.Status = binanceOrder.Status
	updateOrder(newOrder)

	binanceCheckError(respBytes)
}

func binanceOrderCreate(pair, side, price string, quantity, stoploss, takeprofit float64, autorepeat, reforderid int) {

	orderParams := fmt.Sprintf(binanceOrderCreateParams, pair, side, price, quantity)
	respBytes := binanceRestAPI("POST", binanceRestURL+"/order?", orderParams)

	//Check if Response is an Error
	binanceCheckError(respBytes)

	binanceOrder := binanceOrderType{}
	json.Unmarshal(respBytes, &binanceOrder)

	if binanceOrder.OrderID == 0 {
		return
	}

	time.Sleep(time.Millisecond * 375)
	newOrder := getOrder(binanceOrder.OrderID, "binance")
	if newOrder.OrderID == 0 {
		time.Sleep(time.Second * 2)
		newOrder = getOrder(binanceOrder.OrderID, "binance")
	}

	if newOrder.OrderID == 0 {
		return
	}

	newOrder.Stoploss = stoploss
	newOrder.Takeprofit = takeprofit
	newOrder.AutoRepeat = autorepeat

	if newOrder.Stoploss > 0 || newOrder.Takeprofit > 0 {
		newOrder.RefEnabled = 1
	}

	// log.Printf("New Order respBytes: %s\n", respBytes)
	// log.Printf("New Order Created: %+v \n", newOrder)

	newOrder.RefOrderID = reforderid
	updateOrder(newOrder)

	if reforderid > 0 {
		prvOrder := getOrder(reforderid, "binance")
		prvOrder.RefOrderID = binanceOrder.OrderID
		updateOrder(prvOrder)
	}
}

func binanceOrderCancel(pair string, orderid int) {
	orderParams := fmt.Sprintf(binanceOrderCancelParams, pair, orderid)
	respBytes := binanceRestAPI("DELETE", binanceRestURL+"/order?", orderParams)

	binanceOrder := binanceOrderType{}
	json.Unmarshal(respBytes, &binanceOrder)
	time.Sleep(time.Millisecond * 100)

	cancelledOrder := getOrder(orderid, "binance")
	cancelledOrder.Status = binanceOrder.Status
	updateOrder(cancelledOrder)
}

func binanceUpdateOrder(binanceOrder binanceOrderType) {

	if binanceOrder.Symbol == "" {
		return
	}

	order := getOrder(binanceOrder.OrderID, "binance")
	// sqlCheck := "select * from orders where orderid = $1 and exchange = 'binance' limit 1"
	// utils.SqlDB.Get(&order, sqlCheck, binanceOrder.OrderID)

	order.Exchange = "binance"
	if !(order.OrderID > 0) {
		order.Side = binanceOrder.Side
		order.Pair = binanceOrder.Symbol
		order.OrderID = binanceOrder.OrderID
		order.Status = binanceOrder.Status

		if binanceOrder.Time != 0 {
			order.Created = fmt.Sprintf("%s", time.Unix(binanceOrder.Time/1000, 0))
		}

		if binanceOrder.TransactTime != 0 {
			order.Created = fmt.Sprintf("%s", time.Unix(binanceOrder.TransactTime/1000, 0))
		}

		order.Price, _ = strconv.ParseFloat(binanceOrder.Price, 64)
		order.Quantity, _ = strconv.ParseFloat(binanceOrder.OrigQty, 64)
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
		order.Status = binanceOrder.Status
		if binanceOrder.Time != 0 {
			order.Updated = fmt.Sprintf("%s", time.Unix(binanceOrder.Time/1000, 0))
		}

		if binanceOrder.TransactTime != 0 {
			order.Updated = fmt.Sprintf("%s", time.Unix(binanceOrder.TransactTime/1000, 0))
		}
	}

	updateOrderOnly(order)
}
