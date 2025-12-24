package mem0

import (
	"context"
	"fmt"
	"nofx/ai"
	"testing"
)

// TestMem0WithAIModelFactory 测试Mem0与AI模型工厂的集成
func TestMem0WithAIModelFactory(t *testing.T) {
	// 模拟配置
	cfg := &Config{
		Enabled: true,
		APIKey:  "test-key",
		APIURL:  "https://api.mem0.ai/v1",
		UserID:  "test-user",
		OrgID:   "test-org",

		// AI模型配置（新增）
		UnderstandingModel: "mock",      // 使用Mock模型进行测试
		FallbackModel:      "mock",      // 备用模型

		// 其他配置
		CacheTTLMinutes: 30,
		MetricsEnabled:  true,
	}

	// 创建AI模型工厂
	factory := ai.NewAIModelFactory(nil, nil)

	// 获取理解模型
	model, err := factory.CreateUnderstandingModel(cfg.UnderstandingModel)
	if err != nil {
		t.Fatalf("failed to create understanding model: %v", err)
	}

	if model == nil {
		t.Fatal("expected model instance")
	}

	// 验证模型可用
	ctx := context.Background()
	modelInfo := model.GetModelInfo()

	if modelInfo.Name == "" {
		t.Fatal("expected model name")
	}

	// 验证健康检查
	err = model.Health(ctx)
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}

	// 验证API调用
	response, err := model.CallAPI(ctx, "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	if response == "" {
		t.Fatal("expected non-empty response")
	}

	t.Logf("✅ Integration test passed")
	t.Logf("   Model: %s", modelInfo.Name)
	t.Logf("   Response: %s", response)
}

// TestMem0FallbackModel 测试Mem0的模型降级机制
func TestMem0FallbackModel(t *testing.T) {
	factory := ai.NewAIModelFactory(nil, nil)

	// 测试从不支持的模型降级到备用模型
	model, err := factory.CreateWithFallback("unsupported-model", "mock")

	if err != nil {
		t.Fatalf("expected fallback to work, got error: %v", err)
	}

	if model == nil {
		t.Fatal("expected model from fallback")
	}

	// 验证是Mock模型（备用模型）
	info := model.GetModelInfo()
	if info.Name != "mock-model" {
		t.Fatalf("expected mock model, got %s", info.Name)
	}

	t.Logf("✅ Fallback test passed - successfully fell back to mock model")
}

// TestMem0ConfigIntegration 测试Mem0配置与AI模型的集成
func TestMem0ConfigIntegration(t *testing.T) {
	// 创建模拟Config
	cfg := &Config{
		Enabled:            true,
		UnderstandingModel: "mock",
		FallbackModel:      "mock",
	}

	// 验证配置值
	if cfg.UnderstandingModel != "mock" {
		t.Fatalf("expected understanding model 'mock', got %s", cfg.UnderstandingModel)
	}

	if cfg.FallbackModel != "mock" {
		t.Fatalf("expected fallback model 'mock', got %s", cfg.FallbackModel)
	}

	// 创建工厂并验证可以创建指定的模型
	factory := ai.NewAIModelFactory(nil, nil)
	model, err := factory.CreateUnderstandingModel(cfg.UnderstandingModel)

	if err != nil {
		t.Fatalf("failed to create configured model: %v", err)
	}

	if model == nil {
		t.Fatal("expected model")
	}

	t.Logf("✅ Configuration integration test passed")
	t.Logf("   Understanding Model: %s", cfg.UnderstandingModel)
	t.Logf("   Fallback Model: %s", cfg.FallbackModel)
}

// BenchmarkMem0ModelCreation Mem0模型创建的性能基准
func BenchmarkMem0ModelCreation(b *testing.B) {
	factory := ai.NewAIModelFactory(nil, nil)
	cfg := &Config{
		UnderstandingModel: "mock",
		FallbackModel:      "mock",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		factory.CreateUnderstandingModel(cfg.UnderstandingModel)
	}
}

