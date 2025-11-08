package trader

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHyperliquidAgentWalletSecurity 測試 Agent Wallet 安全機制
func TestHyperliquidAgentWalletSecurity(t *testing.T) {
	// 生成測試用的私鑰
	agentPrivateKey, err := crypto.GenerateKey()
	require.NoError(t, err, "生成測試私鑰失敗")

	agentPrivateKeyBytes := crypto.FromECDSA(agentPrivateKey)
	agentPrivateKeyHex := hex.EncodeToString(agentPrivateKeyBytes)
	agentAddress := crypto.PubkeyToAddress(agentPrivateKey.PublicKey).Hex()

	// 生成另一個地址作為主錢包地址
	mainPrivateKey, err := crypto.GenerateKey()
	require.NoError(t, err, "生成主錢包私鑰失敗")
	mainWalletAddress := crypto.PubkeyToAddress(mainPrivateKey.PublicKey).Hex()

	t.Run("場景1: 未提供主錢包地址應該拒絕", func(t *testing.T) {
		// 測試：空字符串作為主錢包地址
		trader, err := NewHyperliquidTrader(
			"0x"+agentPrivateKeyHex,
			"",   // 空的主錢包地址
			true, // testnet
		)

		// 驗證：應該返回錯誤
		assert.Error(t, err, "未提供主錢包地址應該返回錯誤")
		assert.Nil(t, trader, "未提供主錢包地址時 trader 應為 nil")
		assert.Contains(t, err.Error(), "安全错误", "錯誤訊息應包含'安全错误'")
		assert.Contains(t, err.Error(), "hyperliquid_wallet_addr", "錯誤訊息應提示缺少字段名")
	})

	t.Run("場景2: 誤用主錢包私鑰（地址相同）", func(t *testing.T) {
		// 測試：使用 agent 私鑰，但主錢包地址設為 agent 地址（模擬誤用主錢包私鑰）
		// 注意：這個測試需要實際連接到 Hyperliquid API，所以我們只驗證初始化邏輯
		// 在實際環境中，這會觸發警告日誌

		// 由於 NewHyperliquidTrader 會嘗試連接 API，我們只能測試參數驗證部分
		// 這裡我們驗證地址比較邏輯
		assert.NotEqual(t, agentAddress, mainWalletAddress,
			"測試設置錯誤：agent 地址和主錢包地址不應相同")

		// 如果地址相同，應該記錄警告（但不會拒絕初始化）
		// 這個場景需要在集成測試中驗證日誌輸出
	})

	t.Run("場景3: 私鑰格式驗證", func(t *testing.T) {
		testCases := []struct {
			name        string
			privateKey  string
			shouldError bool
		}{
			{
				name:        "有效的私鑰（帶0x前綴）",
				privateKey:  "0x" + agentPrivateKeyHex,
				shouldError: false,
			},
			{
				name:        "有效的私鑰（不帶0x前綴）",
				privateKey:  agentPrivateKeyHex,
				shouldError: false,
			},
			{
				name:        "無效的私鑰（過短）",
				privateKey:  "0x1234",
				shouldError: true,
			},
			{
				name:        "無效的私鑰（非十六進制）",
				privateKey:  "0xZZZZ",
				shouldError: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := NewHyperliquidTrader(
					tc.privateKey,
					mainWalletAddress,
					true,
				)

				if tc.shouldError {
					assert.Error(t, err, "應該返回錯誤：%s", tc.name)
				}
				// 注意：有效的私鑰會嘗試連接 API，所以可能因為網絡原因失敗
				// 我們主要關注私鑰解析本身不會崩潰
			})
		}
	})

	t.Run("場景4: 地址格式驗證", func(t *testing.T) {
		validPrivateKey := "0x" + agentPrivateKeyHex

		testCases := []struct {
			name          string
			walletAddress string
			shouldPass    bool
		}{
			{
				name:          "有效的以太坊地址",
				walletAddress: mainWalletAddress,
				shouldPass:    true,
			},
			{
				name:          "有效的地址（小寫）",
				walletAddress: strings.ToLower(mainWalletAddress),
				shouldPass:    true,
			},
			{
				name:          "空地址",
				walletAddress: "",
				shouldPass:    false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := NewHyperliquidTrader(
					validPrivateKey,
					tc.walletAddress,
					true,
				)

				if !tc.shouldPass {
					assert.Error(t, err, "應該返回錯誤：%s", tc.name)
				}
			})
		}
	})
}

