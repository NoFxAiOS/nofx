package hyperliquid

import (
	"errors"
	"strings"
	"testing"

	hl "github.com/sonirico/go-hyperliquid"
)

func TestInitExchangeClientConvertsPanicToError(t *testing.T) {
	// The SDK constructor panics when its automatic meta fetch fails
	// (go-hyperliquid info.go NewInfo: panic(err)). The wrapper must turn
	// that into an error instead of crashing the HTTP handler.
	_, err := initExchangeClient(func() *hl.Exchange {
		panic(errors.New("failed to fetch meta: API error 0: "))
	})
	if err == nil {
		t.Fatal("expected error when the SDK constructor panics, got nil")
	}
	if !strings.Contains(err.Error(), "failed to fetch meta") {
		t.Errorf("error should carry the panic cause, got %q", err.Error())
	}
}

func TestInitExchangeClientPassesThroughSuccess(t *testing.T) {
	want := &hl.Exchange{}
	got, err := initExchangeClient(func() *hl.Exchange {
		return want
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatal("wrapper must return the constructed exchange unchanged")
	}
}
