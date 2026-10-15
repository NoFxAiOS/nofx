package usstock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"
)

// ErrInsufficientBars means the selected source has fewer closed bars than
// requested. The returned Series contains all available bars from that source.
var ErrInsufficientBars = errors.New("insufficient closed bars")

var durations = map[string]time.Duration{TF15m: 15 * time.Minute, TF1h: time.Hour, TF4h: 4 * time.Hour, TF1d: 24 * time.Hour, TF1w: 7 * 24 * time.Hour}

const maxKlineRequests = 5

type barCacheEntry struct {
	bars   []Bar
	loaded time.Time
}

var bstockCache = struct {
	sync.Mutex
	entries map[string]barCacheEntry
}{entries: make(map[string]barCacheEntry)}

// GetSeries uses only closed bars from one source, preferring bStock and
// switching the whole timeframe to Yahoo when bStock is short and permitted.
func GetSeries(ctx context.Context, symbol, timeframe string, need int, allowYahoo bool) (*Series, error) {
	if _, ok := durations[timeframe]; !ok {
		return nil, fmt.Errorf("unsupported timeframe %q", timeframe)
	}
	if need <= 0 {
		return nil, fmt.Errorf("need must be positive")
	}
	info, ok := LookupSymbol(ctx, symbol)
	if !ok {
		return nil, fmt.Errorf("unknown or unavailable bStock symbol %q", symbol)
	}
	series := &Series{Symbol: symbol, Underlying: info.Underlying, Timeframe: timeframe, Source: SourceBStock}
	bars, err := bstockBars(ctx, symbol, timeframe, need)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if allowYahoo && (err != nil || len(bars) < need) {
		series.Source = SourceYahoo
		bars, err = yahooBars(ctx, info.Underlying, timeframe, need)
	}
	series.Bars = bars
	if len(bars) > need {
		series.Bars = append([]Bar(nil), bars[len(bars)-need:]...)
	}
	if len(series.Bars) < need {
		insufficient := fmt.Errorf("%w: %s %s source=%s have=%d need=%d", ErrInsufficientBars, symbol, timeframe, series.Source, len(series.Bars), need)
		if err != nil {
			return series, errors.Join(insufficient, err)
		}
		return series, insufficient
	}
	return series, err
}

func bstockBars(ctx context.Context, symbol, timeframe string, need int) ([]Bar, error) {
	key := fmt.Sprintf("%s/%s/%d", symbol, timeframe, need)
	bstockCache.Lock()
	defer bstockCache.Unlock()
	ttl := time.Minute
	if timeframe == TF1d || timeframe == TF1w {
		ttl = 30 * time.Minute
	}
	if entry, ok := bstockCache.entries[key]; ok && nowFunc().Sub(entry.loaded) < ttl {
		return append([]Bar(nil), entry.bars...), nil
	}
	now := nowFunc()
	byTime := make(map[int64]Bar)
	var endTime int64
	for page := 0; page < maxKlineRequests; page++ {
		limit := need - len(byTime) + 1 // allow for a forming bar
		if limit > 1000 {
			limit = 1000
		}
		q := url.Values{"symbol": {symbol}, "interval": {timeframe}, "limit": {strconv.Itoa(limit)}}
		if endTime != 0 {
			q.Set("endTime", strconv.FormatInt(endTime, 10))
		}
		var rows [][]json.RawMessage
		if err := fetchJSON(ctx, spotBaseURL, "/api/v3/klines", q, false, &rows); err != nil {
			return sortedBars(byTime), err
		}
		if len(rows) == 0 {
			break
		}
		earliest := int64(1<<63 - 1)
		for _, row := range rows {
			if len(row) < 6 {
				return nil, fmt.Errorf("malformed Binance kline")
			}
			var ms int64
			if err := json.Unmarshal(row[0], &ms); err != nil {
				return nil, fmt.Errorf("Binance openTime: %w", err)
			}
			if ms < earliest {
				earliest = ms
			}
			bar := Bar{OpenTime: time.UnixMilli(ms).UTC()}
			fields := []*float64{&bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume}
			for i, field := range fields {
				var value string
				if err := json.Unmarshal(row[i+1], &value); err != nil {
					return nil, fmt.Errorf("Binance kline value: %w", err)
				}
				n, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return nil, fmt.Errorf("Binance kline number: %w", err)
				}
				*field = n
			}
			if !bar.OpenTime.Add(durations[timeframe]).After(now) {
				byTime[ms] = bar
			}
		}
		if len(byTime) >= need {
			break
		}
		nextEnd := earliest - 1
		if endTime != 0 && nextEnd >= endTime {
			break
		}
		endTime = nextEnd
	}
	bars := sortedBars(byTime)
	bstockCache.entries[key] = barCacheEntry{append([]Bar(nil), bars...), nowFunc()}
	return bars, nil
}

func sortedBars(byTime map[int64]Bar) []Bar {
	bars := make([]Bar, 0, len(byTime))
	for _, bar := range byTime {
		bars = append(bars, bar)
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].OpenTime.Before(bars[j].OpenTime) })
	return bars
}

// GetUnderlyingSeries returns the underlying's Yahoo series only (regular
// session; never bStock). Used for prompt context where bStock's 24/7 thin
// off-hours bars would distort intraday candles and RSI (e.g. 4h). A short
// series is returned together with an ErrInsufficientBars error.
func GetUnderlyingSeries(ctx context.Context, symbol, timeframe string, need int) (*Series, error) {
	if _, ok := durations[timeframe]; !ok {
		return nil, fmt.Errorf("unsupported timeframe %q", timeframe)
	}
	info, ok := LookupSymbol(ctx, symbol)
	if !ok {
		return nil, fmt.Errorf("unknown or unavailable bStock symbol %q", symbol)
	}
	bars, err := yahooBars(ctx, info.Underlying, timeframe, need)
	series := &Series{Symbol: symbol, Underlying: info.Underlying, Timeframe: timeframe, Source: SourceYahoo, Bars: bars}
	if len(bars) > need {
		series.Bars = append([]Bar(nil), bars[len(bars)-need:]...)
	}
	if err != nil {
		return series, err
	}
	if len(series.Bars) < need {
		return series, fmt.Errorf("%w: %s %s source=%s have=%d need=%d", ErrInsufficientBars, symbol, timeframe, SourceYahoo, len(series.Bars), need)
	}
	return series, nil
}
