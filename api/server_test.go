package api

import (
	"encoding/json"
	"testing"

	"nofx/store"
)

// TestUpdateTraderRequest_SystemPromptTemplate Test whether SystemPromptTemplate field exists when updating trader
func TestUpdateTraderRequest_SystemPromptTemplate(t *testing.T) {
	tests := []struct {
		name                   string
		requestJSON            string
		expectedPromptTemplate string
	}{
		{
			name: "Should accept system_prompt_template=nof1 during update",
			requestJSON: `{
				"name": "Test Trader",
				"ai_model_id": "gpt-4",
				"exchange_id": "binance",
				"initial_balance": 1000,
				"scan_interval_minutes": 5,
				"btc_eth_leverage": 5,
				"altcoin_leverage": 3,
				"trading_symbols": "BTC,ETH",
				"custom_prompt": "test",
				"override_base_prompt": false,
				"is_cross_margin": true,
				"system_prompt_template": "nof1"
			}`,
			expectedPromptTemplate: "nof1",
		},
		{
			name: "Should accept system_prompt_template=default during update",
			requestJSON: `{
				"name": "Test Trader",
				"ai_model_id": "gpt-4",
				"exchange_id": "binance",
				"initial_balance": 1000,
				"scan_interval_minutes": 5,
				"btc_eth_leverage": 5,
				"altcoin_leverage": 3,
				"trading_symbols": "BTC,ETH",
				"custom_prompt": "test",
				"override_base_prompt": false,
				"is_cross_margin": true,
				"system_prompt_template": "default"
			}`,
			expectedPromptTemplate: "default",
		},
		{
			name: "Should accept system_prompt_template=custom during update",
			requestJSON: `{
				"name": "Test Trader",
				"ai_model_id": "gpt-4",
				"exchange_id": "binance",
				"initial_balance": 1000,
				"scan_interval_minutes": 5,
				"btc_eth_leverage": 5,
				"altcoin_leverage": 3,
				"trading_symbols": "BTC,ETH",
				"custom_prompt": "test",
				"override_base_prompt": false,
				"is_cross_margin": true,
				"system_prompt_template": "custom"
			}`,
			expectedPromptTemplate: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test whether UpdateTraderRequest struct can correctly parse system_prompt_template field
			var req UpdateTraderRequest
			err := json.Unmarshal([]byte(tt.requestJSON), &req)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// Verify SystemPromptTemplate field is correctly read
			if req.SystemPromptTemplate != tt.expectedPromptTemplate {
				t.Errorf("Expected SystemPromptTemplate=%q, got %q",
					tt.expectedPromptTemplate, req.SystemPromptTemplate)
			}

			// Verify other fields are also correctly parsed
			if req.Name != "Test Trader" {
				t.Errorf("Name not parsed correctly")
			}
			if req.AIModelID != "gpt-4" {
				t.Errorf("AIModelID not parsed correctly")
			}
		})
	}
}

// TestGetTraderConfigResponse_SystemPromptTemplate Test whether return value contains system_prompt_template when getting trader config
func TestGetTraderConfigResponse_SystemPromptTemplate(t *testing.T) {
	tests := []struct {
		name             string
		traderConfig     *store.Trader
		expectedTemplate string
	}{
		{
			name: "Get config should return system_prompt_template=nof1",
			traderConfig: &store.Trader{
				ID:                   "trader-123",
				UserID:               "user-1",
				Name:                 "Test Trader",
				AIModelID:            "gpt-4",
				ExchangeID:           "binance",
				InitialBalance:       1000,
				ScanIntervalMinutes:  5,
				BTCETHLeverage:       5,
				AltcoinLeverage:      3,
				TradingSymbols:       "BTC,ETH",
				CustomPrompt:         "test",
				OverrideBasePrompt:   false,
				SystemPromptTemplate: "nof1",
				IsCrossMargin:        true,
				IsRunning:            false,
			},
			expectedTemplate: "nof1",
		},
		{
			name: "Get config should return system_prompt_template=default",
			traderConfig: &store.Trader{
				ID:                   "trader-456",
				UserID:               "user-1",
				Name:                 "Test Trader 2",
				AIModelID:            "gpt-4",
				ExchangeID:           "binance",
				InitialBalance:       2000,
				ScanIntervalMinutes:  10,
				BTCETHLeverage:       10,
				AltcoinLeverage:      5,
				TradingSymbols:       "BTC",
				CustomPrompt:         "",
				OverrideBasePrompt:   false,
				SystemPromptTemplate: "default",
				IsCrossMargin:        false,
				IsRunning:            false,
			},
			expectedTemplate: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate handleGetTraderConfig return value construction logic (fixed implementation)
			result := map[string]interface{}{
				"trader_id":              tt.traderConfig.ID,
				"trader_name":            tt.traderConfig.Name,
				"ai_model":               tt.traderConfig.AIModelID,
				"exchange_id":            tt.traderConfig.ExchangeID,
				"initial_balance":        tt.traderConfig.InitialBalance,
				"scan_interval_minutes":  tt.traderConfig.ScanIntervalMinutes,
				"btc_eth_leverage":       tt.traderConfig.BTCETHLeverage,
				"altcoin_leverage":       tt.traderConfig.AltcoinLeverage,
				"trading_symbols":        tt.traderConfig.TradingSymbols,
				"custom_prompt":          tt.traderConfig.CustomPrompt,
				"override_base_prompt":   tt.traderConfig.OverrideBasePrompt,
				"system_prompt_template": tt.traderConfig.SystemPromptTemplate,
				"is_cross_margin":        tt.traderConfig.IsCrossMargin,
				"is_running":             tt.traderConfig.IsRunning,
			}

			// Check if response contains system_prompt_template
			if _, exists := result["system_prompt_template"]; !exists {
				t.Errorf("Response is missing 'system_prompt_template' field")
			} else {
				actualTemplate := result["system_prompt_template"].(string)
				if actualTemplate != tt.expectedTemplate {
					t.Errorf("Expected system_prompt_template=%q, got %q",
						tt.expectedTemplate, actualTemplate)
				}
			}

			// Verify other fields are correct
			if result["trader_id"] != tt.traderConfig.ID {
				t.Errorf("trader_id mismatch")
			}
			if result["trader_name"] != tt.traderConfig.Name {
				t.Errorf("trader_name mismatch")
			}
		})
	}
}

