package manager

import (
	"testing"
)

// TestRemoveTrader 测试从内存中移除trader
func TestRemoveTrader(t *testing.T) {
	tm := NewTraderManager()

	// 创建一个模拟的 trader 并添加到 map
	traderID := "test-trader-123"
	tm.traders[traderID] = nil // 使用 nil 作为占位符，实际测试中只需验证删除逻辑

	// 验证 trader 存在
	if _, exists := tm.traders[traderID]; !exists {
		t.Fatal("trader 应该存在于 map 中")
	}

	// 调用 RemoveTrader
	tm.RemoveTrader(traderID)

	// 验证 trader 已被移除
	if _, exists := tm.traders[traderID]; exists {
		t.Error("trader 应该已从 map 中移除")
	}
}

// TestRemoveTrader_NonExistent 测试移除不存在的trader不会报错
func TestRemoveTrader_NonExistent(t *testing.T) {
	tm := NewTraderManager()

	// 尝试移除不存在的 trader，不应该 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("移除不存在的 trader 不应该 panic: %v", r)
		}
	}()

	tm.RemoveTrader("non-existent-trader")
}

// TestRemoveTrader_Concurrent 测试并发移除trader的安全性
func TestRemoveTrader_Concurrent(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-concurrent"

	// 添加 trader
	tm.traders[traderID] = nil

	// 并发调用 RemoveTrader
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			tm.RemoveTrader(traderID)
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证 trader 已被移除
	if _, exists := tm.traders[traderID]; exists {
		t.Error("trader 应该已从 map 中移除")
	}
}

// TestGetTrader_AfterRemove 测试移除后获取trader返回错误
func TestGetTrader_AfterRemove(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-get"

	// 添加 trader
	tm.traders[traderID] = nil

	// 移除 trader
	tm.RemoveTrader(traderID)

	// 尝试获取已移除的 trader
	_, err := tm.GetTrader(traderID)
	if err == nil {
		t.Error("获取已移除的 trader 应该返回错误")
	}
}
