package ai

import (
	"database/sql"
	"fmt"
	"nofx/config"
)

// AIModelFactory AI模型工厂
// 根据配置动态创建不同的AI模型实例
// 使用Factory设计模式，符合"开闭原则" - 对扩展开放，对修改关闭
type AIModelFactory struct {
	appConfig interface{} // 通用配置接口(可以是任何类型的Config)
	db        *sql.DB

	// 缓存已创建的模型实例（可选，用于避免重复创建）
	modelCache map[string]AIModel
}

// NewAIModelFactory 创建工厂实例
// appConfig: 任何类型的配置对象(不强制要求特定的Config类型)
// db: 数据库连接,用于读取Gemini配置等(可为nil)
func NewAIModelFactory(appConfig interface{}, db *sql.DB) *AIModelFactory {
	return &AIModelFactory{
		appConfig:  appConfig,
		db:         db,
		modelCache: make(map[string]AIModel),
	}
}

// CreateUnderstandingModel 根据配置名称创建理解模型（用于Mem0）
// modelName: 模型名称 - "gemini", "gpt-4", "deepseek" 等
// 返回值: 实现AIModel接口的具体模型实例，或错误信息
//
// 降级策略：
// 如果请求的模型创建失败且配置了fallback_model，自动尝试创建备用模型
func (f *AIModelFactory) CreateUnderstandingModel(modelName string) (AIModel, error) {
	// 检查缓存
	if cached, ok := f.modelCache[modelName]; ok {
		return cached, nil
	}

	// 创建模型
	model, err := f.createModel(modelName)
	if err != nil {
		return nil, err
	}

	// 缓存模型
	f.modelCache[modelName] = model
	return model, nil
}

// CreateWithFallback 创建模型，支持自动降级
// primaryModel: 主模型名称
// fallbackModel: 备用模型名称（如果主模型创建失败）
// 返回值: 创建成功的模型实例或错误（两个都失败时返回错误）
func (f *AIModelFactory) CreateWithFallback(primaryModel, fallbackModel string) (AIModel, error) {
	// 尝试创建主模型
	model, err := f.CreateUnderstandingModel(primaryModel)
	if err == nil {
		return model, nil
	}

	// 主模型失败，尝试备用模型
	if fallbackModel != "" && fallbackModel != primaryModel {
		fallbackModel, err := f.CreateUnderstandingModel(fallbackModel)
		if err == nil {
			return fallbackModel, nil
		}
	}

	// 两个都失败
	return nil, fmt.Errorf("failed to create model: primary=%s (err: %v), fallback=%s", primaryModel, err, fallbackModel)
}

// ClearCache 清空模型缓存
func (f *AIModelFactory) ClearCache() {
	f.modelCache = make(map[string]AIModel)
}

// ClearCacheEntry 清空特定模型的缓存
func (f *AIModelFactory) ClearCacheEntry(modelName string) {
	delete(f.modelCache, modelName)
}

// 私有方法

// createModel 内部方法：根据名称创建具体的模型实例
func (f *AIModelFactory) createModel(modelName string) (AIModel, error) {
	switch modelName {
	case "gemini":
		return f.createGeminiModel()

	case "gpt-4", "gpt4":
		return f.createGPT4Model()

	case "deepseek":
		return f.createDeepSeekModel()

	case "mock":
		// 用于测试
		return NewMockAIModel(), nil

	default:
		return nil, fmt.Errorf("unsupported model: %s", modelName)
	}
}

// createGeminiModel 创建Gemini模型实例
func (f *AIModelFactory) createGeminiModel() (AIModel, error) {
	// 从数据库加载Gemini配置
	geminiCfg, err := config.LoadGeminiConfig(f.db)
	if err != nil {
		return nil, fmt.Errorf("failed to load Gemini config: %w", err)
	}

	// 创建Gemini模型
	model, err := NewGeminiModel(geminiCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini model: %w", err)
	}

	return model, nil
}

// createGPT4Model 创建GPT-4模型实例（目前使用Mock实现作为占位符）
// 在实际生产中，这里应该创建真正的GPT-4客户端
func (f *AIModelFactory) createGPT4Model() (AIModel, error) {
	// TODO: 实现真正的GPT-4模型
	// 现在返回Mock实现
	mock := NewMockAIModel()
	mock.SetResponse(`{"model": "gpt-4", "status": "mock"}`)
	return mock, nil
}

// createDeepSeekModel 创建DeepSeek模型实例（目前使用Mock实现作为占位符）
func (f *AIModelFactory) createDeepSeekModel() (AIModel, error) {
	// TODO: 实现真正的DeepSeek模型
	// 现在返回Mock实现
	mock := NewMockAIModel()
	mock.SetResponse(`{"model": "deepseek", "status": "mock"}`)
	return mock, nil
}

// SupportedModels 返回所有支持的模型名称列表
func (f *AIModelFactory) SupportedModels() []string {
	return []string{
		"gemini",
		"gpt-4",
		"gpt4",
		"deepseek",
		"mock",
	}
}

// IsModelSupported 检查模型是否被支持
func (f *AIModelFactory) IsModelSupported(modelName string) bool {
	supportedModels := f.SupportedModels()
	for _, supported := range supportedModels {
		if supported == modelName {
			return true
		}
	}
	return false
}

// GetModelInfo 不创建模型实例就获取模型信息
// 用于配置验证和监控，无需实际创建模型
func (f *AIModelFactory) GetModelInfo(modelName string) (ModelInfo, error) {
	// 检查缓存中是否已有实例
	if cached, ok := f.modelCache[modelName]; ok {
		return cached.GetModelInfo(), nil
	}

	// 创建临时实例以获取模型信息
	model, err := f.CreateUnderstandingModel(modelName)
	if err != nil {
		return ModelInfo{}, err
	}

	return model.GetModelInfo(), nil
}
