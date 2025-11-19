# LIGHTER 前端實現指南

## 📋 概述

本文檔詳細說明如何在 NOFX 前端添加 LIGHTER DEX 的完整支持，包括：
1. **API Key 配置界面** - 讓用戶輸入 L1 私鑰和 API Key
2. **V1/V2 狀態顯示** - 顯示當前使用的 SDK 版本
3. **安全輸入處理** - 使用加密輸入組件保護私鑰

---

## 🎯 需要修改的文件

### 1. `web/src/components/traders/ExchangeConfigModal.tsx`

這是主要的交易所配置彈窗組件，需要添加 LIGHTER 特定的輸入字段。

#### 步驟 1.1: 添加狀態變量

在現有的 Aster 和 Hyperliquid 狀態變量後面添加（約第 70 行）：

```typescript
// LIGHTER 特定字段
const [lighterWalletAddr, setLighterWalletAddr] = useState('')
const [lighterPrivateKey, setLighterPrivateKey] = useState('')
const [lighterApiKeyPrivateKey, setLighterApiKeyPrivateKey] = useState('')
```

#### 步驟 1.2: 更新安全輸入目標類型

修改 `secureInputTarget` 類型定義（約第 74 行）：

```typescript
const [secureInputTarget, setSecureInputTarget] = useState<
  null | 'hyperliquid' | 'aster' | 'lighter'  // 添加 'lighter'
>(null)
```

#### 步驟 1.3: 初始化表單數據

在 `useEffect` 中添加 LIGHTER 字段初始化（約第 96 行）：

```typescript
// LIGHTER 字段
setLighterWalletAddr(selectedExchange.lighterWalletAddr || '')
setLighterPrivateKey('') // Don't load existing private key for security
setLighterApiKeyPrivateKey('') // Don't load existing API key for security
```

#### 步驟 1.4: 添加表單輸入字段

在 Hyperliquid 配置部分後面添加（約第 831 行）：

```tsx
{/* LIGHTER 特定配置 */}
{selectedExchange?.id === 'lighter' && (
  <>
    {/* L1 Wallet Address */}
    <div className="mb-4">
      <label
        className="block text-sm font-semibold mb-2"
        style={{ color: '#EAECEF' }}
      >
        {t('lighterWalletAddress', language)}
      </label>
      <input
        type="text"
        value={lighterWalletAddr}
        onChange={(e) => setLighterWalletAddr(e.target.value)}
        placeholder={t('enterLighterWalletAddress', language)}
        className="w-full px-3 py-2 rounded"
        style={{
          background: '#0B0E11',
          border: '1px solid #2B3139',
          color: '#EAECEF',
        }}
        required
      />
      <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
        {t('lighterWalletAddressDesc', language)}
      </div>
    </div>

    {/* L1 Private Key (Secure Input) */}
    <div className="mb-4">
      <label
        className="block text-sm font-semibold mb-2"
        style={{ color: '#EAECEF' }}
      >
        {t('lighterPrivateKey', language)}
        <button
          type="button"
          onClick={() => setSecureInputTarget('lighter')}
          className="ml-2 text-xs underline"
          style={{ color: '#F0B90B' }}
        >
          {t('useSecureInput', language)}
        </button>
      </label>
      <input
        type="password"
        value={lighterPrivateKey}
        onChange={(e) => setLighterPrivateKey(e.target.value)}
        placeholder={t('enterLighterPrivateKey', language)}
        className="w-full px-3 py-2 rounded font-mono text-sm"
        style={{
          background: '#0B0E11',
          border: '1px solid #2B3139',
          color: '#EAECEF',
        }}
        required
      />
      <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
        {t('lighterPrivateKeyDesc', language)}
      </div>
    </div>

    {/* API Key Private Key (Secure Input) */}
    <div className="mb-4">
      <label
        className="block text-sm font-semibold mb-2"
        style={{ color: '#EAECEF' }}
      >
        {t('lighterApiKeyPrivateKey', language)} ⭐
      </label>
      <input
        type="password"
        value={lighterApiKeyPrivateKey}
        onChange={(e) => setLighterApiKeyPrivateKey(e.target.value)}
        placeholder={t('enterLighterApiKeyPrivateKey', language)}
        className="w-full px-3 py-2 rounded font-mono text-sm"
        style={{
          background: '#0B0E11',
          border: '1px solid #2B3139',
          color: '#EAECEF',
        }}
      />
      <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
        {t('lighterApiKeyPrivateKeyDesc', language)}
      </div>
      <div className="text-xs mt-2 p-2 rounded" style={{
        background: '#1E2329',
        border: '1px solid #2B3139',
        color: '#F0B90B'
      }}>
        💡 {t('lighterApiKeyOptionalNote', language)}
      </div>
    </div>

    {/* V1/V2 狀態顯示 */}
    <div className="mb-4 p-3 rounded" style={{
      background: lighterApiKeyPrivateKey ? '#0F3F2E' : '#3F2E0F',
      border: '1px solid ' + (lighterApiKeyPrivateKey ? '#10B981' : '#F59E0B')
    }}>
      <div className="flex items-center gap-2">
        <div className="text-sm font-semibold" style={{
          color: lighterApiKeyPrivateKey ? '#10B981' : '#F59E0B'
        }}>
          {lighterApiKeyPrivateKey ? '✅ LIGHTER V2' : '⚠️ LIGHTER V1'}
        </div>
      </div>
      <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
        {lighterApiKeyPrivateKey
          ? t('lighterV2Description', language)
          : t('lighterV1Description', language)
        }
      </div>
    </div>
  </>
)}
```

