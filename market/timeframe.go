package market

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// supportedTimeframes 定义支持的时间周期与其对应的分钟数。
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

// NormalizeTimeframe 规范化传入的时间周期字符串（大小写、不带空格），并校验是否受支持。
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

// TFDuration 返回给定周期对应的时间长度。
func TFDuration(tf string) (time.Duration, error) {
	norm, err := NormalizeTimeframe(tf)
	if err != nil {
		return 0, err
	}
	return supportedTimeframes[norm], nil
}

// MustNormalizeTimeframe 与 NormalizeTimeframe 类似，但在不受支持时 panic。
func MustNormalizeTimeframe(tf string) string {
	norm, err := NormalizeTimeframe(tf)
	if err != nil {
		panic(err)
	}
	return norm
}

// SupportedTimeframes 返回所有受支持的时间周期（已排序的切片）。
func SupportedTimeframes() []string {
	keys := make([]string, 0, len(supportedTimeframes))
	for k := range supportedTimeframes {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
