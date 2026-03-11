package backtest

import (
	"fmt"
	"strings"

	"nofx/mcp"
	_ "nofx/mcp/payment"
	_ "nofx/mcp/provider"
)

// configureMCPClient creates/clones an MCP client based on configuration (returns mcp.AIClient interface).
// Note: mcp.New() returns an interface type; here we convert to concrete implementation before copying to avoid concurrent shared state.
func configureMCPClient(cfg BacktestConfig, base mcp.AIClient) (mcp.AIClient, error) {
	providerName := strings.ToLower(strings.TrimSpace(cfg.AICfg.Provider))

	// Inherit base client
	if providerName == "" || providerName == "inherit" || providerName == "default" {
		client := cloneBaseClient(base)
		if cfg.AICfg.APIKey != "" || cfg.AICfg.BaseURL != "" || cfg.AICfg.Model != "" {
			client.SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		}
		return client, nil
	}

	// Custom provider uses cloned base
	if providerName == "custom" {
		if cfg.AICfg.BaseURL == "" || cfg.AICfg.APIKey == "" || cfg.AICfg.Model == "" {
			return nil, fmt.Errorf("custom provider requires base_url, api key and model")
		}
		client := cloneBaseClient(base)
		client.SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
		return client, nil
	}

	// Create client via registry
	client := mcp.NewAIClientByProvider(providerName)
	if client == nil {
		return nil, fmt.Errorf("unsupported ai provider %s", cfg.AICfg.Provider)
	}

	if cfg.AICfg.APIKey == "" {
		return nil, fmt.Errorf("%s provider requires api key", providerName)
	}

	// Payment providers ignore custom URL
	switch providerName {
	case "blockrun-base", "blockrun-sol", "claw402":
		client.SetAPIKey(cfg.AICfg.APIKey, "", cfg.AICfg.Model)
	default:
		client.SetAPIKey(cfg.AICfg.APIKey, cfg.AICfg.BaseURL, cfg.AICfg.Model)
	}
	return client, nil
}

// cloneBaseClient copies the base client to avoid shared mutable state.
// Uses the ClientEmbedder interface to extract the underlying *mcp.Client
// from any provider type that embeds it.
func cloneBaseClient(base mcp.AIClient) *mcp.Client {
	if embedder, ok := base.(mcp.ClientEmbedder); ok {
		if inner := embedder.BaseClient(); inner != nil {
			cp := *inner
			return &cp
		}
	}
	if c, ok := base.(*mcp.Client); ok {
		cp := *c
		return &cp
	}
	// Fall back to a new default client
	return mcp.NewClient().(*mcp.Client)
}
