package utils

import (
	"encoding/json"
	"fmt"
	"log"
)

// LogSuccess logs a successful operation
func LogSuccess(operation string) {
	log.Printf("✓ %s成功", operation)
}

// LogError logs a failed operation
func LogError(operation string, err error) {
	log.Printf("❌ %s失败: %v", operation, err)
}

// LogWarning logs a warning message
func LogWarning(operation, message string) {
	log.Printf("⚠️ %s警告: %s", operation, message)
}

// LogInfo logs an informational message
func LogInfo(message string) {
	log.Printf("🔄 %s", message)
}

// LogDebug logs debug information with data
func LogDebug(operation string, data interface{}) {
	if jsonData, err := json.MarshalIndent(data, "  ", "  "); err == nil {
		log.Printf("🔍 [DEBUG] %s:\n%s", operation, string(jsonData))
	} else {
		log.Printf("🔍 [DEBUG] %s: %+v", operation, data)
	}
}

// UnmarshalJSON unmarshals JSON with standardized error handling
func UnmarshalJSON[T any](data []byte, result *T, operation string) error {
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("解析%s失败: %w", operation, err)
	}
	return nil
}

// MarshalJSON marshals to JSON with standardized error handling
func MarshalJSON(data interface{}, operation string) ([]byte, error) {
	result, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("序列化%s失败: %w", operation, err)
	}
	return result, nil
}
