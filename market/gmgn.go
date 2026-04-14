package market

import (
	"fmt"
	gmgnprovider "nofx/provider/gmgn"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	gmgnAPIKeyMu sync.RWMutex
	gmgnAPIKey   string
)

func SetGMGNAPIKey(apiKey string) {
	gmgnAPIKeyMu.Lock()
	defer gmgnAPIKeyMu.Unlock()
	gmgnAPIKey = strings.TrimSpace(apiKey)
}

func getGMGNClient() (*gmgnprovider.Client, error) {
	gmgnAPIKeyMu.RLock()
	apiKey := gmgnAPIKey
	gmgnAPIKeyMu.RUnlock()
	if apiKey == "" {
		apiKey = os.Getenv("GMGN_API_KEY")
	}
	return gmgnprovider.NewClient(apiKey, "")
}

func getKlinesFromGMGN(symbol, resolution string, count int) ([]Kline, error) {
	chain, address, err := gmgnprovider.ParseSymbol(symbol)
	if err != nil {
		return nil, err
	}
	client, err := getGMGNClient()
	if err != nil {
		return nil, err
	}

	if count <= 0 {
		count = 100
	}
	duration := timeframeToDuration(resolution)
	if duration <= 0 {
		duration = 5 * time.Minute
	}
	to := time.Now().UTC()
	from := to.Add(-time.Duration(count) * duration)

	resp, err := client.GetTokenKline(chain, address, resolution, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	klines := make([]Kline, 0, len(resp.List))
	for _, item := range resp.List {
		openTime := item.Time
		if openTime < 1_000_000_000_000 {
			openTime *= 1000
		}
		closeTime := openTime + duration.Milliseconds()
		klines = append(klines, Kline{
			OpenTime:    openTime,
			Open:        gmgnprovider.ParseFloatString(item.Open),
			High:        gmgnprovider.ParseFloatString(item.High),
			Low:         gmgnprovider.ParseFloatString(item.Low),
			Close:       gmgnprovider.ParseFloatString(item.Close),
			Volume:      gmgnprovider.ParseFloatString(item.Volume),
			CloseTime:   closeTime,
			QuoteVolume: gmgnprovider.ParseFloatString(item.Amount),
		})
	}
	return klines, nil
}

func getGMGNTokenPrice(symbol string) (float64, error) {
	chain, address, err := gmgnprovider.ParseSymbol(symbol)
	if err != nil {
		return 0, err
	}
	client, err := getGMGNClient()
	if err != nil {
		return 0, err
	}
	info, err := client.GetTokenInfo(chain, address)
	if err != nil {
		return 0, err
	}
	if info.Price == nil {
		return 0, fmt.Errorf("gmgn price unavailable for %s", symbol)
	}
	return gmgnprovider.ParseFloatString(info.Price.Price), nil
}

func timeframeToDuration(tf string) time.Duration {
	switch tf {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return 0
	}
}
