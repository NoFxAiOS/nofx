package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nofx/auth"
	"nofx/store"
)

func TestStrategyUpdateAPIMigratesLegacyProtectionValueShapeToModeValueJSON(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "strategy-legacy-shape.db")

	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create test store failed: %v", err)
	}
	defer func() {
		if st != nil {
			_ = st.Close()
		}
		_ = os.Remove(dbPath)
	}()

	auth.SetJWTSecret("test-secret-strategy-legacy-shape")
	token, err := auth.GenerateJWT("u-legacy", "legacy@example.com")
	if err != nil {
		t.Fatalf("generate jwt failed: %v", err)
	}

	srv := NewServer(nil, st, nil, 0)

	legacyConfig := `{
		"language":"zh",
		"coin_source":{"source_type":"static","static_coins":["BTCUSDT"],"use_ai500":false,"use_oi_top":false,"use_oi_low":false},
		"indicators":{"klines":{"primary_timeframe":"15m","primary_count":25,"enable_multi_timeframe":false},"enable_raw_klines":true},
		"risk_control":{"max_positions":1,"btc_eth_max_leverage":1,"altcoin_max_leverage":1,"max_margin_usage":0.9,"min_position_size":10,"min_risk_reward_ratio":2,"min_confidence":70},
		"protection":{
			"full_tp_sl":{"enabled":false,"mode":"ai","take_profit":{"enabled":false},"stop_loss":{"enabled":false}},
			"ladder_tp_sl":{"enabled":true,"mode":"ai","take_profit_enabled":false,"stop_loss_enabled":true,"rules":[{"take_profit_pct":3,"take_profit_close_ratio_pct":30,"stop_loss_pct":2,"stop_loss_close_ratio_pct":50}]},
			"drawdown_take_profit":{"enabled":false,"mode":"manual","rules":[]},
			"break_even_stop":{"enabled":false,"trigger_mode":"profit_pct","trigger_value":1,"offset_pct":0.1},
			"regime_filter":{"enabled":false,"allowed_regimes":["narrow","standard","wide"],"block_high_funding":false,"max_funding_rate_abs":0.01,"block_high_volatility":false,"max_atr14_pct":3,"require_trend_alignment":false}
		}
	}`

	strategy := &store.Strategy{
		ID:            "st-legacy",
		UserID:        "u-legacy",
		Name:          "legacy-strategy",
		Description:   "before",
		Config:        legacyConfig,
		IsPublic:      false,
		ConfigVisible: true,
	}
	if err := st.Strategy().Create(strategy); err != nil {
		t.Fatalf("create strategy failed: %v", err)
	}

	updateBody := map[string]any{
		"name":           "legacy-strategy",
		"description":    "after",
		"is_public":      false,
		"config_visible": true,
		"config": map[string]any{
			"protection": map[string]any{
				"full_tp_sl": map[string]any{
					"mode":        "ai",
					"take_profit": map[string]any{"mode": "ai", "value": 0},
					"stop_loss":   map[string]any{"mode": "ai", "value": 0},
				},
				"ladder_tp_sl": map[string]any{
					"mode":              "ai",
					"take_profit_price": map[string]any{"mode": "ai", "value": 0},
					"take_profit_size":  map[string]any{"mode": "ai", "value": 0},
				},
			},
		},
	}
	payload, err := json.Marshal(updateBody)
	if err != nil {
		t.Fatalf("marshal update payload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/strategies/st-legacy", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected PUT status 200, got %d, body=%s", w.Code, w.Body.String())
	}

	stored, err := st.Strategy().Get("u-legacy", "st-legacy")
	if err != nil {
		t.Fatalf("get updated strategy failed: %v", err)
	}

	if strings.Contains(stored.Config, `"take_profit":{"enabled"`) || strings.Contains(stored.Config, `"stop_loss":{"enabled"`) {
		t.Fatalf("expected stored config to migrate away from legacy enabled shape, got %s", stored.Config)
	}
	if !strings.Contains(stored.Config, `"take_profit":{"mode":"ai"`) {
		t.Fatalf("expected stored config to persist full take_profit mode=ai, got %s", stored.Config)
	}
	if !strings.Contains(stored.Config, `"stop_loss":{"mode":"ai"`) {
		t.Fatalf("expected stored config to persist full stop_loss mode=ai, got %s", stored.Config)
	}
	if !strings.Contains(stored.Config, `"take_profit_price":{"mode":"ai"`) {
		t.Fatalf("expected stored config to persist ladder take_profit_price mode=ai, got %s", stored.Config)
	}
}
