package market

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// supportedTimeframes defines supported timeframes and their corresponding durations.
var supportedTimeframes = map[string]time.Duration{
	"1m":  time.Minute,
	"3m":  3 * time.Minute,
	"5m":  5 * time.Minute,
	"15m": 15 * time.Minute,
	"30m": 30 * time.Minute,
	"1h":  time.Hour,
	"2h":  2 * time.Hour,
	"4h":  4 * time.Hour,
	"6h":  6 * time.Hour,
	"12h": 12 * time.Hour,
	"1d":  24 * time.Hour,
}

// NormalizeTimeframe normalizes the incoming timeframe string (case-insensitive, no spaces), and validates if it's supported.
func NormalizeTimeframe(tf string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(tf))
	if trimmed == "" {
		return "", fmt.Errorf("timeframe cannot be empty")
	}
	if _, ok := supportedTimeframes[trimmed]; !ok {
		return "", fmt.Errorf("unsupported timeframe '%s'", tf)
	}
	return trimmed, nil
}

// TFDuration returns the time duration corresponding to the given timeframe.
func TFDuration(tf string) (time.Duration, error) {
	norm, err := NormalizeTimeframe(tf)
	if err != nil {
		return 0, err
	}
	return supportedTimeframes[norm], nil
}

// MustNormalizeTimeframe is similar to NormalizeTimeframe, but panics when unsupported.
func MustNormalizeTimeframe(tf string) string {
	norm, err := NormalizeTimeframe(tf)
	if err != nil {
		panic(err)
	}
	return norm
}

// SupportedTimeframes returns all supported timeframes (sorted slice).
func SupportedTimeframes() []string {
	keys := make([]string, 0, len(supportedTimeframes))
	for k := range supportedTimeframes {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
