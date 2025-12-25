package hook

import (
	"log"
)

type HookFunc func(args ...any) any

var (
	Hooks       map[string]HookFunc = map[string]HookFunc{}
	EnableHooks                     = true
)

func HookExec[T any](key string, args ...any) *T {
	if !EnableHooks {
		// Hooks are disabled, skip silently
		var zero *T
		return zero
	}
	if hook, exists := Hooks[key]; exists && hook != nil {
		log.Printf("🔌 Execute hook: %s", key)
		res := hook(args...)
		return res.(*T)
	}
	// Hook not found, skip silently (no log spam)
	var zero *T
	return zero
}

func RegisterHook(key string, hook HookFunc) {
	Hooks[key] = hook
}

// hook list
const (
	GETIP              = "GETIP"              // func (userID string) *IpResult
	NEW_BINANCE_TRADER = "NEW_BINANCE_TRADER" // func (userID string, client *futures.Client) *NewBinanceTraderResult
	NEW_ASTER_TRADER   = "NEW_ASTER_TRADER"   // func (userID string, client *http.Client) *NewAsterTraderResult
	SET_HTTP_CLIENT    = "SET_HTTP_CLIENT"    // func (client *http.Client) *SetHttpClientResult
)
