package trader

import (
	"fmt"
)

// SafeFloat64 安全地從 map 中提取 float64 值
func SafeFloat64(m map[string]interface{}, key string) (float64, error) {
	v, exists := m[key]
	if !exists {
		return 0, fmt.Errorf("key '%s' not found", key)
	}

	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		// 某些 API 可能返回字符串格式的數字
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		if err != nil {
			return 0, fmt.Errorf("cannot convert key '%s' value '%v' to float64: %w", key, val, err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("key '%s' has unexpected type %T", key, v)
	}
}

// SafeString 安全地從 map 中提取 string 值
func SafeString(m map[string]interface{}, key string) (string, error) {
	v, exists := m[key]
	if !exists {
		return "", fmt.Errorf("key '%s' not found", key)
	}

	switch val := v.(type) {
	case string:
		return val, nil
	case fmt.Stringer:
		return val.String(), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// SafeInt 安全地從 map 中提取 int 值
func SafeInt(m map[string]interface{}, key string) (int, error) {
	v, exists := m[key]
	if !exists {
		return 0, fmt.Errorf("key '%s' not found", key)
	}

	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	default:
		return 0, fmt.Errorf("key '%s' has unexpected type %T", key, v)
	}
}

// SafeFloat64OrDefault 安全地提取 float64，失敗時返回默認值
func SafeFloat64OrDefault(m map[string]interface{}, key string, defaultValue float64) float64 {
	val, err := SafeFloat64(m, key)
	if err != nil {
		return defaultValue
	}
	return val
}

// SafeStringOrDefault 安全地提取 string，失敗時返回默認值
func SafeStringOrDefault(m map[string]interface{}, key string, defaultValue string) string {
	val, err := SafeString(m, key)
	if err != nil {
		return defaultValue
	}
	return val
}

// SafeIntOrDefault 安全地提取 int，失敗時返回默認值
func SafeIntOrDefault(m map[string]interface{}, key string, defaultValue int) int {
	val, err := SafeInt(m, key)
	if err != nil {
		return defaultValue
	}
	return val
}
