package coinank_api

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank"
	"nofx/provider/coinank/coinank_enum"
	"strconv"
	"strings"

	"golang.org/x/net/websocket"
)

const MainWsUrl = "wss://ws.coinank.com/ws"

type KlineWs struct {
	conn      *websocket.Conn
	KlineCh   <-chan *WsResult[coinank.KlineResult]
	TickersCh <-chan *WsResult[KlineTickers]
}

// WsConn connect ws , read data from KlineCh and TickersCh
func WsConn(ctx context.Context, needKline bool, needTicker bool) (*KlineWs, error) {
	conn, ch, err := ws(ctx)
	if err != nil {
		return nil, err
	}
	klineCh, tickersCh := handleResponse(ch, needKline, needTicker)
	ws := &KlineWs{
		conn:      conn,
		KlineCh:   klineCh,
		TickersCh: tickersCh,
	}
	return ws, nil
}

// Subscribe subscribe kline
func (ws *KlineWs) Subscribe(symbol string, exchange coinank_enum.Exchange, interval coinank_enum.Interval) error {
	var args = "kline@" + symbol + "@" + string(exchange) + "@" + string(interval)
	info := SubscribeInfo{
		Op:   "subscribe",
		Args: args,
	}
	json, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = websocket.Message.Send(ws.conn, json)
	if err != nil {
		return err
	}
	return nil
}

// UnSubscribe unsubscribe kline
func (ws *KlineWs) UnSubscribe(symbol string, exchange coinank_enum.Exchange, interval coinank_enum.Interval) error {
	var args = "kline@" + symbol + "@" + string(exchange) + "@" + string(interval)
	info := SubscribeInfo{
		Op:   "unsubscribe",
		Args: args,
	}
	json, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = websocket.Message.Send(ws.conn, json)
	if err != nil {
		return err
	}
	return nil
}

// Close websocket
func (ws *KlineWs) Close() error {
	return ws.conn.Close()
}

func ws(ctx context.Context) (*websocket.Conn, <-chan string, error) {
	config, err := websocket.NewConfig(MainWsUrl, "http://localhost")
	if err != nil {
		return nil, nil, err
	}
	conn, err := config.DialContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	ch := make(chan string, 1024)
	go read(conn, ch)
	return conn, ch, nil
}

func read(conn *websocket.Conn, ch chan string) {
	defer conn.Close()
	defer close(ch)
	for {
		var msg string
		err := websocket.Message.Receive(conn, &msg)
		if err != nil {
			return
		}
		ch <- msg
	}
}

func handleResponse(ch <-chan string, needKline bool, needTicker bool) (<-chan *WsResult[coinank.KlineResult], <-chan *WsResult[KlineTickers]) {
	klineCh := make(chan *WsResult[coinank.KlineResult], 1024)
	tickersCh := make(chan *WsResult[KlineTickers], 1024)
	go func() {
		if needKline {
			defer close(klineCh)
		} else {
			close(klineCh)
		}
		if needTicker {
			defer close(tickersCh)
		} else {
			close(tickersCh)
		}
		for msg := range ch {
			if needKline && strings.HasPrefix(msg, "{\"op\":\"push\",\"success\":true,\"args\":\"kline") {
				var result WsResult[[]any]
				err := json.Unmarshal([]byte(msg), &result)
				if err == nil && result.Success {
					kline := coinank.KlineResult{}
					k := result.Data
					kline.StartTime = toInt64(k[0])
					kline.EndTime = toInt64(k[1])
					kline.Open = toFloat64(k[2])
					kline.Close = toFloat64(k[3])
					kline.High = toFloat64(k[4])
					kline.Low = toFloat64(k[5])
					kline.Volume = toFloat64(k[6])
					kline.Quantity = toFloat64(k[7])
					kline.Count = toFloat64(k[8])
					var resp WsResult[coinank.KlineResult]
					resp.Success = result.Success
					resp.Data = kline
					resp.Args = result.Args
					resp.Op = result.Op
					klineCh <- &resp
				}
			} else if needTicker && strings.HasPrefix(msg, "{\"op\":\"push\",\"success\":true,\"args\":\"tickers") {
				var result WsResult[KlineTickers]
				err := json.Unmarshal([]byte(msg), &result)
				if err == nil && result.Success {
					tickersCh <- &result
				}
			}
		}
	}()
	return klineCh, tickersCh
}

func toInt64(v any) int64 {
	f := toFloat64(v)
	return int64(f)
}

func toFloat64(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	if f, ok := v.(string); ok {
		s, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return 0
		}
		return s
	}
	return 0
}

type SubscribeInfo struct {
	Op   string `json:"op"`
	Args string `json:"args"`
}

type KlineTickers struct {
	OiCcy          string `json:"oiCcy"`
	OiVol          string `json:"oiVol"`
	Symbol         string `json:"symbol"`
	ExchangeName   string `json:"exchangeName"`
	PriceChange24H string `json:"priceChange24h"`
	Low24H         string `json:"low24h"`
	High24H        string `json:"high24h"`
	VolCcy24H      string `json:"volCcy24h"`
	LastPrice      string `json:"lastPrice"`
	Vol24H         string `json:"vol24h"`
	Turnover24H    string `json:"turnover24h"`
	OiUSD          string `json:"oiUSD"`
	FundingRate    string `json:"fundingRate"`
	LastOiVol      string `json:"lastOiVol"`
	MarkPrice      string `json:"markPrice"`
	BasisRate      string `json:"basisRate"`
	Basis          string `json:"basis"`
}

type WsResult[T any] struct {
	Op      string `json:"op"`
	Success bool   `json:"success"`
	Args    string `json:"args"`
	Data    T      `json:"data"`
}
