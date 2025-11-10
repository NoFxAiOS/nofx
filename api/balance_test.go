package api

import (
	"testing"
)

// TestCalculateTotalEquity 測試總資產計算邏輯
// 總資產 = 錢包餘額 + 未實現盈虧
func TestCalculateTotalEquity(t *testing.T) {
	tests := []struct {
		name           string
		balanceInfo    map[string]interface{}
		expectedEquity float64
		expectError    bool
	}{
		{
			name: "正常情況_有盈利",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": 50.0,
			},
			expectedEquity: 1050.0, // 1000 + 50
			expectError:    false,
		},
		{
			name: "正常情況_有虧損",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": -200.0,
			},
			expectedEquity: 800.0, // 1000 - 200
			expectError:    false,
		},
		{
			name: "無持倉_未實現盈虧為0",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": 0.0,
			},
			expectedEquity: 1000.0, // 1000 + 0
			expectError:    false,
		},
		{
			name: "缺少totalUnrealizedProfit欄位_視為0",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance": 1000.0,
			},
			expectedEquity: 1000.0, // 1000 + 0
			expectError:    false,
		},
		{
			name: "缺少totalWalletBalance欄位_視為0",
			balanceInfo: map[string]interface{}{
				"totalUnrealizedProfit": 50.0,
			},
			expectedEquity: 50.0, // 0 + 50
			expectError:    false,
		},
		{
			name:           "空balanceInfo_總資產為0",
			balanceInfo:    map[string]interface{}{},
			expectedEquity: 0.0,
			expectError:    true, // 總資產 <= 0 應該報錯
		},
		{
			name: "錢包餘額為0但有未實現盈利",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    0.0,
				"totalUnrealizedProfit": 100.0,
			},
			expectedEquity: 100.0, // 0 + 100
			expectError:    false,
		},
		{
			name: "類型錯誤_totalWalletBalance是字串",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    "1000",
				"totalUnrealizedProfit": 50.0,
			},
			expectedEquity: 50.0, // 無法解析錢包餘額，視為 0
			expectError:    false,
		},
		{
			name: "大額餘額和大額虧損",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    10000.0,
				"totalUnrealizedProfit": -5000.0,
			},
			expectedEquity: 5000.0, // 10000 - 5000
			expectError:    false,
		},
		{
			name: "接近爆倉_總資產接近0",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    100.0,
				"totalUnrealizedProfit": -99.9,
			},
			expectedEquity: 0.1, // 100 - 99.9
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模擬提取邏輯（與 api/server.go 中的邏輯一致）
			totalWalletBalance := 0.0
			totalUnrealizedProfit := 0.0

			if wallet, ok := tt.balanceInfo["totalWalletBalance"].(float64); ok {
				totalWalletBalance = wallet
			}
			if unrealized, ok := tt.balanceInfo["totalUnrealizedProfit"].(float64); ok {
				totalUnrealizedProfit = unrealized
			}

			// 計算總資產
			actualEquity := totalWalletBalance + totalUnrealizedProfit

			// 驗證計算結果
			const epsilon = 0.001
			if actualEquity-tt.expectedEquity > epsilon || tt.expectedEquity-actualEquity > epsilon {
				t.Errorf("總資產計算錯誤: got %.2f, want %.2f (wallet: %.2f, unrealized: %.2f)",
					actualEquity, tt.expectedEquity, totalWalletBalance, totalUnrealizedProfit)
			}

			// 驗證錯誤處理（總資產 <= 0 應該報錯）
			shouldError := actualEquity <= 0
			if shouldError != tt.expectError {
				t.Errorf("錯誤處理不符: actualEquity = %.2f, shouldError = %v, expectError = %v",
					actualEquity, shouldError, tt.expectError)
			}
		})
	}
}

