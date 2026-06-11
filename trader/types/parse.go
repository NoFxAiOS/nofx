package types

import (
	"fmt"
	"strconv"
)

// ParseFloatField parses a numeric string field from an exchange API response.
// An empty string is treated as zero, since exchanges commonly omit fields
// that have no value (e.g. unrealized PnL with no open position). A non-empty
// value that fails to parse returns an error naming the field, so a malformed
// API response surfaces instead of silently becoming a zero balance.
func ParseFloatField(field, value string) (float64, error) {
	if value == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s value %q: %w", field, value, err)
	}
	return v, nil
}
