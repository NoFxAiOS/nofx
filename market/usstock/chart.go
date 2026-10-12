package usstock

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// ChartKline is one Binance spot candle for charting. Unlike bstockBars it
// includes the forming (unclosed) bar.
type ChartKline struct {
	OpenTime    int64 // unix ms
	CloseTime   int64 // unix ms
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      float64
	QuoteVolume float64
}

var chartIntervals = map[string]bool{
	"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
	"1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true,
	"1d": true, "3d": true, "1w": true, "1M": true,
}

// ChartKlines fetches the latest `limit` spot klines (ascending, forming bar
// included) in a single uncached request. limit <= 0 means 500; max 1000.
func ChartKlines(ctx context.Context, symbol, interval string, limit int) ([]ChartKline, error) {
	if !chartIntervals[interval] {
		return nil, fmt.Errorf("unsupported Binance spot interval: %s", interval)
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}
	q := url.Values{"symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(limit)}}
	var rows [][]json.RawMessage
	if err := fetchJSON(ctx, spotBaseURL, "/api/v3/klines", q, false, &rows); err != nil {
		return nil, err
	}
	out := make([]ChartKline, 0, len(rows))
	for _, row := range rows {
		k, err := parseChartRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

func parseChartRow(row []json.RawMessage) (ChartKline, error) {
	var k ChartKline
	if len(row) < 8 {
		return k, fmt.Errorf("malformed Binance kline")
	}
	if err := json.Unmarshal(row[0], &k.OpenTime); err != nil {
		return k, fmt.Errorf("Binance openTime: %w", err)
	}
	if err := json.Unmarshal(row[6], &k.CloseTime); err != nil {
		return k, fmt.Errorf("Binance closeTime: %w", err)
	}
	fields := []struct {
		idx int
		dst *float64
	}{{1, &k.Open}, {2, &k.High}, {3, &k.Low}, {4, &k.Close}, {5, &k.Volume}, {7, &k.QuoteVolume}}
	for _, f := range fields {
		var s string
		if err := json.Unmarshal(row[f.idx], &s); err != nil {
			return k, fmt.Errorf("Binance kline value: %w", err)
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return k, fmt.Errorf("Binance kline number: %w", err)
		}
		*f.dst = n
	}
	return k, nil
}
