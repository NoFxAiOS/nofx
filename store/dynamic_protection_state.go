package store

import (
	"encoding/json"
	"fmt"
	"time"
)

const DynamicProtectionStateConfigKey = "dynamic_protection_state_v1"

type DynamicProtectionRecord struct {
	Key                 string  `json:"key"`
	TraderID            string  `json:"trader_id"`
	ExchangeID          string  `json:"exchange_id"`
	Symbol              string  `json:"symbol"`
	Side                string  `json:"side"`
	PositionFingerprint string  `json:"position_fingerprint"`
	ProtectionType      string  `json:"protection_type"`
	RuleFingerprint     string  `json:"rule_fingerprint"`
	CloseRatioPct       float64 `json:"close_ratio_pct"`
	Status              string  `json:"status"`
	ExchangeOrderID     string  `json:"exchange_order_id,omitempty"`
	ActivationPrice     float64 `json:"activation_price,omitempty"`
	TriggerPrice        float64 `json:"trigger_price,omitempty"`
	CallbackRatio       float64 `json:"callback_ratio,omitempty"`
	Quantity            float64 `json:"quantity,omitempty"`
	UpdatedAt           int64   `json:"updated_at"`
}

type DynamicProtectionState struct {
	Records map[string]DynamicProtectionRecord `json:"records"`
}

func BuildDynamicProtectionKey(traderID, exchangeID, symbol, side, positionFingerprint, protectionType, ruleFingerprint string, closeRatioPct float64) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%.4f", traderID, exchangeID, symbol, side, positionFingerprint, protectionType, ruleFingerprint, closeRatioPct)
}

func (s *Store) LoadDynamicProtectionState() (*DynamicProtectionState, error) {
	raw, err := s.GetSystemConfig(DynamicProtectionStateConfigKey)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return &DynamicProtectionState{Records: map[string]DynamicProtectionRecord{}}, nil
	}
	var state DynamicProtectionState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("failed to decode dynamic protection state: %w", err)
	}
	if state.Records == nil {
		state.Records = map[string]DynamicProtectionRecord{}
	}
	return &state, nil
}

func (s *Store) SaveDynamicProtectionRecord(record DynamicProtectionRecord) error {
	state, err := s.LoadDynamicProtectionState()
	if err != nil {
		return err
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = time.Now().UTC().UnixMilli()
	}
	if record.Key == "" {
		record.Key = BuildDynamicProtectionKey(record.TraderID, record.ExchangeID, record.Symbol, record.Side, record.PositionFingerprint, record.ProtectionType, record.RuleFingerprint, record.CloseRatioPct)
	}
	for key, existing := range state.Records {
		if key == record.Key {
			continue
		}
		if existing.TraderID != record.TraderID || existing.ExchangeID != record.ExchangeID || existing.Symbol != record.Symbol || existing.Side != record.Side || existing.PositionFingerprint != record.PositionFingerprint || existing.ProtectionType != record.ProtectionType || existing.Status != "armed" {
			continue
		}
		// Native trailing ownership is singleton per active position/protection type.
		// When a newer arm succeeds, older persisted owners must not be restored on restart.
		if record.Status == "armed" && (record.ProtectionType == "native_trailing" || record.ProtectionType == "native_partial_trailing") {
			existing.Status = "replaced"
			existing.UpdatedAt = record.UpdatedAt
			state.Records[key] = existing
		}
	}
	state.Records[record.Key] = record
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to encode dynamic protection state: %w", err)
	}
	return s.SetSystemConfig(DynamicProtectionStateConfigKey, string(data))
}

func (s *Store) DeleteDynamicProtectionRecordsForInactive(activeKeys map[string]struct{}) error {
	state, err := s.LoadDynamicProtectionState()
	if err != nil {
		return err
	}
	changed := false
	for key, record := range state.Records {
		positionKey := record.Symbol + "_" + record.Side
		if _, ok := activeKeys[positionKey]; ok {
			continue
		}
		delete(state.Records, key)
		changed = true
	}
	if !changed {
		return nil
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to encode dynamic protection state: %w", err)
	}
	return s.SetSystemConfig(DynamicProtectionStateConfigKey, string(data))
}
