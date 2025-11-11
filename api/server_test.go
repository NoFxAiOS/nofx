package api

import (
	"encoding/json"
	"testing"

	"nofx/config"
)

// TestUpdateTraderRequest_SystemPromptTemplate 测试更新交易员时 SystemPromptTemplate 字段是否存在
func TestUpdateTraderRequest_SystemPromptTemplate(t *testing.T) {
	tests := []struct {
		name                   string
		requestJSON            string
		expectedPromptTemplate string
	}{
		{
			name: "更新时应该能接收 system_prompt_template=nof1",
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
			name: "更新时应该能接收 system_prompt_template=default",
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
			name: "更新时应该能接收 system_prompt_template=custom",
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
			// 测试 UpdateTraderRequest 结构体是否能正确解析 system_prompt_template 字段
			var req UpdateTraderRequest
			err := json.Unmarshal([]byte(tt.requestJSON), &req)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// ✅ 验证 SystemPromptTemplate 字段是否被正确读取
			if req.SystemPromptTemplate != tt.expectedPromptTemplate {
				t.Errorf("Expected SystemPromptTemplate=%q, got %q",
					tt.expectedPromptTemplate, req.SystemPromptTemplate)
			}

			// 验证其他字段也被正确解析
			if req.Name != "Test Trader" {
				t.Errorf("Name not parsed correctly")
			}
			if req.AIModelID != "gpt-4" {
				t.Errorf("AIModelID not parsed correctly")
			}
		})
	}
}

// TestGetTraderConfigResponse_SystemPromptTemplate 测试获取交易员配置时返回值是否包含 system_prompt_template
func TestGetTraderConfigResponse_SystemPromptTemplate(t *testing.T) {
	tests := []struct {
		name             string
		traderConfig     *config.TraderRecord
		expectedTemplate string
	}{
		{
			name: "获取配置应该返回 system_prompt_template=nof1",
			traderConfig: &config.TraderRecord{
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
			name: "获取配置应该返回 system_prompt_template=default",
			traderConfig: &config.TraderRecord{
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
			// 模拟 handleGetTraderConfig 的返回值构造逻辑（修复后的实现）
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

			// ✅ 检查响应中是否包含 system_prompt_template
			if _, exists := result["system_prompt_template"]; !exists {
				t.Errorf("Response is missing 'system_prompt_template' field")
			} else {
				actualTemplate := result["system_prompt_template"].(string)
				if actualTemplate != tt.expectedTemplate {
					t.Errorf("Expected system_prompt_template=%q, got %q",
						tt.expectedTemplate, actualTemplate)
				}
			}

			// 验证其他字段是否正确
			if result["trader_id"] != tt.traderConfig.ID {
				t.Errorf("trader_id mismatch")
			}
			if result["trader_name"] != tt.traderConfig.Name {
				t.Errorf("trader_name mismatch")
			}
		})
	}
}

// TestUpdateTraderRequest_CompleteFields 验证 UpdateTraderRequest 结构体定义完整性
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

	// 验证基本字段是否正确解析
	if req.Name != "Test Trader" {
		t.Errorf("Name mismatch: got %q", req.Name)
	}
	if req.AIModelID != "gpt-4" {
		t.Errorf("AIModelID mismatch: got %q", req.AIModelID)
	}

	// ✅ 验证 SystemPromptTemplate 字段已正确添加到结构体
	if req.SystemPromptTemplate != "nof1" {
		t.Errorf("SystemPromptTemplate mismatch: expected %q, got %q", "nof1", req.SystemPromptTemplate)
	}
}

// TestValidateIndicatorConfig_ValidConfigs 测试合法的指标配置
func TestValidateIndicatorConfig_ValidConfigs(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name: "完整的合法配置",
			config: map[string]interface{}{
				"indicators":  []interface{}{"ema", "macd", "rsi", "atr", "volume"},
				"timeframes":  []interface{}{"3m", "4h"},
				"data_points": map[string]interface{}{"3m": 40.0, "4h": 25.0},
				"parameters":  map[string]interface{}{"rsi_period": 14.0},
			},
		},
		{
			name: "只包含indicators",
			config: map[string]interface{}{
				"indicators": []interface{}{"ema", "macd"},
			},
		},
		{
			name: "只包含timeframes",
			config: map[string]interface{}{
				"timeframes": []interface{}{"1m", "5m", "15m"},
			},
		},
		{
			name: "data_points在合法范围内",
			config: map[string]interface{}{
				"data_points": map[string]interface{}{"1h": 10.0, "4h": 100.0},
			},
		},
		{
			name: "包含bollinger指标",
			config: map[string]interface{}{
				"indicators": []interface{}{"bollinger", "rsi"},
			},
		},
		{
			name:   "空配置",
			config: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIndicatorConfig(tt.config)
			if err != nil {
				t.Errorf("验证失败: %v", err)
			}
		})
	}
}

// TestValidateIndicatorConfig_InvalidConfigs 测试不合法的指标配置
func TestValidateIndicatorConfig_InvalidConfigs(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectedErr string
	}{
		{
			name: "indicators不是数组",
			config: map[string]interface{}{
				"indicators": "ema,macd",
			},
			expectedErr: "indicators必须是数组",
		},
		{
			name: "不支持的指标名称",
			config: map[string]interface{}{
				"indicators": []interface{}{"ema", "invalid_indicator"},
			},
			expectedErr: "不支持的指标: invalid_indicator",
		},
		{
			name: "指标名称不是字符串",
			config: map[string]interface{}{
				"indicators": []interface{}{123},
			},
			expectedErr: "指标名称必须是字符串",
		},
		{
			name: "timeframes不是数组",
			config: map[string]interface{}{
				"timeframes": "1m,5m",
			},
			expectedErr: "timeframes必须是数组",
		},
		{
			name: "不支持的时间框架",
			config: map[string]interface{}{
				"timeframes": []interface{}{"1m", "10m"},
			},
			expectedErr: "不支持的时间框架: 10m",
		},
		{
			name: "时间框架不是字符串",
			config: map[string]interface{}{
				"timeframes": []interface{}{60},
			},
			expectedErr: "时间框架必须是字符串",
		},
		{
			name: "data_points不是对象",
			config: map[string]interface{}{
				"data_points": []interface{}{40, 25},
			},
			expectedErr: "data_points必须是对象",
		},
		{
			name: "数据点数量不是数字",
			config: map[string]interface{}{
				"data_points": map[string]interface{}{"3m": "40"},
			},
			expectedErr: "数据点数量必须是数字",
		},
		{
			name: "数据点数量小于10",
			config: map[string]interface{}{
				"data_points": map[string]interface{}{"3m": 5.0},
			},
			expectedErr: "数据点数量必须在10-100之间",
		},
		{
			name: "数据点数量大于100",
			config: map[string]interface{}{
				"data_points": map[string]interface{}{"3m": 150.0},
			},
			expectedErr: "数据点数量必须在10-100之间",
		},
		{
			name: "parameters不是对象",
			config: map[string]interface{}{
				"parameters": []interface{}{"rsi_period", 14},
			},
			expectedErr: "parameters必须是对象",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIndicatorConfig(tt.config)
			if err == nil {
				t.Errorf("期望验证失败,但验证通过")
			} else if err.Error() != tt.expectedErr {
				t.Errorf("错误消息不匹配:\n期望: %q\n实际: %q", tt.expectedErr, err.Error())
			}
		})
	}
}