#### 步驟 1.5: 更新提交按鈕驗證邏輯

在 `disabled` 屬性中添加 LIGHTER 驗證（約第 860 行）：

```typescript
(selectedExchange.id === 'lighter' &&
  (!lighterWalletAddr.trim() || !lighterPrivateKey.trim())) ||
```

#### 步驟 1.6: 處理安全輸入回調

在 `TwoStageKeyModal` 的 `onComplete` 回調中添加 LIGHTER 處理（查找現有的 onComplete 函數）：

```typescript
if (secureInputTarget === 'lighter') {
  setLighterPrivateKey(result.combinedKey)
  setSecureInputTarget(null)
  toast.success(t('lighterPrivateKeyImported', language))
}
```

---

### 2. `web/src/i18n/translations.ts`

添加所有 LIGHTER 相關的翻譯字符串。

#### 步驟 2.1: 在中文翻譯中添加（zh-TW 部分）

```typescript
// LIGHTER 配置
lighterWalletAddress: 'L1 錢包地址',
lighterPrivateKey: 'L1 私鑰',
lighterApiKeyPrivateKey: 'API Key 私鑰',
enterLighterWalletAddress: '請輸入以太坊錢包地址（0x...）',
enterLighterPrivateKey: '請輸入 L1 私鑰（32 字節）',
enterLighterApiKeyPrivateKey: '請輸入 API Key 私鑰（40 字節，可選）',
lighterWalletAddressDesc: '您的以太坊錢包地址，用於識別賬戶',
lighterPrivateKeyDesc: 'L1 私鑰用於賬戶識別（32 字節 ECDSA 私鑰）',
lighterApiKeyPrivateKeyDesc: 'API Key 私鑰用於簽名交易（40 字節 Poseidon2 私鑰）',
lighterApiKeyOptionalNote: '如果不提供 API Key，系統將使用功能受限的 V1 模式',
lighterV1Description: '基本模式 - 功能受限，僅用於測試框架',
lighterV2Description: '完整模式 - 支持 Poseidon2 簽名和真實交易',
lighterPrivateKeyImported: 'LIGHTER 私鑰已導入',
```

#### 步驟 2.2: 在英文翻譯中添加（en 部分）

```typescript
// LIGHTER Configuration
lighterWalletAddress: 'L1 Wallet Address',
lighterPrivateKey: 'L1 Private Key',
lighterApiKeyPrivateKey: 'API Key Private Key',
enterLighterWalletAddress: 'Enter Ethereum wallet address (0x...)',
enterLighterPrivateKey: 'Enter L1 private key (32 bytes)',
enterLighterApiKeyPrivateKey: 'Enter API Key private key (40 bytes, optional)',
lighterWalletAddressDesc: 'Your Ethereum wallet address for account identification',
lighterPrivateKeyDesc: 'L1 private key for account identification (32-byte ECDSA key)',
lighterApiKeyPrivateKeyDesc: 'API Key private key for transaction signing (40-byte Poseidon2 key)',
lighterApiKeyOptionalNote: 'Without API Key, system will use limited V1 mode',
lighterV1Description: 'Basic Mode - Limited functionality, testing framework only',
lighterV2Description: 'Full Mode - Supports Poseidon2 signing and real trading',
lighterPrivateKeyImported: 'LIGHTER private key imported',
```

---

### 3. `web/src/components/traders/sections/ExchangesSection.tsx`

更新 API 調用以包含 LIGHTER 參數。

#### 步驟 3.1: 找到 `handleSaveExchange` 函數

#### 步驟 3.2: 在函數簽名中添加 LIGHTER 參數

