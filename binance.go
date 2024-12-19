package main

import (
	"backpocket/utils"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	binanceRestURL      = "https://api.binance.com/api/v3"
	binanceWebsocketURL = "wss://stream.binance.com:9443/stream?streams="

	binanceListOrdersParams  = "symbol=%s&startTime=%v&limit=1000"
	binanceOrderQueryParams  = "symbol=%s&orderId=%d"
	binanceOrderCancelParams = "symbol=%s&orderId=%d"
	binanceOrderCreateParams = "type=LIMIT&timeInForce=GTC&symbol=%s&side=%s&price=%s&quantity=%s"
)

var (
	binanceAPIKey    string
	binanceSecretkey string
)

type binanceErrorType struct {
	Code int    `json:",omitempty"`
	Msg  string `json:",omitempty"`
}

type binanceMarketWSRequest struct {
	ID     uint     `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params,omitempty"`
}

type binanceStreamTradeResp struct {
	ID     uint     `json:"id,omitempty"`
	Result []string `json:"result,omitempty"`

	Stream string `json:"stream,omitempty"`
	Data   struct {
		Event        string `json:"e"`
		EventTime    uint   `json:"E"`
		Symbol       string `json:"s"`
		TradeID      uint   `json:"a"`
		Price        string `json:"p"`
		Quantity     string `json:"q"`
		FirstID      uint   `json:"f"`
		LastID       uint   `json:"l"`
		TradeTime    uint   `json:"T"`
		IsBuyerMaker bool   `json:"m"`
	} `json:"data,omitempty"`
}

type binanceStreamKlineResp struct {
	ID     uint     `json:"id,omitempty"`
	Result []string `json:"result,omitempty"`

	Stream string `json:"stream,omitempty"`
	Data   struct {
		Event     string `json:"e"`
		EventTime uint   `json:"E"`
		Symbol    string `json:"s"`
		Kline     struct {
			StartTime    uint   `json:"t"`
			CloseTime    uint   `json:"T"`
			Symbol       string `json:"s"`
			Interval     string `json:"i"`
			FirstTradeID int    `json:"f"`
			LastTradeID  int    `json:"L"`

			Open        string `json:"o"`
			Close       string `json:"c"`
			High        string `json:"h"`
			Low         string `json:"l"`
			Volume      string `json:"v"`
			NumOfTrades int    `json:"n"`
			Closed      bool   `json:"x"`
			VolumeQuote string `json:"q"`

			TakerBuyBase  string `json:"V"`
			TakerBuyQuote string `json:"Q"`
		} `json:"k,omitempty"`
	} `json:"data,omitempty"`

	// Data struct {
	// 	Symbol       string `json:"s"`
	// 	UpdateID     uint   `json:"u"`
	// 	BestBidPrice string `json:"b"`
	// 	BestBidQty   string `json:"B"`
	// 	BestAskPrice string `json:"a"`
	// 	BestAskQty   string `json:"A"`
	// } `json:"data,omitempty"`
}

/*
{
  "e": "kline",     // Event type
  "E": 123456789,   // Event time
  "s": "BNBBTC",    // Symbol
  "k": {
    "t": 123400000, // Kline start time
    "T": 123460000, // Kline close time
    "s": "BNBBTC",  // Symbol
    "i": "1m",      // Interval
    "f": 100,       // First trade ID
    "L": 200,       // Last trade ID
    "o": "0.0010",  // Open price
    "c": "0.0020",  // Close price
    "h": "0.0025",  // High price
    "l": "0.0015",  // Low price
    "v": "1000",    // Base asset volume
    "n": 100,       // Number of trades
    "x": false,     // Is this kline closed?
    "q": "1.0000",  // Quote asset volume
    "V": "500",     // Taker buy base asset volume
    "Q": "0.500",   // Taker buy quote asset volume
    "B": "123456"   // Ignore
  }
}
*/

type binanceStreamBookDepthResp struct {
	ID     uint     `json:"id,omitempty"`
	Result []string `json:"result,omitempty"`

	Stream string `json:"stream,omitempty"`
	Data   struct {
		LastUpdateID uint `json:"lastUpdateId"`

		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
	} `json:"data,omitempty"`

	// Data struct {
	// 	Symbol       string `json:"s"`
	// 	UpdateID     uint   `json:"u"`
	// 	BestBidPrice string `json:"b"`
	// 	BestBidQty   string `json:"B"`
	// 	BestAskPrice string `json:"a"`
	// 	BestAskQty   string `json:"A"`
	// } `json:"data,omitempty"`
}
type binanceStreamAssetResp struct {
	Stream string `json:"stream,omitempty"`
	Data   struct {
		Event     string `json:"e"`
		EventTime uint   `json:"E"`
	} `json:"data,omitempty"`
}

// outboundAccountPosition fields
type binanceOutboundAccountPosition struct {
	Stream string `json:"stream,omitempty"`
	Data   struct {
		Event     string `json:"e"`
		EventTime uint   `json:"E"`

		Balances []struct {
			Asset  string `json:"a,omitempty"`
			Free   string `json:"f,omitempty"`
			Locked string `json:"l,omitempty"`
		} `json:"B,omitempty"`
	} `json:"data,omitempty"`
}

