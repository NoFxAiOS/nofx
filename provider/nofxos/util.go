package nofxos

import (
	"fmt"
	"strings"
)

// Language represents the language for formatting output
type Language string

const (
	LangChinese Language = "zh-CN"
	LangEnglish Language = "en-US"
)

func formatPlainNumber(v float64) string {
	s := fmt.Sprintf("%.8f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

func formatSignedPlainNumber(v float64) string {
	if v > 0 {
		return "+" + formatPlainNumber(v)
	}
	return formatPlainNumber(v)
}

// formatValue formats a numeric value with sign and appropriate suffix.
// Keep the mantissa as a plain decimal with no thousands separators so AI
// prompts stay machine-readable.
func formatValue(v float64) string {
	sign := "+"
	if v < 0 {
		sign = ""
	}
	absV := v
	if absV < 0 {
		absV = -absV
	}
	if absV >= 1e9 {
		return fmt.Sprintf("%s%sB", sign, formatPlainNumber(v/1e9))
	} else if absV >= 1e6 {
		return fmt.Sprintf("%s%sM", sign, formatPlainNumber(v/1e6))
	} else if absV >= 1e3 {
		return fmt.Sprintf("%s%sK", sign, formatPlainNumber(v/1e3))
	}
	return sign + formatPlainNumber(v)
}