// BenchmarkMem0ModelWithFallback Mem0带降级的模型创建的性能基准
func BenchmarkMem0ModelWithFallback(b *testing.B) {
	factory := ai.NewAIModelFactory(nil, nil)
	cfg := &Config{
		UnderstandingModel: "mock",
		FallbackModel:      "mock",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		factory.CreateWithFallback(cfg.UnderstandingModel, cfg.FallbackModel)
	}
}

// TestGetFullDecisionV2WithAIModel 测试GetFullDecisionV2与AI模型的集成
func TestGetFullDecisionV2WithAIModel(t *testing.T) {
	// 1. 创建Mem0配置(使用mock模型)
	cfg := &Config{
		Enabled:            true,
		UnderstandingModel: "mock", // 使用Mock模型进行测试
		FallbackModel:      "mock",
		CacheTTLMinutes:    30,
	}

	// 2. 创建所有必要的组件
	store := &MockMemoryStore{}
	compressor := NewContextCompressor(700)
	kb := NewGlobalKnowledgeBase(store)
	raf := NewRiskAwareFormatter()
	sm := NewStageManager()
	warmer := NewCacheWarmer(store, 0, 0) // 禁用预热以加速测试

	// 3. 创建GetFullDecisionV2并注入AI模型
	gfd, err := NewGetFullDecisionV2(store, compressor, kb, raf, sm, warmer, cfg)
	if err != nil {
		t.Fatalf("❌ Failed to create GetFullDecisionV2 with AI model: %v", err)
	}

	if gfd == nil {
		t.Fatal("❌ GetFullDecisionV2 instance is nil")
	}

	// 4. 验证模型已正确注入
	if gfd.model == nil {
		t.Fatal("❌ AI model not injected into GetFullDecisionV2")
	}

	modelInfo := gfd.model.GetModelInfo()
	if modelInfo.Name != "mock-model" {
		t.Errorf("❌ Expected model name 'mock-model', got %s", modelInfo.Name)
	}

	// 5. 测试决策生成流程
	ctx := context.Background()
	query := Query{
		Type:  "semantic_search",
		Limit: 5,
	}

	decision, err := gfd.GenerateDecision(ctx, query)
	if err != nil {
		t.Fatalf("❌ Failed to generate decision: %v", err)
	}

	// 6. 验证决策结果
	if decision.Model != cfg.UnderstandingModel {
		t.Errorf("❌ Expected model %s in decision, got %s", cfg.UnderstandingModel, decision.Model)
	}

	if decision.Recommendation == "" {
		t.Fatal("❌ Decision recommendation should not be empty")
	}

	if decision.Confidence < 0 || decision.Confidence > 1 {
		t.Errorf("❌ Confidence should be between 0 and 1, got %.2f", decision.Confidence)
	}

	t.Logf("✅ GetFullDecisionV2 with AI model integration test passed")
	t.Logf("   Model: %s", decision.Model)
	t.Logf("   Confidence: %.2f", decision.Confidence)
	t.Logf("   Recommendation preview: %.0s", decision.Recommendation)
}

// TestGetFullDecisionV2ModelFallback 测试GetFullDecisionV2的模型降级机制
func TestGetFullDecisionV2ModelFallback(t *testing.T) {
	// 1. 创建配置:主模型失败,降级到mock
	cfg := &Config{
		Enabled:            true,
		UnderstandingModel: "unsupported-model", // 不支持的模型
		FallbackModel:      "mock",               // 降级到mock
	}

	// 2. 创建GetFullDecisionV2
	store := &MockMemoryStore{}
	compressor := NewContextCompressor(700)
	kb := NewGlobalKnowledgeBase(store)
	raf := NewRiskAwareFormatter()
	sm := NewStageManager()
	warmer := NewCacheWarmer(store, 0, 0)

	gfd, err := NewGetFullDecisionV2(store, compressor, kb, raf, sm, warmer, cfg)
	if err != nil {
		t.Fatalf("❌ Failed to create GetFullDecisionV2: %v", err)
	}

	// 3. 验证降级到了mock模型
	modelInfo := gfd.model.GetModelInfo()
	if modelInfo.Name != "mock-model" {
		t.Errorf("❌ Should fallback to mock model, got %s", modelInfo.Name)
	}

	// 4. 验证决策仍然可以正常生成
	ctx := context.Background()
	query := Query{Type: "semantic_search", Limit: 5}

	decision, err := gfd.GenerateDecision(ctx, query)
	if err != nil {
		t.Fatalf("❌ Failed to generate decision after fallback: %v", err)
	}

	if decision.Recommendation == "" {
		t.Fatal("❌ Fallback model should still generate recommendations")
	}

	t.Logf("✅ Model fallback test passed")
	t.Logf("   Primary model: %s (unsupported)", cfg.UnderstandingModel)
	t.Logf("   Fallback model: %s (used)", cfg.FallbackModel)
}

