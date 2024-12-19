package main

import (
	"backpocket/models"
	"backpocket/utils"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

/*
	Binance Order Status:
		NEW, FILLED, CANCELLED, PARTIALLY_FILLED
*/

var (
	chanRestartBinanceOrderStream = make(chan bool, 10)
)

type binanceOrderType struct {
	OrderID, RefOrderID  uint64
	Takeprofit, Stoploss float64

	Time, TransactTime int64
	Price, OrigQty, ExecutedQty, CummulativeQuoteQty,
	Symbol, Status, Side string
}

// func binanceOrderBookStream() {

// for {
//loop through enabled markets.

//get the binanceOrders linked to the order

// time.After(time.Minute * 15)
// }
// }

func binanceAllOrders(pair string, starttime int64) {
	queryParams := fmt.Sprintf(binanceListOrdersParams, pair, starttime)
	respBytes := binanceRestAPI("GET", binanceRestURL+"/allOrders?", queryParams)

	var binanceOrderList []binanceOrderType
	json.Unmarshal(respBytes, &binanceOrderList)

	newBatchedOrders := []models.Order{}
	updateBatchedOrders := []models.Order{}
	for _, binanceOrder := range binanceOrderList {
		if binanceOrder.Symbol == "" {
			continue
		}
		order, isnew := binanceUpdateOrder(binanceOrder)
		if isnew {
			newBatchedOrders = append(newBatchedOrders, order)
		} else {
			updateBatchedOrders = append(updateBatchedOrders, order)
		}
	}

	// log.Printf("len(newBatchedOrders): %s %v \n", pair, len(newBatchedOrders))
	// log.Printf("len(updateBatchedOrders): %s %v \n", pair, len(updateBatchedOrders))

	if len(newBatchedOrders) > 0 {
		if err := utils.SqlDB.Transaction(func(tx *gorm.DB) error {
			if err := tx.CreateInBatches(newBatchedOrders, 500).Error; err != nil {
				return err //Rollback
			}
			return nil
		}); err != nil {
			log.Println("Error Creating Batches: ", err.Error())
			log.Printf("newBatchedOrders: %+v \n", newBatchedOrders)
		}
	}

	if len(updateBatchedOrders) > 0 {
		values := make([]clause.Expr, 0, len(updateBatchedOrders))
		for _, order := range updateBatchedOrders {
			values = append(values, gorm.Expr("(?::bigint, ?, ?::double precision, ?::double precision, ?::timestamp) ", order.ID, order.Status, order.Quantity, order.Total, order.Updatedate))
		}

		batchedValues := make([]clause.Expr, 0, 250)
		for i, v := range values {
			batchedValues = append(batchedValues, v)
			if (i+1)%250 == 0 {
				batchedUpdateQueryOrders(batchedValues)
				batchedValues = make([]clause.Expr, 0, 250)
			}
		}

		if len(batchedValues) > 0 {
			batchedUpdateQueryOrders(batchedValues)
		}
	}
}

func batchedUpdateQueryOrders(batchedValues []clause.Expr) {
	valuesExpr := gorm.Expr("?", batchedValues)
	valuesExpr.WithoutParentheses = true

	if tx := utils.SqlDB.Exec(
		"UPDATE orders SET status = tmp.status, quantity = tmp.quantity, total = tmp.total, updatedate = tmp.updatedate FROM (VALUES ?) tmp(id,status,quantity,total,updatedate) WHERE orders.id = tmp.id",
		valuesExpr,
	); tx.Error != nil {
		log.Printf("Error Creating Batches: %+v \n", tx.Error)
	}
	time.Sleep(time.Millisecond * 100)
}

func binanceOrderQuery(pair string, orderid uint64) {
	queryParams := fmt.Sprintf(binanceOrderQueryParams, pair, orderid)
	respBytes := binanceRestAPI("GET", binanceRestURL+"/order?", queryParams)

	binanceOrder := binanceOrderType{}
	json.Unmarshal(respBytes, &binanceOrder)
	time.Sleep(time.Millisecond * 100)

	newOrder := getOrder(orderid, "binance")
	newOrder.Status = binanceOrder.Status
	updateOrderAndSave(newOrder, true)

	binanceCheckError(respBytes)
}

func binanceOrderCreate(pair, side, price, quantity string, stoploss, takeprofit float64, autorepeat int, reforderid uint64) {

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
		log.Println("New Order not found in OrderList, check if binance sent an executionReport response on the websocket")
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
	updateOrderAndSave(newOrder, true)

	if reforderid > 0 {
		prvOrder := getOrder(reforderid, "binance")
		prvOrder.RefOrderID = binanceOrder.OrderID
		updateOrderAndSave(prvOrder, true)
	}
}

func binanceOrderCancel(pair string, orderid uint64) {
	orderParams := fmt.Sprintf(binanceOrderCancelParams, pair, orderid)
	respBytes := binanceRestAPI("DELETE", binanceRestURL+"/order?", orderParams)

	binanceOrder := binanceOrderType{}
	json.Unmarshal(respBytes, &binanceOrder)
	time.Sleep(time.Millisecond * 100)

	cancelledOrder := getOrder(orderid, "binance")
	cancelledOrder.Status = binanceOrder.Status
	updateOrderAndSave(cancelledOrder, true)
}

func binanceUpdateOrder(binanceOrder binanceOrderType) (order models.Order, isnew bool) {

	order = getOrder(binanceOrder.OrderID, "binance")

	order.Exchange = "binance"
	if !(order.OrderID > 0) {
		isnew = true
		order.ID = models.TableID()
		order.Side = binanceOrder.Side
		order.Pair = binanceOrder.Symbol
		order.OrderID = binanceOrder.OrderID
		order.Status = binanceOrder.Status

		if binanceOrder.Time != 0 {
			order.Createdate = time.Unix(binanceOrder.Time/1000, 0)
		}

		if binanceOrder.TransactTime != 0 {
			order.Createdate = time.Unix(binanceOrder.TransactTime/1000, 0)
		}

		order.Price, _ = strconv.ParseFloat(binanceOrder.Price, 64)
		order.Quantity, _ = strconv.ParseFloat(binanceOrder.OrigQty, 64)
		order.Total = utils.TruncateFloat(order.Price*order.Quantity, 8)

		cummulativeQuoteQty, _ := strconv.ParseFloat(binanceOrder.CummulativeQuoteQty, 64)
		executedQty, _ := strconv.ParseFloat(binanceOrder.ExecutedQty, 64)
		if binanceOrder.Status == "CANCELED" && executedQty > 0 {
			order.Status = "FILLED"
			order.Quantity = executedQty
			order.Total = cummulativeQuoteQty
		}
	} else {
		order.Status = binanceOrder.Status
		executedQty, _ := strconv.ParseFloat(binanceOrder.ExecutedQty, 64)
		cummulativeQuoteQty, _ := strconv.ParseFloat(binanceOrder.CummulativeQuoteQty, 64)
		if binanceOrder.Status == "CANCELED" && executedQty > 0 {
			order.Status = "FILLED"
		}

		order.Quantity = executedQty
		order.Total = cummulativeQuoteQty

		if binanceOrder.Time != 0 {
			order.Updatedate = time.Unix(binanceOrder.Time/1000, 0)
		}

		if binanceOrder.TransactTime != 0 {
			order.Updatedate = time.Unix(binanceOrder.TransactTime/1000, 0)
		}
	}

	go updateOrderAndSave(order, false)
	return
}
