package okx

import "testing"

func TestParseOKXAlgoOrderResponseSuccess(t *testing.T) {
	algoID, err := parseOKXAlgoOrderResponse([]byte(`[{"algoId":"123","sCode":"0","sMsg":""}]`), "stop loss")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if algoID != "123" {
		t.Fatalf("expected algo id 123, got %q", algoID)
	}
}

func TestParseOKXAlgoOrderResponseRejectsBusinessFailure(t *testing.T) {
	_, err := parseOKXAlgoOrderResponse([]byte(`[{"algoId":"","sCode":"51280","sMsg":"trigger price invalid"}]`), "stop loss")
	if err == nil {
		t.Fatal("expected rejection error")
	}
	if got := err.Error(); got != "OKX stop loss rejected: code=51280 msg=trigger price invalid" {
		t.Fatalf("unexpected error: %s", got)
	}
}
