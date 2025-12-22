package news

import (
	"fmt"
	"testing"
	"time"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// 初始状态：closed
	if cb.State() != "closed" {
		t.Errorf("Initial state should be closed, got %s", cb.State())
	}

	// 模拟3次失败
	for i := 0; i < 3; i++ {
		err := cb.Call(func() error {
			return fmt.Errorf("fail %d", i)
		})
		if err == nil {
			t.Errorf("Expected error on call %d", i)
		}
	}

	// 3次失败后应该是 open
	if cb.State() != "open" {
		t.Errorf("After 3 failures, state should be open, got %s", cb.State())
	}

	// 尝试调用应该快速失败
	start := time.Now()
	err := cb.Call(func() error {
		return nil
	})
	elapsed := time.Since(start)

	if err != ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}

	// 快速失败：应该在1ms内返回
	if elapsed > 10*time.Millisecond {
		t.Errorf("Circuit open should fail fast, took %v", elapsed)
	}

	// 等待冷却期
	time.Sleep(150 * time.Millisecond)

	// 现在应该进入 half-open，成功调用应该恢复
	for i := 0; i < 2; i++ {
		err := cb.Call(func() error {
			return nil
		})
		if err != nil {
			t.Errorf("Half-open call %d failed: %v", i, err)
		}
	}

	// 2次成功后应该是 closed
	if cb.State() != "closed" {
		t.Errorf("After recovery, state should be closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_FastFail(t *testing.T) {
	cb := NewCircuitBreaker(1, 100*time.Millisecond)

	// 1次失败就打开
	cb.Call(func() error { return fmt.Errorf("fail") })

	if cb.State() != "open" {
		t.Fatal("Circuit should be open")
	}

	// 100次并发调用应该都快速失败
	done := make(chan bool, 100)
	start := time.Now()

	for i := 0; i < 100; i++ {
		go func() {
			cb.Call(func() error { time.Sleep(100 * time.Millisecond); return nil })
			done <- true
		}()
	}

	// 等待所有调用完成
	for i := 0; i < 100; i++ {
		<-done
	}

	elapsed := time.Since(start)

	// 100个调用应该在100ms内完成（如果是快速失败）
	// 如果每个调用都等待100ms，会超过1秒
	if elapsed > 200*time.Millisecond {
		t.Errorf("Fast fail not working: took %v for 100 calls", elapsed)
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)
	successThreshold := 2

	// 2次失败打开
	cb.Call(func() error { return fmt.Errorf("fail") })
	cb.Call(func() error { return fmt.Errorf("fail") })

	if cb.State() != "open" {
		t.Fatal("Circuit should be open")
	}

	// 等待冷却期
	time.Sleep(150 * time.Millisecond)

	// 第1次成功：进入half-open
	err := cb.Call(func() error { return nil })
	if err != nil {
		t.Errorf("First recovery call failed: %v", err)
	}

	if cb.State() != "half-open" {
		t.Errorf("Should be half-open, got %s", cb.State())
	}

	// 第2次成功：恢复到closed
	for i := 1; i < successThreshold; i++ {
		cb.Call(func() error { return nil })
	}

	if cb.State() != "closed" {
		t.Errorf("After %d successes, should be closed, got %s", successThreshold, cb.State())
	}
}

func TestCircuitBreaker_ResetInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	// 2次失败打开
	cb.Call(func() error { return fmt.Errorf("fail") })
	cb.Call(func() error { return fmt.Errorf("fail") })

	// 等待冷却期进入half-open
	time.Sleep(150 * time.Millisecond)
	cb.Call(func() error { return nil })

	if cb.State() != "half-open" {
		t.Fatal("Should be half-open")
	}

	// 一次失败应该回到open
	cb.Call(func() error { return fmt.Errorf("fail during recovery") })

	if cb.State() != "open" {
		t.Errorf("Failure in half-open should go back to open, got %s", cb.State())
	}
}

func TestCircuitBreaker_GetMetrics(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	metrics := cb.GetMetrics()
	if metrics["state"] != "closed" {
		t.Errorf("Initial state metric should be closed")
	}
	if metrics["state_value"] != 0 {
		t.Errorf("Closed state value should be 0")
	}

	// 失败3次
	for i := 0; i < 3; i++ {
		cb.Call(func() error { return fmt.Errorf("fail") })
	}

	metrics = cb.GetMetrics()
	if metrics["state"] != "open" {
		t.Errorf("Open state metric should be open")
	}
	if metrics["state_value"] != 1 {
		t.Errorf("Open state value should be 1")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(1, 1*time.Second)

	// 打开断路器
	cb.Call(func() error { return fmt.Errorf("fail") })

	if cb.State() != "open" {
		t.Fatal("Should be open")
	}

	// 手动重置
	cb.Reset()

	if cb.State() != "closed" {
		t.Errorf("After reset, should be closed, got %s", cb.State())
	}

	// 应该可以正常调用
	err := cb.Call(func() error { return nil })
	if err != nil {
		t.Errorf("Call after reset failed: %v", err)
	}
}
