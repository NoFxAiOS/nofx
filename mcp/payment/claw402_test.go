package payment

import (
	"testing"
)

func TestClaw402GPT6Endpoint(t *testing.T) {
	client := NewClaw402ClientWithOptions().(*Claw402Client)
	client.SetAPIKey(claw402TestPrivateKey, "", "gpt-6")

	if got, want := client.resolveEndpoint(), "/api/v1/ai/openai/chat/6"; got != want {
		t.Fatalf("resolveEndpoint() = %q, want %q", got, want)
	}
	if got, want := client.BaseURL, DefaultClaw402URL+"/api/v1/ai/openai/chat/6"; got != want {
		t.Fatalf("BaseURL = %q, want %q", got, want)
	}

	body := client.BuildMCPRequestBody("system", "user")
	if _, ok := body["temperature"]; ok {
		t.Fatal("GPT-6 request must omit temperature")
	}
	if _, ok := body["max_tokens"]; ok {
		t.Fatal("Claw402 request must defer max_tokens to the gateway")
	}
	if _, ok := body["max_completion_tokens"]; ok {
		t.Fatal("Claw402 request must defer max_completion_tokens to the gateway")
	}
}
