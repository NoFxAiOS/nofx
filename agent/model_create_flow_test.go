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

func TestHandleModelCreateSkillAcceptsBareClaw402PrivateKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-model-create-claw402.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	a := New(nil, st, DefaultConfig(), slog.Default())
	session := skillSession{
		Name:   "model_management",
		Action: "create",
		Phase:  "collecting",
		Fields: map[string]string{
			"provider":       "claw402",
			"name":           "Claw402 (Base USDC)",
			"custom_model_name": "deepseek",
		},
	}

	reply := a.handleModelCreateSkill("default", 42, "zh", "0x205d759b80bae1afa31a36c4afaeec0b10378c1c55e3363bcde5a1db75c747ca", session)

	if strings.Contains(reply, "还缺这些字段：钱包私钥") {
		t.Fatalf("expected bare private key to be accepted, got: %s", reply)
	}
	if !strings.Contains(reply, "我先整理了一份模型配置草稿") {
		t.Fatalf("expected draft summary after accepting private key, got: %s", reply)
	}
}
