package agent

import (
	"log/slog"
	"path/filepath"
	"testing"

	"nofx/mcp"
	"nofx/store"
)

type staticClientEmbedder struct {
	client *mcp.Client
}

func (s staticClientEmbedder) BaseClient() *mcp.Client { return s.client }

func TestLoadAIClientFromStoreUserPrefersModelWithBalance(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-model-selection.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if err := st.AIModel().UpdateWithName("default", "default_openai", "OpenAI", true, "sk-test", "", "gpt-5.2"); err != nil {
		t.Fatalf("create openai model: %v", err)
	}
	if err := st.AIModel().UpdateWithName("default", "wallet_claw402", "Claw402", true, "0x205d759b80bae1afa31a36c4afaeec0b10378c1c55e3363bcde5a1db75c747ca", "", "glm-5"); err != nil {
		t.Fatalf("create claw402 model: %v", err)
	}

	restoreWalletAddress := agentWalletAddressFromPrivateKey
	restoreBalanceQuery := agentQueryUSDCBalanceCached
	t.Cleanup(func() {
		agentWalletAddressFromPrivateKey = restoreWalletAddress
		agentQueryUSDCBalanceCached = restoreBalanceQuery
	})

	agentWalletAddressFromPrivateKey = func(privateKey string) (string, error) {
		if privateKey == "0x205d759b80bae1afa31a36c4afaeec0b10378c1c55e3363bcde5a1db75c747ca" {
			return "0xabc", nil
		}
		return "", nil
	}
	agentQueryUSDCBalanceCached = func(address string) (float64, error) {
		if address == "0xabc" {
			return 12.5, nil
		}
		return 0, nil
	}

	a := New(nil, st, DefaultConfig(), slog.Default())
	_, modelName, ok := a.loadAIClientFromStoreUser("default")
	if !ok {
		t.Fatalf("expected model selection to succeed")
	}
	if modelName != "glm-5" {
		t.Fatalf("expected model with wallet balance to be selected, got %q", modelName)
	}
}

func TestAgentNeedsOpenAIForceStreamForCustomOpenAIBaseURL(t *testing.T) {
	if !agentNeedsOpenAIForceStream(mcp.ProviderOpenAI, "https://gateway.example/v1") {
		t.Fatalf("expected OpenAI-compatible custom base URL to force stream")
	}
	if agentNeedsOpenAIForceStream(mcp.ProviderOpenAI, "") {
		t.Fatalf("expected default OpenAI base URL not to force stream")
	}
	if agentNeedsOpenAIForceStream(mcp.ProviderDeepSeek, "https://gateway.example/v1") {
		t.Fatalf("expected non-OpenAI provider not to force stream")
	}
}

func TestLoadAIClientFromStoreUserForcesStreamForCustomOpenAIBaseURL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-force-stream.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := st.AIModel().UpdateWithName("default", "custom_openai", "Custom OpenAI", true, "sk-test", "https://gateway.example/v1", "gpt-5.5"); err != nil {
		t.Fatalf("create model: %v", err)
	}

	a := New(nil, st, DefaultConfig(), slog.Default())
	client, _, ok := a.loadAIClientFromStoreUser("default")
	if !ok {
		t.Fatalf("expected model selection to succeed")
	}
	embedder, ok := client.(mcp.ClientEmbedder)
	if !ok {
		if baseClient, direct := client.(*mcp.Client); direct {
			embedder = staticClientEmbedder{client: baseClient}
		} else {
			t.Fatalf("expected mcp client embedder, got %T", client)
		}
	}
	base := embedder.BaseClient()
	if !base.ForceStream {
		t.Fatalf("expected ForceStream on agent-created mcp client")
	}
	body := base.BuildRequestBodyFromRequest(&mcp.Request{Messages: []mcp.Message{mcp.NewUserMessage("hi")}})
	if got, ok := body["stream"].(bool); !ok || !got {
		t.Fatalf("expected request body stream=true, got %#v", body["stream"])
	}
}