```typescript
const handleSaveExchange = async (
  exchangeId: string,
  apiKey: string,
  secretKey?: string,
  testnet?: boolean,
  hyperliquidWalletAddr?: string,
  asterUser?: string,
  asterSigner?: string,
  asterPrivateKey?: string,
  lighterWalletAddr?: string,      // 新增
  lighterPrivateKey?: string,      // 新增
  lighterApiKeyPrivateKey?: string // 新增
) => {
  // ... 函數實現
}
```

#### 步驟 3.3: 在 API 調用中包含 LIGHTER 參數

```typescript
await api.updateExchangeConfig(exchangeId, {
  apiKey,
  secretKey,
  testnet,
  hyperliquidWalletAddr,
  asterUser,
  asterSigner,
  asterPrivateKey,
  lighterWalletAddr,         // 新增
  lighterPrivateKey,         // 新增
  lighterApiKeyPrivateKey,   // 新增
})
```

---

### 4. `web/src/lib/api.ts`

更新 API 客戶端方法簽名。

#### 步驟 4.1: 找到 `updateExchangeConfig` 方法

#### 步驟 4.2: 更新請求參數接口

```typescript
interface UpdateExchangeConfigRequest {
  apiKey?: string
  secretKey?: string
  testnet?: boolean
  hyperliquidWalletAddr?: string
  asterUser?: string
  asterSigner?: string
  asterPrivateKey?: string
  lighterWalletAddr?: string        // 新增
  lighterPrivateKey?: string        // 新增
  lighterApiKeyPrivateKey?: string  // 新增
}
```

---

## 🎨 視覺效果

### V1 模式顯示（無 API Key）
```
┌────────────────────────────────────────┐
│ ⚠️ LIGHTER V1                          │
│ 基本模式 - 功能受限，僅用於測試框架       │
└────────────────────────────────────────┘
背景: #3F2E0F (橙色調)
邊框: #F59E0B (橙色)
```

### V2 模式顯示（有 API Key）
```
┌────────────────────────────────────────┐
│ ✅ LIGHTER V2                          │
│ 完整模式 - 支持 Poseidon2 簽名和真實交易 │
└────────────────────────────────────────┘
背景: #0F3F2E (綠色調)
邊框: #10B981 (綠色)
```

---

## 🔒 安全注意事項

1. **私鑰永不回顯**
   - 編輯現有配置時，私鑰字段應為空
   - 只在保存時發送新的私鑰值

2. **安全輸入選項**
   - 提供「使用安全輸入」按鈕
   - 通過 TwoStageKeyModal 組件導入私鑰
   - 支持分段輸入和加密存儲

3. **可選的 API Key**
   - 不強制要求 API Key
   - 明確提示 V1 和 V2 的區別
   - 允許後續升級到 V2

---

## 📝 測試清單

### 功能測試
- [ ] 創建新的 LIGHTER 配置
- [ ] 編輯現有的 LIGHTER 配置
- [ ] 驗證必填字段（錢包地址、L1 私鑰）
- [ ] 驗證可選字段（API Key 私鑰）
- [ ] V1/V2 狀態正確顯示
- [ ] 安全輸入功能正常工作

### UI 測試
- [ ] 輸入字段樣式正確
- [ ] 幫助文本清晰可讀
- [ ] V1/V2 狀態框顏色正確
- [ ] 響應式布局正常
- [ ] 深色主題兼容

### 數據驗證
- [ ] API 請求包含所有字段
- [ ] 後端正確保存配置
- [ ] Trader 正確檢測 V1/V2 模式
- [ ] 私鑰安全處理

---

## 🚀 實現順序建議

1. **第一步**: 更新翻譯文件
   - 最簡單，不會破壞任何功能
   - 提前準備好所有文本

2. **第二步**: 修改 API 接口
   - 更新類型定義
   - 確保前後端對齊

3. **第三步**: 實現 Modal 組件
   - 添加狀態變量
   - 實現表單字段
   - 添加驗證邏輯

4. **第四步**: 集成安全輸入
   - 更新 TwoStageKeyModal 回調
   - 測試加密導入流程

5. **第五步**: 全面測試
   - 功能測試
   - UI 測試
   - 集成測試

---

## 📚 參考資料

- **後端實現**: `LIGHTER_INTEGRATION.md`
- **SDK 文檔**: https://github.com/elliottech/lighter-go
- **API 文檔**: https://apidocs.lighter.xyz/
- **現有組件**: `ExchangeConfigModal.tsx` (Hyperliquid 和 Aster 實現)

---

**創建時間**: 2025-01-20
**文檔版本**: 1.0.0
**作者**: Claude (Anthropic)
