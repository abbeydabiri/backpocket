package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	wsConnNotificationsMutex = sync.RWMutex{}

	wsConnNotifications     = make(map[*websocket.Conn]bool)
	wsBroadcastNotification = make(chan notifications, 10240)
)

type notifications struct {
	Type, Title, Message string
}

func wsHandlerNotifications(httpRes http.ResponseWriter, httpReq *http.Request) {
	if wsConn := wsHandleConnections(httpRes, httpReq); wsConn != nil {

		wsConn.SetPongHandler(func(string) error {
			wsConn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		wsConnNotificationsMutex.Lock()
		wsConnNotifications[wsConn] = true
		wsConnNotificationsMutex.Unlock()

	}
}

func wsHandlerNotificationBroadcast() {
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			wsConnNotificationsMutex.Lock()
			for wsConn := range wsConnNotifications {
				if err := wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					delete(wsConnNotifications, wsConn)
					wsConn.Close()
				}
			}
			wsConnNotificationsMutex.Unlock()
		}
	}()

	go func() {

		for notify := range wsBroadcastNotification {
			if notify.Type == "" {
				continue
			}
			wsConnNotificationsMutex.Lock()
			for wsConn := range wsConnNotifications {
				if err := wsConn.WriteJSON(notify); err != nil {
					delete(wsConnNotifications, wsConn)
					wsConn.Close()
				}
			}
			wsConnNotificationsMutex.Unlock()
		}
	}()
}
