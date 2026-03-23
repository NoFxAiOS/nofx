package market

import (
	"nofx/safe"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	binanceFuturesKlinesURL = "https://fapi.binance.com/fapi/v1/klines"
	binanceMaxKlineLimit    = 1500
)

// GetKlinesRange fetches K-line series within specified time range (closed interval), returns data sorted by time in ascending order.
func GetKlinesRange(symbol string, timeframe string, start, end time.Time) ([]Kline, error) {
	symbol = Normalize(symbol)
	normTF, err := NormalizeTimeframe(timeframe)
	if err != nil {
		return nil, err
	}
	if !end.After(start) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()

	var all []Kline
	cursor := startMs

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        5,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	for cursor < endMs {
		req, err := http.NewRequest("GET", binanceFuturesKlinesURL, nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		q.Set("symbol", symbol)
		q.Set("interval", normTF)
		q.Set("limit", fmt.Sprintf("%d", binanceMaxKlineLimit))
		q.Set("startTime", fmt.Sprintf("%d", cursor))
		q.Set("endTime", fmt.Sprintf("%d", endMs))
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := safe.ReadAllLimited(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("binance klines api returned status %d: %s", resp.StatusCode, string(body))
		}

		var raw [][]interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			break
		}

		batch := make([]Kline, 0, len(raw))
		for _, item := range raw {
			if len(item) < 7 {
				continue // skip malformed entries
			}
			openTimeF, ok := item[0].(float64)
			if !ok {
				continue
			}
			closeTimeF, ok := item[6].(float64)
			if !ok {
				continue
			}
			open, _ := parseFloat(item[1])
			high, _ := parseFloat(item[2])
			low, _ := parseFloat(item[3])
			cls, _ := parseFloat(item[4])
			volume, _ := parseFloat(item[5])

			batch = append(batch, Kline{
				OpenTime:  int64(openTimeF),
				Open:      open,
				High:      high,
				Low:       low,
				Close:     cls,
				Volume:    volume,
				CloseTime: int64(closeTimeF),
			})
		}
		if len(batch) == 0 {
			break
		}

		all = append(all, batch...)

		last := batch[len(batch)-1]
		cursor = last.CloseTime + 1

		// If returned quantity is less than request limit, reached the end, can exit early.
		if len(batch) < binanceMaxKlineLimit {
			break
		}
	}

	return all, nil
}
