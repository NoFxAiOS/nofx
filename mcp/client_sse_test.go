package mcp

import (
	"errors"
	"strings"
	"testing"
)

type failingReader struct {
	payload []byte
	done    bool
}

func (r *failingReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		return copy(p, r.payload), nil
	}
	return 0, errors.New("connection reset")
}

func TestParseSSEStreamAcceptsDataWithoutSpace(t *testing.T) {
	body := "data:{\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"

	got, _, err := ParseSSEStream(strings.NewReader(body), nil, nil)
	if err != nil {
		t.Fatalf("ParseSSEStream returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("content = %q, want %q", got, "hello")
	}
}

func TestParseSSEStreamAcceptsFinalMessageContent(t *testing.T) {
	body := "data: {\"choices\":[{\"message\":{\"content\":\"final answer\"},\"finish_reason\":\"stop\"}]}\n\n"

	got, _, err := ParseSSEStream(strings.NewReader(body), nil, nil)
	if err != nil {
		t.Fatalf("ParseSSEStream returned error: %v", err)
	}
	if got != "final answer" {
		t.Fatalf("content = %q, want %q", got, "final answer")
	}
}

func TestParseSSEStreamJoinsMultiLineDataEvent(t *testing.T) {
	body := "data: {\"choices\":[\ndata: {\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"
	got, _, err := ParseSSEStream(strings.NewReader(body), nil, nil)
	if err != nil || got != "hello" {
		t.Fatalf("content=%q err=%v, want hello", got, err)
	}
}

func TestParseSSEStreamAcceptsLegacyChoiceText(t *testing.T) {
	body := "data: {\"choices\":[{\"text\":\"legacy answer\"}]}\n\ndata: [DONE]\n\n"

	got, _, err := ParseSSEStream(strings.NewReader(body), nil, nil)
	if err != nil {
		t.Fatalf("ParseSSEStream returned error: %v", err)
	}
	if got != "legacy answer" {
		t.Fatalf("content = %q, want %q", got, "legacy answer")
	}
}

func TestParseSSEStreamAllowsLargeEventLine(t *testing.T) {
	content := strings.Repeat("x", 128*1024)
	body := "data: {\"choices\":[{\"delta\":{\"content\":\"" + content + "\"}}]}\n\ndata: [DONE]\n\n"

	got, _, err := ParseSSEStream(strings.NewReader(body), nil, nil)
	if err != nil {
		t.Fatalf("ParseSSEStream returned error: %v", err)
	}
	if got != content {
		t.Fatalf("content length = %d, want %d", len(got), len(content))
	}
}

func TestDescribeSSEBodyReportsReasoningOnlyShape(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking\"}}]}\n\ndata: [DONE]\n\n")
	got := DescribeSSEBody(body)
	for _, want := range []string{"data_lines=2", "json_lines=1", "choices=1", "reasoning_chunks=1", "content_chunks=0", "done=true"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic %q does not contain %q", got, want)
		}
	}
}

func TestParseSSEStreamRejectsMalformedEventAfterContent(t *testing.T) {
	body := strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\ndata: {malformed}\n\ndata: [DONE]\n\n")
	text, _, err := ParseSSEStream(body, nil, nil)
	if err == nil {
		t.Fatal("expected malformed event to fail closed")
	}
	if text != "partial" {
		t.Fatalf("text = %q, want diagnostic partial text", text)
	}
}

func TestParseSSEStreamRejectsReaderErrorAfterContent(t *testing.T) {
	body := &failingReader{payload: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")}
	text, _, err := ParseSSEStream(body, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "stream interrupted") {
		t.Fatalf("err = %v, want stream interruption", err)
	}
	if text != "partial" {
		t.Fatalf("text = %q, want diagnostic partial text", text)
	}
}

func TestParseSSEStreamRejectsMissingCompletion(t *testing.T) {
	body := strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
	text, _, err := ParseSSEStream(body, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "completion marker") {
		t.Fatalf("err = %v, want missing completion marker", err)
	}
	if text != "partial" {
		t.Fatalf("text = %q, want diagnostic partial text", text)
	}
}

func TestParseSSEStreamCapturesUsageAfterFinishChunk(t *testing.T) {
	body := strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"done\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7,\"total_tokens\":18}}\n\ndata: [DONE]\n\n")
	text, usage, err := ParseSSEStream(body, nil, nil)
	if err != nil {
		t.Fatalf("ParseSSEStream returned error: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want done", text)
	}
	if usage == nil || usage.PromptTokens != 11 || usage.CompletionTokens != 7 || usage.TotalTokens != 18 {
		t.Fatalf("usage = %#v, want 11/7/18", usage)
	}
}

func TestParseSSEStreamRejectsReaderErrorAfterFinishChunk(t *testing.T) {
	body := &failingReader{payload: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"done\"},\"finish_reason\":\"stop\"}]}\n\n")}
	_, _, err := ParseSSEStream(body, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "stream interrupted") {
		t.Fatalf("err = %v, want stream interruption", err)
	}
}
