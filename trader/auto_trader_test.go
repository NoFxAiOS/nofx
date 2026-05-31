package trader

import (
	"testing"
	"time"

	"nofx/mcp"
	"nofx/store"
)

func TestNewAutoTrader_ForcesStreamForCustomOpenAIBaseURL(t *testing.T) {
	at, err := NewAutoTrader(AutoTraderConfig{
		ID:              "test-custom-openai-stream",
		Name:            "test custom openai stream",
		AIModel:         mcp.ProviderOpenAI,
		CustomAPIKey:    "sk-test-key",
		CustomAPIURL:    "https://gateway.example/v1",
		CustomModelName: "gpt-5.5",
		Exchange:        "gate",
		InitialBalance:  100,
		ScanInterval:    time.Minute,
		StrategyConfig:  &store.StrategyConfig{},
	}, nil, "test-user")
	if err != nil {
		t.Fatalf("NewAutoTrader() error = %v", err)
	}

	embedder, ok := at.mcpClient.(mcp.ClientEmbedder)
	if !ok {
		t.Fatalf("mcp client %T does not expose base client", at.mcpClient)
	}
	if !embedder.BaseClient().ForceStream {
		t.Fatal("expected custom OpenAI-compatible client to force streaming")
	}
}

func TestNewAutoTrader_DoesNotForceStreamForDefaultOpenAI(t *testing.T) {
	at, err := NewAutoTrader(AutoTraderConfig{
		ID:              "test-openai-no-stream",
		Name:            "test openai no stream",
		AIModel:         mcp.ProviderOpenAI,
		CustomAPIKey:    "sk-test-key",
		CustomModelName: "gpt-5.4",
		Exchange:        "gate",
		InitialBalance:  100,
		ScanInterval:    time.Minute,
		StrategyConfig:  &store.StrategyConfig{},
	}, nil, "test-user")
	if err != nil {
		t.Fatalf("NewAutoTrader() error = %v", err)
	}

	embedder, ok := at.mcpClient.(mcp.ClientEmbedder)
	if !ok {
		t.Fatalf("mcp client %T does not expose base client", at.mcpClient)
	}
	if embedder.BaseClient().ForceStream {
		t.Fatal("expected default OpenAI client not to force streaming")
	}
}

func TestAIClientOptions_ForceStreamForCustomOpenAIBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		aiModel   string
		customURL string
		want      bool
	}{
		{name: "custom openai base url", aiModel: mcp.ProviderOpenAI, customURL: "https://gateway.example/v1", want: true},
		{name: "non openai", aiModel: mcp.ProviderDeepSeek, customURL: "https://gateway.example/v1", want: false},
		{name: "default openai", aiModel: mcp.ProviderOpenAI, customURL: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := aiClientOptions(tt.aiModel, tt.customURL)
			client := mcp.NewClient(opts...).(*mcp.Client)
			if client.ForceStream != tt.want {
				t.Fatalf("ForceStream = %v, want %v", client.ForceStream, tt.want)
			}
		})
	}
}
