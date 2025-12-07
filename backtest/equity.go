package backtest

import (
	"math"
	"sort"

	"nofx/market"
)

// ResampleEquity resamples equity curve based on timeframe.
func ResampleEquity(points []EquityPoint, timeframe string) ([]EquityPoint, error) {
	if timeframe == "" {
		return points, nil
	}
	dur, err := market.TFDuration(timeframe)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 {
		return points, nil
	}

	durMs := dur.Milliseconds()
	if durMs <= 0 {
		return points, nil
	}

	bucketMap := make(map[int64]EquityPoint)
	bucketKeys := make([]int64, 0)
	for _, pt := range points {
		bucket := (pt.Timestamp / durMs) * durMs
		if _, exists := bucketMap[bucket]; !exists {
			bucketKeys = append(bucketKeys, bucket)
		}
		bucketPoint := pt
		bucketPoint.Timestamp = bucket
		bucketMap[bucket] = bucketPoint
	}

	sort.Slice(bucketKeys, func(i, j int) bool {
		return bucketKeys[i] < bucketKeys[j]
	})

	resampled := make([]EquityPoint, 0, len(bucketKeys))
	for _, key := range bucketKeys {
		resampled = append(resampled, bucketMap[key])
	}

	return resampled, nil
}

// LimitEquityPoints limits the number of data points within a given range (uniform sampling).
func LimitEquityPoints(points []EquityPoint, limit int) []EquityPoint {
	if limit <= 0 || len(points) <= limit {
		return points
	}

	step := float64(len(points)) / float64(limit)
	result := make([]EquityPoint, 0, limit)
	for i := 0; i < limit; i++ {
		idx := int(math.Round(step * float64(i)))
		if idx >= len(points) {
			idx = len(points) - 1
		}
		result = append(result, points[idx])
	}

	return result
}

// LimitTradeEvents applies uniform sampling to trade events.
func LimitTradeEvents(events []TradeEvent, limit int) []TradeEvent {
	if limit <= 0 || len(events) <= limit {
		return events
	}

	step := float64(len(events)) / float64(limit)
	result := make([]TradeEvent, 0, limit)
	for i := 0; i < limit; i++ {
		idx := int(math.Round(step * float64(i)))
		if idx >= len(events) {
			idx = len(events) - 1
		}
		result = append(result, events[idx])
	}
	return result
}

// AlignEquityTimestamps ensures timestamps are sorted in ascending order.
func AlignEquityTimestamps(points []EquityPoint) []EquityPoint {
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp < points[j].Timestamp
	})
	return points
}
