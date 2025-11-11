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
		log.Printf("🔌 Hooks are disabled, skip hook: %s", key)
		var zero *T
		return zero
	}
	if hook, exists := Hooks[key]; exists && hook != nil {
		log.Printf("🔌 Execute hook: %s", key)
		res := hook(args...)
		// 🔒 安全的类型断言，防止 panic
		if typedRes, ok := res.(*T); ok {
			return typedRes
		}
		log.Printf("⚠️ Hook %s returned wrong type, expected *%T but got %T", key, new(T), res)
	} else {
		log.Printf("🔌 Do not find hook: %s", key)
	}
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
