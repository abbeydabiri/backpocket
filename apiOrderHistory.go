package main

import (
	"backpocket/models"
	"backpocket/utils"
	"log"
	"net/http"
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
					return
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

func searchOrderSQL(msg searchOrderMsgType) (filteredOrderList []models.Order) {

	var searchText string
	var searchParams []interface{}

	if msg.Pair != "" {
		searchText = " pair = ? "
		searchParams = append(searchParams, msg.Pair)
	}

	if msg.Exchange != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " exchange = ? "
		searchParams = append(searchParams, msg.Exchange)
	}

	if msg.Status != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " status = ? "
		searchParams = append(searchParams, msg.Status)
	}

	if msg.Side != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " side = ? "
		searchParams = append(searchParams, msg.Side)
	}

	if msg.RefID != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " refid = ?::bigint "
		searchParams = append(searchParams, msg.RefID)
	}

	if msg.OrderID != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " orderid = ?::bigint "
		searchParams = append(searchParams, msg.OrderID)
	}

	if msg.Start != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " createdate >= ?::timestamp "
		searchParams = append(searchParams, msg.Start)
	}

	if msg.Stop != "" {
		if searchText != "" {
			searchText += " AND "
		}
		searchText += " createdate <= ?::timestamp "
		searchParams = append(searchParams, msg.Stop)
	}

	orderby := "exchange, pair, orderid desc"

	if err := utils.SqlDB.Where(searchText, searchParams...).Order(orderby).Find(&filteredOrderList).Error; err != nil {
		log.Println(err.Error())
	}

	// sql := utils.SqlDB.ToSQL(func(tx *gorm.DB) *gorm.DB {
	// 	return tx.Where(searchText, searchParams...).Order(orderby).Find(&filteredOrderList)
	// })
	// log.Println(sql)

	return
}
