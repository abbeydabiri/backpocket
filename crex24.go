package main

import (
	"backpocket/utils"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/x2v3/signalr"
	"github.com/x2v3/signalr/hubs"
)

const (
	crex24RestURL    = "https://api.crex24.com"
	crex24SignalrURL = "api.crex24.com"
	crex24SignalrHub = "publicCryptoHub"

	crex24WebsocketURL = "wss://stream.crex24.com:9443/stream?streams="

	crex24ListOrdersParams  = "instrument=%s"
	crex24OrderQueryParams  = "id=%d"
	crex24OrderCancelParams = `{"ids":[%v]}`
	crex24OrderCreateParams = `{"instrument":"%s","side","%s",price=%f,volume=%f, type="limit",timeInForce="GTC"}`
	// crex24OrderCreateParams = "type=LIMIT&timeInForce=GTC&symbol=%s&side=%s&price=%s&quantity=%f"
)

var (
	crex24APIKey    string
	crex24Secretkey string
)

type crex24ErrorType struct {
	ErrorDescription string `json:",errorDescription"`
}

type crex24MarketWSRequest struct {
	ID     uint     `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params,omitempty"`
}

// type crex24StreamTradeResp struct {
// 	ID     uint     `json:"id,omitempty"`
// 	Result []string `json:"result,omitempty"`

// 	Stream string `json:"stream,omitempty"`
// 	Data   struct {
// 		Event        string `json:"e"`
// 		EventTime    uint   `json:"E"`
// 		Symbol       string `json:"s"`
// 		TradeID      uint   `json:"t"`
// 		Price        string `json:"p"`
// 		Quantity     string `json:"q"`
// 		BuyerOrdID   uint   `json:"b"`
// 		SellerOrdID  uint   `json:"a"`
// 		TradeTime    uint   `json:"T"`
// 		IsBuyerMaker bool   `json:"m"`
// 	} `json:"data,omitempty"`
// }

type crex24WSTickerResp struct {
	LST []struct {
		I, //instrument
		LST, //last price
		PC, //percentage change
		L, //lowest price
		H, //highest price
		BV, //base volume
		QV, //quote volume
		VB, //volume in BTC
		VU string //volume in USD
	} `json:"LST,omitempty"`

	U []struct {
		I string
		U []struct {
			V, N string
		}
	} `json:"U,omitempty"`
}

type crex24StreamTradeResp struct {
	I   string
	LST []struct {
		P,
		V,
		S string
		T uint
	} `json:"LST,omitempty"`

	NT []struct {
		P,
		V,
		S string
		T uint
	} `json:"NT,omitempty"`
}

type crex24StreamBookDepthResp struct {
	I string
	B []struct {
		P, V string
	} `json:"B,omitempty"`

	S []struct {
		P, V string
	} `json:"S,omitempty"`

	BU []struct {
		V interface{} //map[string]string
		N string
	} `json:"BU,omitempty"`

	SU []struct {
		V interface{} //map[string]string
		N string
	} `json:"SU,omitempty"`
}

func crex24Keys() {
	crex24APIKey = utils.Config.Crex24.Key
	crex24Secretkey = utils.Config.Crex24.Secret
	if crex24APIKey == "" || crex24Secretkey == "" {
		log.Panicf("crex24APIkey or crex24Secretkey environment variables are missing \n")
	}
}

func crex24CheckError(respBytes []byte) {
	crex24Error := crex24ErrorType{}
	json.Unmarshal(respBytes, &crex24Error)

	if crex24Error.ErrorDescription != "" {
		wsBroadcastNotification <- notifications{
			Type: "info", Title: "*Crex24 Exchange*", Message: crex24Error.ErrorDescription,
		}
	}
}

func crex24RestAPI(method, urlPath string, urlBody []byte) []byte {
	if method != "POST" && method != "GET" && method != "DELETE" {
		return nil
	}

	apiNonce := fmt.Sprintf("%v", time.Now().UnixNano())
	hmac512Msg := urlPath + apiNonce

	if urlBody != nil && method == "POST" {
		hmac512Msg += fmt.Sprintf("%s", urlBody)
	}

	keyByte := []byte(crex24Secretkey)
	hmac512new := hmac.New(sha512.New, keyByte)
	hmac512new.Write([]byte(hmac512Msg))
	hmac512Sign := hex.EncodeToString(hmac512new.Sum(nil))

	httpClient := http.Client{Timeout: time.Duration(time.Second * 30)}
	httpRequest, _ := http.NewRequest(method, crex24RestURL+urlPath, nil)
	httpRequest.Header.Set("X-CREX24-API-KEY", crex24APIKey)
	httpRequest.Header.Set("X-CREX24-API-NONCE", apiNonce)
	httpRequest.Header.Set("X-CREX24-API-SIGN", hmac512Sign)

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		log.Println(err.Error())
		return nil
	}

	defer httpResponse.Body.Close()
	bodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		log.Println(err.Error())
		return nil
	}

	return bodyBytes
}

func crex24WSDisconnect(streamParams []interface{}, method string, signalrClient *signalr.Client) {
	err := signalrClient.Send(hubs.ClientMsg{
		H: crex24SignalrHub, M: method,
		A: streamParams, I: 1,
	})

	if err != nil {
		log.Println(err.Error())
	}
	signalrClient.Close()
}

func crex24WSConnect() (signalRClient *signalr.Client) {

	// Set the user agent to one that looks like a browser.
	signalRClient = signalr.New(crex24SignalrURL, "2.2", "/signalr", `[{"name":"`+crex24SignalrHub+`"}]`, nil)
	signalRClient.Headers["User-Agent"] = "Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2228.0 Safari/537.36"

	return
	/*
		// Define message and error handlers.
		msgHandler := func(msg signalr.Message) { log.Println(msg) }
		logIfErr := func(err error) {
			if err != nil {
				log.Println(err)
				return
			}
		}

		// Start the connection.
		err := signalrClient.Run(msgHandler, logIfErr)
		logIfErr(err)


		// Subscribe to the USDT-BTC feed.
		err = signalrClient.Send(hubs.ClientMsg{
			H: "publicCryptoHub",
			M: channel,
			A: []interface{}{"USDT-BTC"},
			I: 1,
		})
		logIfErr(err)

	*/
}
