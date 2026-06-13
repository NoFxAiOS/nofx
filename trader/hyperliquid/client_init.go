package hyperliquid

import (
	"fmt"

	hl "github.com/sonirico/go-hyperliquid"
)

// initExchangeClient runs the SDK exchange constructor and converts its
// panic-on-failure behavior into an error. go-hyperliquid's NewExchange
// auto-fetches meta/spotMeta/perpDexs when they are passed as nil and panics
// if any of those API calls fail (NewInfo: panic(err)), so a transient
// Hyperliquid API hiccup would otherwise crash the calling HTTP handler
// with a 500.
func initExchangeClient(build func() *hl.Exchange) (ex *hl.Exchange, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("hyperliquid client initialization failed: %v", r)
		}
	}()
	return build(), nil
}
