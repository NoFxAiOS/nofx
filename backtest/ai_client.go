package backtest

import (
	"fmt"
	"strings"

	"nofx/mcp"
)

// configureMCPClient creates/clones an MCP client based on configuration (returns mcp.AIClient interface).
// Note: mcp.New() returns an interface type; here we convert to concrete implementation before copying to avoid concurrent shared state.
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
	case "claude":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("claude provider requires api key")
		}
		cc := mcp.NewClaudeClientWithOptions()
		cc.(*mcp.ClaudeClient).SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return cc, nil
	case "kimi":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("kimi provider requires api key")
		}
		kc := mcp.NewKimiClientWithOptions()
		kc.(*mcp.KimiClient).SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return kc, nil
	case "gemini":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires api key")
		}
		gc := mcp.NewGeminiClientWithOptions()
		gc.(*mcp.GeminiClient).SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return gc, nil
	case "grok":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("grok provider requires api key")
		}
		grokC := mcp.NewGrokClientWithOptions()
		grokC.(*mcp.GrokClient).SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return grokC, nil
	case "openai":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires api key")
		}
		oaiC := mcp.NewOpenAIClientWithOptions()
		oaiC.(*mcp.OpenAIClient).SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return oaiC, nil
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

// cloneBaseClient copies the base client to avoid shared mutable state.
func cloneBaseClient(base mcp.AIClient) *mcp.Client {
	// Prefer to reuse the passed-in base client (deep copy)
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
	case *mcp.ClaudeClient:
		if c != nil && c.Client != nil {
			cp := *c.Client
			return &cp
		}
	case *mcp.KimiClient:
		if c != nil && c.Client != nil {
			cp := *c.Client
			return &cp
		}
	case *mcp.GeminiClient:
		if c != nil && c.Client != nil {
			cp := *c.Client
			return &cp
		}
	case *mcp.GrokClient:
		if c != nil && c.Client != nil {
			cp := *c.Client
			return &cp
		}
	case *mcp.OpenAIClient:
		if c != nil && c.Client != nil {
			cp := *c.Client
			return &cp
		}
	}
	// Fall back to a new default client
	return mcp.NewClient().(*mcp.Client)
}
