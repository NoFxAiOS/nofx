package market

import "fmt"

// ValidateKlineInterval 验证K线间隔（公开函数，供外部调用）
func ValidateKlineInterval(interval string) error {
	if !IsValidKlineInterval(interval) {
		return fmt.Errorf("无效的K线间隔 '%s'，支持的间隔: 1s, 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M", interval)
	}
	return nil
}
