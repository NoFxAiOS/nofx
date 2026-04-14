package gmgn

import (
	"fmt"
	"strings"
)

// IsChainSymbol reports whether symbol is a GMGN chain:token identifier.
func IsChainSymbol(symbol string) bool {
	_, _, err := ParseSymbol(symbol)
	return err == nil
}

// ParseSymbol parses a GMGN symbol in the format chain:token_address.
func ParseSymbol(symbol string) (string, string, error) {
	raw := strings.TrimSpace(symbol)
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("gmgn symbol must be chain:token_address")
	}
	chain := NormalizeChain(parts[0])
	if _, ok := GetChainConfig(chain); !ok {
		return "", "", fmt.Errorf("unsupported gmgn chain: %s", parts[0])
	}
	address := strings.TrimSpace(parts[1])
	if address == "" {
		return "", "", fmt.Errorf("gmgn token address is required")
	}
	return chain, address, nil
}

// FormatSymbol formats a GMGN chain+token pair into the canonical symbol form.
func FormatSymbol(chain, tokenAddress string) string {
	return NormalizeChain(chain) + ":" + strings.TrimSpace(tokenAddress)
}