// balanceUpdate fields
type binanceBalanceUpdate struct {
	Stream string `json:"stream,omitempty"`
	Data   struct {
		Event     string `json:"e"`
		EventTime uint   `json:"E"`

		//balanceUpdate fields
		Asset           string `json:"a,omitempty"`
		BalanceDelta    string `json:"d,omitempty"`
		Transactiontime int    `json:"T,omitempty"`
	} `json:"data,omitempty"`
}

// executionReport fields
type binanceExecutionReport struct {
	Stream string `json:"stream,omitempty"`
	Data   struct {
		Event     string `json:"e"`
		EventTime uint   `json:"E"`

		//executionReport fields
		Symbol                        string `json:"s,omitempty"`
		ClientOrderID                 string `json:"c,omitempty"`
		Side                          string `json:"S,omitempty"`
		OrderType                     string `json:"o,omitempty"`
		TimeInForce                   string `json:"f,omitempty"`
		OrderQuantity                 string `json:"q,omitempty"`
		OrderPrice                    string `json:"p,omitempty"`
		StopPrice                     string `json:"P,omitempty"`
		IcebergQuantity               string `json:"F,omitempty"`
		OrderListID                   int    `json:"g,omitempty"`
		OriginalClientOID             string `json:"C,omitempty"`
		CurrentExecutionType          string `json:"x,omitempty"`
		CurrentOrderStatus            string `json:"X,omitempty"`
		OrderRejectReason             string `json:"r,omitempty"`
		OrderID                       uint64 `json:"i,omitempty"`
		LastExecutedQty               string `json:"l,omitempty"`
		CummulativeFilledQty          string `json:"z,omitempty"`
		LastExecutedPrice             string `json:"L,omitempty"`
		CommissionAmount              string `json:"n,omitempty"`
		CommissionAsset               string `json:"N,omitempty"`
		TransactionTime               int64  `json:"T,omitempty"`
		TradeID                       int    `json:"t,omitempty"`
		Ignore1                       uint   `json:"I,omitempty"`
		IsOrderOnBook                 bool   `json:"w,omitempty"`
		IsTradeMakerSide              bool   `json:"m,omitempty"`
		Ignore2                       bool   `json:"M,omitempty"`
		CreationTime                  int64  `json:"O,omitempty"`
		CummulativeQuoteTransactedQty string `json:"Z,omitempty"`
		LastQuoteTransactedQty        string `json:"Y,omitempty"`
		QuoteOrderQty                 string `json:"Q,omitempty"`
		WorkingTime                   int64  `json:"W,omitempty"`
		SelfTradePrevention           string `json:"V,omitempty"`
	} `json:"data,omitempty"`
}

func binanceKeys() {
	binanceAPIKey = utils.Config.Binance.Key
	binanceSecretkey = utils.Config.Binance.Secret
	if binanceAPIKey == "" || binanceSecretkey == "" {
		log.Fatalf("binanceApiKey or binanceSecretkey environment variables are missing \n")
	}
}

func binanceCheckError(respBytes []byte) {
	binanceError := binanceErrorType{}
	json.Unmarshal(respBytes, &binanceError)

	if binanceError.Msg != "" {
		wsBroadcastNotification <- notifications{
			Type: "info", Title: "*Binance Exchange*", Message: binanceError.Msg,
		}
	}
}

func binanceRestAPI(method, url, params string) []byte {
	if method != "POST" && method != "GET" && method != "DELETE" {
		return nil
	}

	timestamp := fmt.Sprintf("recvWindow=60000&timestamp=%v", time.Now().UnixNano()/int64(time.Millisecond))
	if params != "" {
		params += "&"
	}
	params += timestamp

	keyByte := []byte(binanceSecretkey)
	hmacnew := hmac.New(sha256.New, keyByte)
	hmacnew.Write([]byte(params))

	url += params + "&signature=" + hex.EncodeToString(hmacnew.Sum(nil))
	httpClient := http.Client{Timeout: time.Duration(time.Second * 30)}
	httpRequest, _ := http.NewRequest(method, url, nil)
	httpRequest.Header.Set("X-MBX-APIKEY", binanceAPIKey)

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		log.Println(err.Error())
		return nil
	}

	defer httpResponse.Body.Close()
	bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		log.Println(err.Error())
		return nil
	}

	return bodyBytes
}

func binanceWSDisconnect(idCount uint, streamParams []string, bwConn *websocket.Conn) {
	idCount = idCount + 1
	bReq := binanceMarketWSRequest{ID: idCount, Method: "UNSUBSCRIBE", Params: streamParams}
	if err := bwConn.WriteJSON(bReq); err != nil {
		log.Panicf("websocket write error: %v \n", err)
	}

	if err := bwConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		log.Panicf("disconnection error: %v \n", err)
	}

	bwConn.Close()
}

func binanceWSConnect(streamParams []string) (bwConn *websocket.Conn) {
	var err error

	// log.Println("Opening connection to binance")

	if len(streamParams) == 0 {
		log.Println("empty streamParam")
	}

	streamURL := strings.Join(streamParams, "/")
	bReq := binanceMarketWSRequest{ID: 1, Method: "SUBSCRIBE", Params: streamParams}

	if bwConn, _, err = websocket.DefaultDialer.Dial(binanceWebsocketURL+streamURL, nil); err != nil {
		log.Println("dial:", err)
	}

	if bwConn == nil {
		log.Println("websocket connection error")
		return
	}

	if err := bwConn.WriteJSON(bReq); err != nil {
		log.Printf("websocket write error: %v \n", err)
	}

	// println(binanceWebsocketURL + streamURL)

	return
}
