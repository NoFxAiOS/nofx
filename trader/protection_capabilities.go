package trader

import "strings"

// ProtectionCapabilities describes what a specific exchange adapter can reliably support
// for protection-order execution and post-open lifecycle management.
type ProtectionCapabilities struct {
	NativeStopLoss             bool
	NativeTakeProfit           bool
	NativePartialClose         bool
	NativeReduceOnly           bool
	CanAmendProtection         bool
	CanDistinguishStopTP       bool
	SupportsAlgoOrders         bool
	SupportsOCO                bool
	SupportsNativeFullTrailing bool
	SupportsNativePartialTrailing bool
}

// GetProtectionCapabilities returns a conservative capability profile for the current exchange.
// These flags are intentionally biased toward safety and will be refined per adapter in later phases.
func (at *AutoTrader) GetProtectionCapabilities() ProtectionCapabilities {
	switch strings.ToLower(at.exchange) {
	case "binance":
		return ProtectionCapabilities{true, true, true, true, true, true, true, false, true, true}
	case "okx":
		return ProtectionCapabilities{true, true, true, true, true, true, true, false, true, true}
	case "gate":
		return ProtectionCapabilities{true, true, true, true, false, true, false, false, false, false}
	case "kucoin":
		return ProtectionCapabilities{true, true, true, true, false, true, false, false, false, false}
	case "bybit":
		return ProtectionCapabilities{true, true, true, true, false, true, false, false, false, false}
	case "bitget":
		return ProtectionCapabilities{true, true, true, true, false, true, false, false, true, true}
	case "aster":
		return ProtectionCapabilities{true, true, true, true, false, true, false, false, false, false}
	case "lighter":
		return ProtectionCapabilities{true, true, false, false, false, true, false, false, false, false}
	case "hyperliquid":
		// Hyperliquid currently cannot reliably distinguish stop-loss vs take-profit cancellations.
		return ProtectionCapabilities{true, true, true, true, false, false, false, false, false, false}
	default:
		return ProtectionCapabilities{}
	}
}
