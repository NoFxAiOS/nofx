package agent

import (
	"log/slog"
	"path/filepath"
	"testing"

	"nofx/store"
)

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
