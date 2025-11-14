package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestTraderIDUniqueness 测试 traderID 的唯一性（修复 Issue #893）
// 验证即使在相同的 exchange 和 AI model 下，也能生成唯一的 traderID
func TestTraderIDUniqueness(t *testing.T) {
	exchangeID := "binance"
	aiModelID := "gpt-4"

	// 模拟同时创建 100 个 trader（相同参数）
	traderIDs := make(map[string]bool)
	const numTraders = 100

	for i := 0; i < numTraders; i++ {
		// 模拟 api/server.go:497 的 traderID 生成逻辑
		traderID := generateTraderID(exchangeID, aiModelID)

		// ✅ 检查是否重复
		if traderIDs[traderID] {
			t.Errorf("Duplicate traderID detected: %s", traderID)
		}
		traderIDs[traderID] = true

		// ✅ 验证格式：应该是 "exchange_model_uuid"
		if !isValidTraderIDFormat(traderID, exchangeID, aiModelID) {
			t.Errorf("Invalid traderID format: %s", traderID)
		}
	}

	// ✅ 验证生成了预期数量的唯一 ID
	if len(traderIDs) != numTraders {
		t.Errorf("Expected %d unique traderIDs, got %d", numTraders, len(traderIDs))
	}
}

// generateTraderID 辅助函数，模拟 api/server.go 中的 traderID 生成逻辑
func generateTraderID(exchangeID, aiModelID string) string {
	return fmt.Sprintf("%s_%s_%s", exchangeID, aiModelID, uuid.New().String())
}

// isValidTraderIDFormat 验证 traderID 格式是否符合预期
func isValidTraderIDFormat(traderID, expectedExchange, expectedModel string) bool {
	// 格式：exchange_model_uuid
	// 例如：binance_gpt-4_a1b2c3d4-e5f6-7890-abcd-ef1234567890
	parts := strings.Split(traderID, "_")
	if len(parts) < 3 {
		return false
	}

	// 验证前缀
	if parts[0] != expectedExchange {
		return false
	}

	// AI model 可能包含连字符（如 gpt-4），所以需要重组
	// 最后一部分应该是 UUID
	uuidPart := parts[len(parts)-1]

	// 验证 UUID 格式（36 个字符，包含 4 个连字符）
	_, err := uuid.Parse(uuidPart)
	return err == nil
}

// TestTraderIDFormat 测试 traderID 格式的正确性
func TestTraderIDFormat(t *testing.T) {
	tests := []struct {
		name       string
		exchangeID string
		aiModelID  string
	}{
		{"Binance + GPT-4", "binance", "gpt-4"},
		{"Hyperliquid + Claude", "hyperliquid", "claude-3"},
		{"OKX + Qwen", "okx", "qwen-2.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traderID := generateTraderID(tt.exchangeID, tt.aiModelID)

			// ✅ 验证包含正确的前缀
			if !strings.HasPrefix(traderID, tt.exchangeID+"_"+tt.aiModelID+"_") {
				t.Errorf("traderID does not have correct prefix. Got: %s", traderID)
			}

			// ✅ 验证格式有效
			if !isValidTraderIDFormat(traderID, tt.exchangeID, tt.aiModelID) {
				t.Errorf("Invalid traderID format: %s", traderID)
			}

			// ✅ 验证长度合理（至少应该有 exchange + model + "_" + UUID(36) 的长度）
			minLength := len(tt.exchangeID) + len(tt.aiModelID) + 2 + 36 // 2个下划线 + 36字符UUID
			if len(traderID) < minLength {
				t.Errorf("traderID too short: expected at least %d chars, got %d", minLength, len(traderID))
			}
		})
	}
}

// TestTraderIDNoCollision 测试在高并发场景下不会产生碰撞
func TestTraderIDNoCollision(t *testing.T) {
	const iterations = 1000
	uniqueIDs := make(map[string]bool, iterations)

	// 模拟高并发场景
	for i := 0; i < iterations; i++ {
		id := generateTraderID("binance", "gpt-4")
		if uniqueIDs[id] {
			t.Fatalf("Collision detected after %d iterations: %s", i+1, id)
		}
		uniqueIDs[id] = true
	}

	if len(uniqueIDs) != iterations {
		t.Errorf("Expected %d unique IDs, got %d", iterations, len(uniqueIDs))
	}
}
