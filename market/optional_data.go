package market

import "time"

type OptionalDataState struct {
	Source     string    `json:"source"`
	Available  bool      `json:"available"`
	Reason     string    `json:"reason,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	FailClosed bool      `json:"fail_closed,omitempty"`
}

func MissingOptionalData(source, reason string) OptionalDataState {
	return OptionalDataState{Source: source, Available: false, Reason: reason, UpdatedAt: time.Now().UTC(), FailClosed: false}
}

func AvailableOptionalData(source string) OptionalDataState {
	return OptionalDataState{Source: source, Available: true, UpdatedAt: time.Now().UTC(), FailClosed: false}
}
