package backtest

import (
	"fmt"
	"strings"

	"nofx/mcp"
)

// configureMCPClient 根据配置创建/克隆 MCP 客户端（返回 mcp.AIClient 接口）。
// 说明：mcp.New() 返回接口类型，这里统一转为具体实现再做拷贝，避免并发共享状态。
func configureMCPClient(cfg BacktestConfig, base mcp.AIClient) (mcp.AIClient, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.AICfg.Provider))

	// DeepSeek
	if provider == "" || provider == "inherit" || provider == "default" {
		client := cloneBaseClient(base)
		if cfg.AICfg.APIKey != "" || cfg.AICfg.BaseURL != "" || cfg.AICfg.Model != "" {
			client.SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		}
		return client, nil
	}

	switch provider {
	case "deepseek":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("deepseek provider requires api key")
		}
		ds := mcp.NewDeepSeekClientWithOptions()
		ds.(*mcp.DeepSeekClient).SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return ds, nil
	case "qwen":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("qwen provider requires api key")
		}
		qc := mcp.NewQwenClientWithOptions()
		qc.(*mcp.QwenClient).SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return qc, nil
	case "custom":
		if cfg.AICfg.BaseURL == "" || cfg.AICfg.APIKey == "" || cfg.AICfg.Model == "" {
			return nil, fmt.Errorf("custom provider requires base_url, api key and model")
		}
		client := cloneBaseClient(base)
		client.SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported ai provider %s", cfg.AICfg.Provider)
	}
}

// cloneBaseClient 复制基础客户端以避免共享可变状态。
func cloneBaseClient(base mcp.AIClient) *mcp.Client {
	// 优先尝试复用传入的基础客户端（深拷贝）
	switch c := base.(type) {
	case *mcp.Client:
		cp := *c
		return &cp
	case *mcp.DeepSeekClient:
		if c != nil && c.Client != nil {
			cp := *c.Client
			return &cp
		}
	case *mcp.QwenClient:
		if c != nil && c.Client != nil {
			cp := *c.Client
			return &cp
		}
	}
	// 回退到新的默认客户端
	return mcp.NewClient().(*mcp.Client)
}