// TestGetFullDecisionV2RuntimeFallback 测试当AI模型运行时失败时的降级逻辑
func TestGetFullDecisionV2RuntimeFallback(t *testing.T) {
	// 1. 设置正常配置
	cfg := &Config{
		Enabled:            true,
		UnderstandingModel: "mock",
		FallbackModel:      "mock",
	}

	// 2. 创建带数据的MockStore
	store := &MockMemoryStore{}
	store.SearchOverride = []Memory{
		{
			ID:           "mem-1",
			Content:      "Historical data suggests buying in this condition",
			QualityScore: 0.9,
			Type:         "decision",
		},
	}

	compressor := NewContextCompressor(700)
	kb := NewGlobalKnowledgeBase(store)
	raf := NewRiskAwareFormatter()
	sm := NewStageManager()
	warmer := NewCacheWarmer(store, 0, 0)

	gfd, _ := NewGetFullDecisionV2(store, compressor, kb, raf, sm, warmer, cfg)

	// 3. 注入错误到Mock模型
	// 需要先转换类型
	if mockModel, ok := gfd.model.(*ai.MockAIModel); ok {
		mockModel.SetError(fmt.Errorf("simulated API timeout"))
	} else {
		t.Fatal("❌ Expected gfd.model to be *ai.MockAIModel")
	}

	// 4. 生成决策
	ctx := context.Background()
	query := Query{Type: "semantic_search", Limit: 5}

	decision, err := gfd.GenerateDecision(ctx, query)
	if err != nil {
		t.Fatalf("❌ GenerateDecision should not fail even if AI model fails: %v", err)
	}

	// 5. 验证是否使用了记忆回退
	if decision.Recommendation != store.SearchOverride[0].Content {
		t.Errorf("❌ Expected recommendation to be the top memory content, got: %s", decision.Recommendation)
	}

	// 6. 验证置信度是否降低
	expectedConfidence := store.SearchOverride[0].QualityScore * 0.8
	if decision.Confidence != expectedConfidence {
		t.Errorf("❌ Expected degraded confidence %.2f, got %.2f", expectedConfidence, decision.Confidence)
	}

	t.Logf("✅ Runtime fallback test passed")
	t.Logf("   Reason: AI model failed with timeout")
	t.Logf("   Result: Fallback to top memory content")
}

// BenchmarkGetFullDecisionV2WithAIModel 测试GetFullDecisionV2决策生成的性能
func BenchmarkGetFullDecisionV2WithAIModel(b *testing.B) {
	cfg := &Config{
		Enabled:            true,
		UnderstandingModel: "mock",
		FallbackModel:      "mock",
	}

	store := &MockMemoryStore{}
	compressor := NewContextCompressor(700)
	kb := NewGlobalKnowledgeBase(store)
	raf := NewRiskAwareFormatter()
	sm := NewStageManager()
	warmer := NewCacheWarmer(store, 0, 0)

	gfd, _ := NewGetFullDecisionV2(store, compressor, kb, raf, sm, warmer, cfg)

	ctx := context.Background()
	query := Query{Type: "semantic_search", Limit: 5}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gfd.GenerateDecision(ctx, query)
	}
}