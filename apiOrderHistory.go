package main

import (
	"backpocket/utils"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func wsHandlerOrderHistory(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		ticker := time.NewTicker(pingPeriod)
		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})
		defer ticker.Stop()
		defer wsConn.Close()

		for {
			msg := searchOrderMsgType{}
			select {
			case <-ticker.C:
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					println("websocket.PingMessage: ", err.Error())
					continue
				}

			default:
				if err := wsConn.ReadJSON(&msg); err != nil {
					continue
				}

				if msg.Pair == "" {
					continue
				}

				if err := wsConn.WriteJSON(searchOrderSQL(msg)); err != nil {
					log.Println(err.Error())
					continue
				}
				//check if msg pair is valid and we can create a new set of orders for it
				//

			}
		}
	}
}

type searchOrderMsgType struct {
	Pair, Status, Side,
	Start, Stop, OrderID,
	RefID, Exchange string
}

func searchOrderSQL(msg searchOrderMsgType) (filteredOrderList []orders) {
	//select from sqlitedb into array of orders
	var sqlParams []interface{}
	sqlSearch := "select * from orders where "

	if msg.Pair == "" {
		msg.Pair = "%"
	} else {
		msg.Pair = strings.Replace(msg.Pair, "|", "%", 1)
	}
	sqlParams = append(sqlParams, msg.Pair)
	sqlSearch += fmt.Sprintf(" pair like $%v ", len(sqlParams))

	if msg.Status != "" {
		sqlParams = append(sqlParams, msg.Status)
		sqlSearch += fmt.Sprintf(" and status = $%v ", len(sqlParams))
	}

	if msg.Side != "" {
		sqlParams = append(sqlParams, msg.Side)
		sqlSearch += fmt.Sprintf(" and side = $%v ", len(sqlParams))
	}

	if msg.Exchange != "" {
		sqlParams = append(sqlParams, msg.Exchange)
		sqlSearch += fmt.Sprintf(" and exchange = $%v ", len(sqlParams))
	}

	if msg.OrderID != "" {
		sqlParams = append(sqlParams, "%"+msg.OrderID+"%")
		sqlSearch += fmt.Sprintf(" and cast(orderid as text) like $%v ", len(sqlParams))
	}

	if msg.RefID != "" {
		sqlParams = append(sqlParams, "%"+msg.RefID+"%")
		sqlSearch += fmt.Sprintf(" and cast(refid as text) like $%v ", len(sqlParams))
	}

	//ass sql query to check dates between
	if msg.Start != "" && msg.Stop != "" {
		sqlParams = append(sqlParams, msg.Start)
		sqlSearch += fmt.Sprintf(" and created >= $%v ", len(sqlParams))

		sqlParams = append(sqlParams, msg.Stop)
		sqlSearch += fmt.Sprintf(" and created <= $%v ", len(sqlParams))
	}

	sqlSearch += " order by created limit 1024000"

	if err := utils.SqlDB.Select(&filteredOrderList, sqlSearch, sqlParams...); err != nil {
		log.Println(err.Error())
		log.Println(sqlSearch)
		log.Println(sqlParams)
	}

	return
}
