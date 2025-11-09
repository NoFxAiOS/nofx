package hook

import (
	"log"
	"net/http"

	"github.com/adshao/go-binance/v2/futures"
)

type NewBinanceTraderResult struct {
	Err    error
	Client *futures.Client
}

func (r *NewBinanceTraderResult) Error() error {
	if r.Err != nil {
		log.Printf("⚠️ 执行NewBinanceTraderResult时出错: %v", r.Err)
	}
	return r.Err
}

func (r *NewBinanceTraderResult) GetResult() *futures.Client {
	r.Error()
	return r.Client
}

type NewAsterTraderResult struct {
	Err    error
	Client *http.Client
}

func (r *NewAsterTraderResult) Error() error {
	if r.Err != nil {
		log.Printf("⚠️ 执行NewAsterTraderResult时出错: %v", r.Err)
	}
	return r.Err
}

func (r *NewAsterTraderResult) GetResult() *http.Client {
	r.Error()
	return r.Client
}
