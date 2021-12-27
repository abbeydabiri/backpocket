package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"backpocket/utils"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/muzykantov/orderbook"
)

var (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorWhite  = "\033[37m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"

	// Time allowed to read the next pong message from the peer.
	pongWait = 180 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	OrderBook *orderbook.OrderBook
)

type wsResponseType struct {
	Action string
	Result interface{}
}

func main() {

	// OrderBook = orderbook.NewOrderBook()
	utils.StartLogger("")
	sqlDBInit()

	// crex24Keys()
	binanceKeys()

	// println(ETHNewMnemonic())

	dbSetupAssets()
	dbSetupOrders()
	dbSetupMarkets()

	// //crex24MarketGet()  //this setups up the markets for trading
	//this setups up the markets for trading
	// dbSetupMarkets()

	muxRouter := mux.NewRouter()

	wsHandlerAssetBroadcast()
	muxRouter.HandleFunc("/ws/assets", wsHandlerAssets)

	wsHandlerNotificationBroadcast()
	muxRouter.HandleFunc("/ws/notifications", wsHandlerNotifications)

	wsHandlerOrderBroadcast()
	muxRouter.HandleFunc("/ws/orders", wsHandlerOrders)
	muxRouter.HandleFunc("/ws/orderhistory", wsHandlerOrderHistory)

	wsHandlerTradeBroadcast()
	muxRouter.HandleFunc("/ws/trades", wsHandlerTrades)

	wsHandlerMarketBroadcast()
	muxRouter.HandleFunc("/ws/markets", wsHandlerMarkets)

	wsHandlerOrderbookBroadcast()
	muxRouter.HandleFunc("/ws/orderbooks", wsHandlerOrderbooks)

	// run our strategy process
	go apiStrategyStopLossTakeProfit()

	go binanceAssetGet()
	go binanceMarketGet()
	go binanceAssetStream()

	time.Sleep(time.Second * 2)

	go binanceTradeStream() //disabled due to data overflooding
	go binanceOrderBookStream()
	go binanceMarket24hrTicker()
	go binanceMarketOHLCVStream()

	//

	//crex24AssetGet
	// go crex24AssetGet()

	// go crex24TradeStream()
	// // go crex24OrderBookRest() //not needed as websocket from signalr does the job well
	// go crex24OrderBookStream()
	// go crex24Market24hrTicker()
	// go crex24MarketOHLCVStream()

	spa := spaHandler{staticPath: "uiapp", indexPath: "index.html"}
	muxRouter.PathPrefix("/").Handler(spa)

	println("listening")
	http.Handle("/", muxRouter)
	go log.Println(http.ListenAndServe(utils.Config.Address, nil))

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigCh:
		stackBuffer := make([]byte, 1<<16)
		runtime.Stack(stackBuffer, true)

		log.Println(string(stackBuffer))
		os.Exit(0)
	}
}

func wsHandleConnections(httpRes http.ResponseWriter, httpReq *http.Request) *websocket.Conn {
	// if r.Header.Get("Origin") != "http://"+r.Host {
	// 	http.Error(httpRes, "Origin not allowed", 403)
	// 	return nil
	// }

	wsConn, err := websocket.Upgrade(httpRes, httpReq, httpRes.Header(), 1024, 1024)
	if err != nil {
		http.Error(httpRes, "Could not open websocket connection", http.StatusBadRequest)
		return nil
	}

	return wsConn

}

type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// get the absolute path to prevent directory traversal
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		// if we failed to get the absolute path respond with a 400 bad request
		// and stop
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// prepend the path with the path to the static directory
	path = filepath.Join(h.staticPath, path)

	// check whether a file exists at the given path
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		// file does not exist, serve index.html
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	} else if err != nil {
		// if we got an error (that wasn't that the file doesn't exist) stating the
		// file, return a 500 internal server error and stop
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// otherwise, use http.FileServer to serve the static dir
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}
