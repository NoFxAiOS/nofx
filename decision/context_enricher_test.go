package decision

import (
	"fmt"
	"testing"
)

// MockEnricher 模拟增强器，用于测试
type MockEnricher struct {
	name          string
	shouldFail    bool
	enabledFlag   bool
	callCount     int
	lastContext   *Context
}

func (me *MockEnricher) Name() string {
	return me.name
}

func (me *MockEnricher) Enrich(ctx *Context) error {
	me.callCount++
	me.lastContext = ctx
	if me.shouldFail {
		return fmt.Errorf("mock error from %s", me.name)
	}
	// 成功时添加一个扩展
	ctx.SetExtension(me.name, "enriched")
	return nil
}

func (me *MockEnricher) IsEnabled(ctx *Context) bool {
	return me.enabledFlag
}

func TestEnrichmentChainAddEnricher(t *testing.T) {
	chain := NewEnrichmentChain()

	if len(chain.enrichers) != 0 {
		t.Error("New chain should have no enrichers")
	}

	e1 := &MockEnricher{name: "test1", enabledFlag: true}
	chain.AddEnricher(e1)

	if len(chain.enrichers) != 1 {
		t.Error("Chain should have 1 enricher after Add")
	}

	// 链式调用
	e2 := &MockEnricher{name: "test2", enabledFlag: true}
	chain.AddEnricher(e2).AddEnricher(nil) // 添加nil应该被忽略

	if len(chain.enrichers) != 2 {
		t.Error("Chain should have 2 enrichers (nil should be skipped)")
	}
}

func TestEnrichmentChainExecuteAll_Success(t *testing.T) {
	chain := NewEnrichmentChain()
	ctx := &Context{
		Extensions: make(map[string]interface{}),
	}

	e1 := &MockEnricher{name: "enricher1", enabledFlag: true, shouldFail: false}
	e2 := &MockEnricher{name: "enricher2", enabledFlag: true, shouldFail: false}

	chain.AddEnricher(e1).AddEnricher(e2)

	errors := chain.ExecuteAll(ctx)

	if len(errors) != 0 {
		t.Errorf("Expected no errors, got %v", errors)
	}

	if e1.callCount != 1 {
		t.Errorf("Enricher1 should be called once, was called %d times", e1.callCount)
	}

	if e2.callCount != 1 {
		t.Errorf("Enricher2 should be called once, was called %d times", e2.callCount)
	}

	// 检查扩展是否被设置
	if _, ok := ctx.Extensions["enricher1"]; !ok {
		t.Error("Extension enricher1 should be set")
	}
	if _, ok := ctx.Extensions["enricher2"]; !ok {
		t.Error("Extension enricher2 should be set")
	}
}

func TestEnrichmentChainExecuteAll_WithFailure(t *testing.T) {
	chain := NewEnrichmentChain()
	ctx := &Context{
		Extensions: make(map[string]interface{}),
	}

	e1 := &MockEnricher{name: "enricher1", enabledFlag: true, shouldFail: false}
	e2 := &MockEnricher{name: "enricher2", enabledFlag: true, shouldFail: true} // 这个会失败
	e3 := &MockEnricher{name: "enricher3", enabledFlag: true, shouldFail: false}

	chain.AddEnricher(e1).AddEnricher(e2).AddEnricher(e3)

	errors := chain.ExecuteAll(ctx)

	// e2失败，但e3应该继续运行
	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}

	if e3.callCount != 1 {
		t.Error("Enricher3 should still be called despite e2 failure")
	}

	// e1和e3应该设置了扩展，但e2没有
	if _, ok := ctx.Extensions["enricher1"]; !ok {
		t.Error("Extension enricher1 should be set")
	}
	if _, ok := ctx.Extensions["enricher2"]; ok {
		t.Error("Extension enricher2 should NOT be set (it failed)")
	}
	if _, ok := ctx.Extensions["enricher3"]; !ok {
		t.Error("Extension enricher3 should be set")
	}
}

func TestEnrichmentChainExecuteAll_Disabled(t *testing.T) {
	chain := NewEnrichmentChain()
	ctx := &Context{
		Extensions: make(map[string]interface{}),
	}

	e1 := &MockEnricher{name: "enricher1", enabledFlag: true, shouldFail: false}
	e2 := &MockEnricher{name: "enricher2", enabledFlag: false, shouldFail: false} // 禁用
	e3 := &MockEnricher{name: "enricher3", enabledFlag: true, shouldFail: false}

	chain.AddEnricher(e1).AddEnricher(e2).AddEnricher(e3)

	errors := chain.ExecuteAll(ctx)

	if len(errors) != 0 {
		t.Error("No errors should occur")
	}

	// e2不应该被调用
	if e2.callCount != 0 {
		t.Error("Disabled enricher2 should not be called")
	}

	// e1和e3应该被调用
	if e1.callCount != 1 {
		t.Error("Enricher1 should be called")
	}
	if e3.callCount != 1 {
		t.Error("Enricher3 should be called")
	}
}

func TestEnrichmentChainExecuteAll_MultipleFailures(t *testing.T) {
	chain := NewEnrichmentChain()
	ctx := &Context{
		Extensions: make(map[string]interface{}),
	}

	e1 := &MockEnricher{name: "enricher1", enabledFlag: true, shouldFail: true}
	e2 := &MockEnricher{name: "enricher2", enabledFlag: true, shouldFail: true}
	e3 := &MockEnricher{name: "enricher3", enabledFlag: true, shouldFail: false}

	chain.AddEnricher(e1).AddEnricher(e2).AddEnricher(e3)

	errors := chain.ExecuteAll(ctx)

	// 应该有2个错误
	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}

	// 所有enrichers应该被调用（即使前面的失败）
	if e1.callCount != 1 || e2.callCount != 1 || e3.callCount != 1 {
		t.Error("All enrichers should be called despite failures")
	}

	// 只有e3应该设置扩展
	if _, ok := ctx.Extensions["enricher3"]; !ok {
		t.Error("Extension enricher3 should be set")
	}
}
