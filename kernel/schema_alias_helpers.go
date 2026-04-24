package kernel

import "strings"

func firstAliasFloat(primary float64, aliasMap map[string]float64, canonical string) float64 {
	if primary > 0 {
		return primary
	}
	for _, alias := range schemaAliases(canonical) {
		if v, ok := aliasMap[alias]; ok && v > 0 {
			return v
		}
	}
	return primary
}

func firstAliasString(primary string, aliasMap map[string]string, canonical string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	for _, alias := range schemaAliases(canonical) {
		if v, ok := aliasMap[alias]; ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return primary
}

func firstAliasSlice(primary []float64, aliasMap map[string][]float64, canonical string) []float64 {
	if len(primary) > 0 {
		return primary
	}
	for _, alias := range schemaAliases(canonical) {
		if v, ok := aliasMap[alias]; ok && len(v) > 0 {
			return v
		}
	}
	return primary
}
