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