// TestPrivateKeyDerivation 測試私鑰地址推導
func TestPrivateKeyDerivation(t *testing.T) {
	// 已知的測試私鑰和對應地址
	testPrivateKeyHex := "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	expectedAddress := "0xFCAd0B19bB29D4674531d6f115237E16AfCE377c" // 這個地址是從上述私鑰推導出的

	// 解析私鑰
	privateKeyHex := strings.TrimPrefix(strings.ToLower(testPrivateKeyHex), "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	require.NoError(t, err, "解析測試私鑰失敗")

	// 推導地址
	derivedAddress := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// 驗證地址推導
	assert.Equal(t,
		strings.ToLower(expectedAddress),
		strings.ToLower(derivedAddress),
		"私鑰推導出的地址不正確")

	t.Logf("✓ 私鑰推導正確: %s -> %s", testPrivateKeyHex, derivedAddress)
}

// TestAgentWalletAddressComparison 測試 Agent 地址與主錢包地址比較邏輯
func TestAgentWalletAddressComparison(t *testing.T) {
	privateKey, _ := crypto.GenerateKey()
	agentAddress := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	testCases := []struct {
		name           string
		mainWalletAddr string
		shouldBeSame   bool
	}{
		{
			name:           "相同地址（大小寫完全一致）",
			mainWalletAddr: agentAddress,
			shouldBeSame:   true,
		},
		{
			name:           "相同地址（全小寫）",
			mainWalletAddr: strings.ToLower(agentAddress),
			shouldBeSame:   true,
		},
		{
			name:           "相同地址（全大寫）",
			mainWalletAddr: strings.ToUpper(agentAddress),
			shouldBeSame:   true,
		},
		{
			name:           "不同地址",
			mainWalletAddr: "0x0000000000000000000000000000000000000000",
			shouldBeSame:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 測試 strings.EqualFold（這是代碼中使用的比較函數）
			isSame := strings.EqualFold(agentAddress, tc.mainWalletAddr)
			assert.Equal(t, tc.shouldBeSame, isSame,
				"地址比較結果不符合預期：agent=%s, main=%s",
				agentAddress, tc.mainWalletAddr)
		})
	}
}

// TestHyperliquidConfigValidation 測試配置驗證邏輯
func TestHyperliquidConfigValidation(t *testing.T) {
	t.Run("私鑰格式處理", func(t *testing.T) {
		privateKey, _ := crypto.GenerateKey()
		privateKeyBytes := crypto.FromECDSA(privateKey)
		privateKeyHex := hex.EncodeToString(privateKeyBytes)

		// 測試各種私鑰格式
		formats := []string{
			"0x" + privateKeyHex,           // 帶 0x 前綴
			privateKeyHex,                  // 不帶 0x 前綴
			"0X" + privateKeyHex,           // 大寫 0X 前綴
			strings.ToUpper(privateKeyHex), // 大寫十六進制
		}

		for i, format := range formats {
			t.Run(fmt.Sprintf("格式 %d", i+1), func(t *testing.T) {
				// 去掉 0x 前綴並轉小寫（模擬代碼中的處理）
				processed := strings.TrimPrefix(strings.ToLower(format), "0x")

				// 驗證可以解析
				parsedKey, err := crypto.HexToECDSA(processed)
				assert.NoError(t, err, "應該能解析格式：%s", format)

				if parsedKey != nil {
					// 驗證推導出的地址一致
					addr1 := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
					addr2 := crypto.PubkeyToAddress(parsedKey.PublicKey).Hex()
					assert.Equal(t,
						strings.ToLower(addr1),
						strings.ToLower(addr2),
						"不同格式應推導出相同地址")
				}
			})
		}
	})
}

// BenchmarkPrivateKeyDerivation 性能測試：私鑰推導地址
func BenchmarkPrivateKeyDerivation(b *testing.B) {
	privateKey, _ := crypto.GenerateKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	}
}

// 輔助函數：生成測試用的私鑰和地址
func generateTestKeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err, "生成測試私鑰失敗")

	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	return privateKey, address
}
