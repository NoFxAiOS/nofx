package coinank_api

import (
	"context"
	"encoding/json"
	"fmt"
	"nofx/provider/coinank/coinank_enum"
	"testing"
	"time"
)

func TestKlineWs(t *testing.T) {
	ctx := context.TODO()
	ws, err := WsConn(ctx, true, true)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for tickers := range ws.TickersCh {
			msg, err := json.Marshal(tickers)
			if err != nil {
				fmt.Println("json err:", err)
			}
			fmt.Println(string(msg))
		}
		fmt.Println("tickersCh closed")
	}()
	go func() {
		for kline := range ws.KlineCh {
			msg, err := json.Marshal(kline)
			if err != nil {
				fmt.Println("json err:", err)
			}
			fmt.Println(string(msg))
		}
		fmt.Println("kline closed")
	}()
	err = ws.Subscribe("BTCUSDT", coinank_enum.Binance, coinank_enum.Minute1)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("sub success")
	time.Sleep(10 * time.Second)
	err = ws.UnSubscribe("BTCUSDT", coinank_enum.Binance, coinank_enum.Minute1)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("unsub success")
	time.Sleep(10 * time.Second)
	err = ws.Subscribe("BTCUSDT", coinank_enum.Binance, coinank_enum.Hour1)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("resub success")
	time.Sleep(10 * time.Second)
	ws.Close()
	fmt.Println("cancel success")
	time.Sleep(10 * time.Second)
	fmt.Println("all success")
}
