package coinank_api

import (
	"context"
	"encoding/json"
	"fmt"
	"nofx/provider/coinank/coinank_enum"
	"testing"
	"time"
)

func TestDepthWs(t *testing.T) {
	ctx := context.TODO()
	ws, err := DepthWsConn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for tickers := range ws.DepthV3Ch {
			msg, err := json.Marshal(tickers)
			if err != nil {
				fmt.Println("json err:", err)
			}
			fmt.Println(string(msg))
		}
		fmt.Println("DepthV3Ch closed")
	}()
	err = ws.Subscribe("BTCUSDT", coinank_enum.Binance, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("sub success")
	time.Sleep(10 * time.Second)
	err = ws.UnSubscribe("BTCUSDT", coinank_enum.Binance, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("unsub success")
	time.Sleep(10 * time.Second)
	ws.Close()
	fmt.Println("cancel success")
}