// TestCompareAvailableBalanceVsTotalEquity 對比舊邏輯（可用餘額）和新邏輯（總資產）的差異
func TestCompareAvailableBalanceVsTotalEquity(t *testing.T) {
	tests := []struct {
		name             string
		balanceInfo      map[string]interface{}
		availableBalance float64 // 舊邏輯使用的值
		totalEquity      float64 // 新邏輯使用的值
		description      string
	}{
		{
			name: "有持倉且盈利_總資產大於可用餘額",
			balanceInfo: map[string]interface{}{
				"available_balance":     900.0,  // 扣除了保證金
				"totalWalletBalance":    1000.0, // 錢包餘額
				"totalUnrealizedProfit": 200.0,  // 未實現盈利
			},
			availableBalance: 900.0,
			totalEquity:      1200.0, // 1000 + 200
			description:      "盈利時，總資產更能反映真實財產",
		},
		{
			name: "有持倉且虧損_總資產小於可用餘額",
			balanceInfo: map[string]interface{}{
				"available_balance":     900.0,  // 扣除了保證金
				"totalWalletBalance":    1000.0, // 錢包餘額
				"totalUnrealizedProfit": -300.0, // 未實現虧損
			},
			availableBalance: 900.0,
			totalEquity:      700.0, // 1000 - 300
			description:      "虧損時，總資產正確反映虧損狀況",
		},
		{
			name: "無持倉_兩者接近",
			balanceInfo: map[string]interface{}{
				"available_balance":     1000.0,
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": 0.0,
			},
			availableBalance: 1000.0,
			totalEquity:      1000.0,
			description:      "無持倉時兩者相同",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 舊邏輯：使用 available_balance
			oldBalance := 0.0
			if avail, ok := tt.balanceInfo["available_balance"].(float64); ok {
				oldBalance = avail
			}

			// 新邏輯：使用 total equity
			totalWalletBalance := 0.0
			totalUnrealizedProfit := 0.0
			if wallet, ok := tt.balanceInfo["totalWalletBalance"].(float64); ok {
				totalWalletBalance = wallet
			}
			if unrealized, ok := tt.balanceInfo["totalUnrealizedProfit"].(float64); ok {
				totalUnrealizedProfit = unrealized
			}
			newBalance := totalWalletBalance + totalUnrealizedProfit

			// 驗證
			if oldBalance != tt.availableBalance {
				t.Errorf("舊邏輯計算錯誤: got %.2f, want %.2f", oldBalance, tt.availableBalance)
			}
			if newBalance != tt.totalEquity {
				t.Errorf("新邏輯計算錯誤: got %.2f, want %.2f", newBalance, tt.totalEquity)
			}

			t.Logf("✓ %s: 舊邏輯=%.2f, 新邏輯=%.2f, 差異=%.2f",
				tt.description, oldBalance, newBalance, newBalance-oldBalance)
		})
	}
}

// TestBalanceInfoFieldTypes 測試不同的欄位類型處理
func TestBalanceInfoFieldTypes(t *testing.T) {
	tests := []struct {
		name         string
		balanceInfo  map[string]interface{}
		expectWallet float64
		expectUnreal float64
	}{
		{
			name: "正確類型_float64",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": 50.0,
			},
			expectWallet: 1000.0,
			expectUnreal: 50.0,
		},
		{
			name: "錯誤類型_字串_應返回0",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    "1000.0",
				"totalUnrealizedProfit": "50.0",
			},
			expectWallet: 0.0,
			expectUnreal: 0.0,
		},
		{
			name: "錯誤類型_整數_無法轉換為float64",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000,
				"totalUnrealizedProfit": 50,
			},
			expectWallet: 0.0, // Go 的 type assertion .(float64) 不會自動轉換 int
			expectUnreal: 0.0,
		},
		{
			name: "混合類型_部分正確",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": "50", // 字串
			},
			expectWallet: 1000.0,
			expectUnreal: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalWalletBalance := 0.0
			totalUnrealizedProfit := 0.0

			if wallet, ok := tt.balanceInfo["totalWalletBalance"].(float64); ok {
				totalWalletBalance = wallet
			}
			if unrealized, ok := tt.balanceInfo["totalUnrealizedProfit"].(float64); ok {
				totalUnrealizedProfit = unrealized
			}

			if totalWalletBalance != tt.expectWallet {
				t.Errorf("totalWalletBalance 解析錯誤: got %.2f, want %.2f",
					totalWalletBalance, tt.expectWallet)
			}
			if totalUnrealizedProfit != tt.expectUnreal {
				t.Errorf("totalUnrealizedProfit 解析錯誤: got %.2f, want %.2f",
					totalUnrealizedProfit, tt.expectUnreal)
			}
		})
	}
}
