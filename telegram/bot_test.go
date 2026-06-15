package telegram

import (
	"testing"

	"nofx/mcp"
)

func TestClientForProvider_ForcesStreamForCustomOpenAIBaseURL(t *testing.T) {
	client := clientForProvider("openai", "https://example.com/v1")
	embedder, ok := client.(mcp.ClientEmbedder)
	if !ok {
		t.Fatalf("client %T does not expose base client", client)
	}
	if !embedder.BaseClient().ForceStream {
		t.Fatal("expected Telegram OpenAI-compatible client with custom base URL to force streaming")
	}
}

func TestClientForProvider_DoesNotForceStreamForDefaultOpenAI(t *testing.T) {
	client := clientForProvider("openai", "")
	embedder, ok := client.(mcp.ClientEmbedder)
	if !ok {
		t.Fatalf("client %T does not expose base client", client)
	}
	if embedder.BaseClient().ForceStream {
		t.Fatal("expected Telegram default OpenAI client not to force streaming")
	}
}
