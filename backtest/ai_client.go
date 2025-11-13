package backtest

import (
	"fmt"
	"strings"

	"nofx/mcp"
)

func configureMCPClient(cfg BacktestConfig, base *mcp.Client) (*mcp.Client, error) {
	var client *mcp.Client
	if base != nil {
		copyClient := *base
		client = &copyClient
	} else {
		client = mcp.New()
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.AICfg.Provider))
	switch provider {
	case "", "inherit", "default":
		if cfg.AICfg.APIKey != "" {
			client.APIKey = cfg.AICfg.APIKey
		}
		if cfg.AICfg.Model != "" {
			client.Model = cfg.AICfg.Model
		}
		if cfg.AICfg.BaseURL != "" {
			client.BaseURL = cfg.AICfg.BaseURL
		}
	case "deepseek":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("deepseek provider requires api key")
		}
		client.SetDeepSeekAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
	case "qwen":
		if cfg.AICfg.APIKey == "" {
			return nil, fmt.Errorf("qwen provider requires api key")
		}
		client.SetQwenAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
	case "custom":
		if cfg.AICfg.BaseURL == "" || cfg.AICfg.APIKey == "" || cfg.AICfg.Model == "" {
			return nil, fmt.Errorf("custom provider requires base_url, api key and model")
		}
		client.SetCustomAPI(cfg.AICfg.BaseURL, cfg.AICfg.APIKey, cfg.AICfg.Model)
	default:
		return nil, fmt.Errorf("unsupported ai provider %s", cfg.AICfg.Provider)
	}

	if cfg.AICfg.Temperature > 0 {
		// no direct field, but we keep for completeness
	}

	return client, nil
}
