// Package memory implements the Memory Layer.
//
// Provides persistent storage for:
// - Trade history and P/L tracking
// - Conversation context (per-user message history)
// - User preferences and risk profiles
// - Strategy performance metrics
// - Market pattern snapshots
//
// Backend: SQLite (pure Go driver, zero CGO)
package memory
