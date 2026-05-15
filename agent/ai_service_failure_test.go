package agent

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestAIServiceFailureHighlightsHTMLGatewayResponse(t *testing.T) {
	a := New(nil, nil, DefaultConfig(), slog.Default())

	msg, err := a.aiServiceFailure("zh", errors.New("fail to parse AI server response: failed to parse response: invalid character '<' looking for beginning of value"))
	if err != nil {
		t.Fatalf("aiServiceFailure returned error: %v", err)
	}

	for _, want := range []string{
		"当前 AI 服务调用失败",
		"上游返回了 HTML 页面或网关/反代错误页",
		"custom_api_url",
		"不是“未配置模型”",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected message to contain %q, got: %s", want, msg)
		}
	}
	if strings.Contains(msg, "更可能是模型服务余额不足、接口报错或超时") {
		t.Fatalf("html parse error should not use the generic balance/timeout-only guidance: %s", msg)
	}
}

func TestAIServiceFailureHighlightsUpstreamEmptyOutputRateLimit(t *testing.T) {
	a := New(nil, nil, DefaultConfig(), slog.Default())

	msg, err := a.aiServiceFailure("zh", errors.New(`API returned error (status 429): {"error":{"code":"upstream_empty_output","message":"Upstream model returned empty output.","param":null,"type":"rate_limit_error"}}`))
	if err != nil {
		t.Fatalf("aiServiceFailure returned error: %v", err)
	}

	for _, want := range []string{
		"当前 AI 服务调用失败",
		"上游模型没有返回有效内容",
		"不应优先归因成“余额不足”",
		"切换到另一个可用模型",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected message to contain %q, got: %s", want, msg)
		}
	}
	if strings.Contains(msg, "更可能是模型服务余额不足、接口报错、鉴权失败或超时") {
		t.Fatalf("upstream empty output should not use the generic balance/auth/timeout guidance: %s", msg)
	}
}

func TestAIServiceFailureHighlightsBannedAccountAuthFailure(t *testing.T) {
	a := New(nil, nil, DefaultConfig(), slog.Default())

	msg, err := a.aiServiceFailure("zh", errors.New(`API returned error (status 401): {"error":{"code":"authentication_failed","message":"login failed: USER_IS_BANNED","param":null,"type":"authentication_error"}}`))
	if err != nil {
		t.Fatalf("aiServiceFailure returned error: %v", err)
	}

	for _, want := range []string{
		"当前 AI 服务调用失败",
		"账号被禁用/封禁",
		"USER_IS_BANNED",
		"换一个可用账号/API Key",
		"切换到另一个已启用模型",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected message to contain %q, got: %s", want, msg)
		}
	}
	for _, unexpected := range []string{"余额不足", "超时"} {
		if strings.Contains(msg, unexpected) {
			t.Fatalf("banned account auth failure should not mention %q: %s", unexpected, msg)
		}
	}
}

func TestCompletedPlanFallbackDoesNotExposeFinalSummaryFailure(t *testing.T) {
	msg := formatCompletedPlanFallback("zh", []PlanStep{
		{
			Type:   planStepTypeTool,
			Status: planStepStatusCompleted,
			Title:  "创建名为 eeg 的策略",
		},
	})
	if msg == "" {
		t.Fatalf("expected fallback message")
	}
	for _, bad := range []string{"失败", "AI", "稍后"} {
		if strings.Contains(msg, bad) {
			t.Fatalf("fallback should not expose final summary failure %q: %s", bad, msg)
		}
	}
	if !strings.Contains(msg, "已完成") || !strings.Contains(msg, "创建名为 eeg 的策略") {
		t.Fatalf("fallback should summarize completed work, got: %s", msg)
	}
}

func TestDeterministicCompletedPlanResponseSkipsLLMForSimpleConfirmation(t *testing.T) {
	state := ExecutionState{
		Steps: []PlanStep{
			{
				ID:     "create_strategy",
				Type:   planStepTypeTool,
				Status: planStepStatusCompleted,
				Title:  "创建名为 eeg 的策略",
			},
			{
				ID:          "respond",
				Type:        planStepTypeRespond,
				Status:      planStepStatusRunning,
				Title:       "策略创建成功",
				Instruction: "确认策略创建成功",
			},
		},
	}
	msg := deterministicCompletedPlanResponse("zh", state, state.Steps[1])
	if msg == "" {
		t.Fatalf("expected deterministic response")
	}
	if !strings.Contains(msg, "已完成") || !strings.Contains(msg, "创建名为 eeg 的策略") {
		t.Fatalf("unexpected deterministic response: %s", msg)
	}
}
