package trader

import (
	"strings"

	"nofx/market"
)

func recordOptionalDataState(states map[string]market.OptionalDataState, source string, available bool, reason string) map[string]market.OptionalDataState {
	if states == nil {
		states = make(map[string]market.OptionalDataState)
	}
	key := strings.TrimSpace(source)
	if key == "" {
		return states
	}
	if available {
		states[key] = market.AvailableOptionalData(key)
	} else {
		states[key] = market.MissingOptionalData(key, reason)
	}
	return states
}
