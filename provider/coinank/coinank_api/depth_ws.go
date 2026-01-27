package coinank_api

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"

	"golang.org/x/net/websocket"
)

const MainDepthWsUrl = "wss://ws.coinank.com/wsDepth/wsKline"

type DepthWs struct {
	conn      *websocket.Conn
	DepthV3Ch <-chan *WsResult[DepthV3]
}

// DepthWsConn connect ws , read data from DepthV3Ch
func DepthWsConn(ctx context.Context) (*DepthWs, error) {
	conn, ch, err := depth_ws(ctx)
	if err != nil {
		return nil, err
	}
	ws := &DepthWs{
		conn:      conn,
		DepthV3Ch: ch,
	}
	return ws, nil
}

// Subscribe subscribe depth
func (ws *DepthWs) Subscribe(symbol string, exchange coinank_enum.Exchange, step string) error {
	var args = "depthV3@" + symbol + "@" + string(exchange) + "@SWAP@" + step
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

// UnSubscribe unsubscribe depth
func (ws *DepthWs) UnSubscribe(symbol string, exchange coinank_enum.Exchange, step string) error {
	var args = "depthV3@" + symbol + "@" + string(exchange) + "@SWAP@" + step
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
func (ws *DepthWs) Close() error {
	return ws.conn.Close()
}

func depth_ws(ctx context.Context) (*websocket.Conn, <-chan *WsResult[DepthV3], error) {
	config, err := websocket.NewConfig(MainDepthWsUrl, "http://localhost")
	if err != nil {
		return nil, nil, err
	}
	conn, err := config.DialContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	ch := make(chan *WsResult[DepthV3], 1024)
	go depth_read(conn, ch)
	return conn, ch, nil
}

func depth_read(conn *websocket.Conn, ch chan *WsResult[DepthV3]) {
	defer conn.Close()
	defer close(ch)
	var msg string
	for {
		err := websocket.Message.Receive(conn, &msg)
		if err != nil {
			return
		}
		var depth WsResult[DepthV3]
		err = json.Unmarshal([]byte(msg), &depth)
		if err == nil {
			ch <- &depth
		}
	}
}

type DepthV3 struct {
	Type string     `json:"type"`
	Ts   uint64     `json:"ts"`
	Asks [][]string `json:"asks"`
	Bids [][]string `json:"bids"`
}
