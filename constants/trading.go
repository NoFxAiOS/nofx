package constants

import "time"

const (
	// Critical API settings (fixes the core issue)
	DefaultAPITimeout  = 120 * time.Second
	DefaultTemperature = 0.5
	DefaultMaxTokens   = 4000 // Increased from 2000 to fix incomplete responses
)