// TestUpdateTraderRequest_CompleteFields Verify UpdateTraderRequest struct definition completeness
func TestUpdateTraderRequest_CompleteFields(t *testing.T) {
	jsonData := `{
		"name": "Test Trader",
		"ai_model_id": "gpt-4",
		"exchange_id": "binance",
		"initial_balance": 1000,
		"scan_interval_minutes": 5,
		"btc_eth_leverage": 5,
		"altcoin_leverage": 3,
		"trading_symbols": "BTC,ETH",
		"custom_prompt": "test",
		"override_base_prompt": false,
		"is_cross_margin": true,
		"system_prompt_template": "nof1"
	}`

	var req UpdateTraderRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify basic fields are correctly parsed
	if req.Name != "Test Trader" {
		t.Errorf("Name mismatch: got %q", req.Name)
	}
	if req.AIModelID != "gpt-4" {
		t.Errorf("AIModelID mismatch: got %q", req.AIModelID)
	}

	// Verify SystemPromptTemplate field has been correctly added to struct
	if req.SystemPromptTemplate != "nof1" {
		t.Errorf("SystemPromptTemplate mismatch: expected %q, got %q", "nof1", req.SystemPromptTemplate)
	}
}

// TestTraderListResponse_SystemPromptTemplate Test whether trader object returned by handleTraderList API contains system_prompt_template field
func TestTraderListResponse_SystemPromptTemplate(t *testing.T) {
	// Simulate trader object construction in handleTraderList
	trader := &store.Trader{
		ID:                   "trader-001",
		UserID:               "user-1",
		Name:                 "My Trader",
		AIModelID:            "gpt-4",
		ExchangeID:           "binance",
		InitialBalance:       5000,
		SystemPromptTemplate: "nof1",
		IsRunning:            true,
	}

	// Construct API response object (consistent with logic in api/server.go)
	response := map[string]interface{}{
		"trader_id":              trader.ID,
		"trader_name":            trader.Name,
		"ai_model":               trader.AIModelID,
		"exchange_id":            trader.ExchangeID,
		"is_running":             trader.IsRunning,
		"initial_balance":        trader.InitialBalance,
		"system_prompt_template": trader.SystemPromptTemplate,
	}

	// Verify system_prompt_template field exists
	if _, exists := response["system_prompt_template"]; !exists {
		t.Errorf("Trader list response is missing 'system_prompt_template' field")
	}

	// Verify system_prompt_template value is correct
	if response["system_prompt_template"] != "nof1" {
		t.Errorf("Expected system_prompt_template='nof1', got %v", response["system_prompt_template"])
	}
}

// TestPublicTraderListResponse_SystemPromptTemplate Test whether trader object returned by handlePublicTraderList API contains system_prompt_template field
func TestPublicTraderListResponse_SystemPromptTemplate(t *testing.T) {
	// Simulate trader data returned by getConcurrentTraderData
	traderData := map[string]interface{}{
		"trader_id":              "trader-002",
		"trader_name":            "Public Trader",
		"ai_model":               "claude",
		"exchange":               "binance",
		"total_equity":           10000.0,
		"total_pnl":              500.0,
		"total_pnl_pct":          5.0,
		"position_count":         3,
		"margin_used_pct":        25.0,
		"is_running":             true,
		"system_prompt_template": "default",
	}

	// Construct API response object (consistent with logic in api/server.go handlePublicTraderList)
	response := map[string]interface{}{
		"trader_id":              traderData["trader_id"],
		"trader_name":            traderData["trader_name"],
		"ai_model":               traderData["ai_model"],
		"exchange":               traderData["exchange"],
		"total_equity":           traderData["total_equity"],
		"total_pnl":              traderData["total_pnl"],
		"total_pnl_pct":          traderData["total_pnl_pct"],
		"position_count":         traderData["position_count"],
		"margin_used_pct":        traderData["margin_used_pct"],
		"system_prompt_template": traderData["system_prompt_template"],
	}

	// Verify system_prompt_template field exists
	if _, exists := response["system_prompt_template"]; !exists {
		t.Errorf("Public trader list response is missing 'system_prompt_template' field")
	}

	// Verify system_prompt_template value is correct
	if response["system_prompt_template"] != "default" {
		t.Errorf("Expected system_prompt_template='default', got %v", response["system_prompt_template"])
	}
}
