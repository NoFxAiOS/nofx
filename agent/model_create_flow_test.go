package agent

import (
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"nofx/store"
)

func TestHandleModelCreateSkillAsksProviderFirstWithClaw402Recommendation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-model-create.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	a := New(nil, st, DefaultConfig(), slog.Default())
	reply := a.handleModelCreateSkill("default", 42, "zh", "请帮我创建一个模型", skillSession{})

	for _, want := range []string{
		"还缺这些字段：模型提供商",
		"可选模型 provider",
		"推荐 `claw402`",
		"并列可选",
		"按次付费",
		"Base USDC 钱包支付",
		"直接创建 Base 钱包",
		"直接扫码充值/支付",
	} {
		if !strings.Contains(reply, want) {
			t.Fatalf("expected reply to contain %q, got: %s", want, reply)
		}
	}
	for _, unexpected := range []string{
		"还缺这些字段：模型提供商、API Key",
		"还缺这些字段：模型提供商、钱包私钥",
		"还缺这些字段：模型提供商、wallet private key",
	} {
		if strings.Contains(reply, unexpected) {
			t.Fatalf("provider-first reply should not ask for credentials yet: %s", reply)
		}
	}
}
