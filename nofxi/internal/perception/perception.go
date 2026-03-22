// Package perception implements the Perception Layer.
//
// Phase 2 — stubs for now.
//
// Planned capabilities:
// - Real-time market data monitoring
// - Price anomaly detection
// - Position tracking and P/L monitoring
// - News and sentiment analysis
// - Custom alert triggers
package perception

// Monitor is the market perception interface.
type Monitor interface {
	// WatchSymbol starts monitoring a symbol for price changes.
	WatchSymbol(symbol string) error

	// Stop stops all monitoring.
	Stop()
}
