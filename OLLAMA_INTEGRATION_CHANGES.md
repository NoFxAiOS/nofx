# Ollama Provider + Test Model Button — Replication Guide

This document describes all uncommitted changes on the current working tree as of 2026-04-06. Use it as a context file when replicating the same work on another fork of NOFX.

The changes do two things:

1. **Add Ollama as a new AI provider** (local LLMs, no API key required, user-supplied Base URL).
2. **Add a "Test Model" button** inside the Model Config Modal so users can verify their API key / Base URL / model name before saving.

Both pieces are intentionally bundled — the test button was needed specifically to give Ollama users fast feedback (since they typically misconfigure the Base URL).

---

## File summary

### Backend (Go)

| File | Change type |
|------|-------------|
| `mcp/providers.go` | Modify — add `ProviderOllama` constant + `DefaultOllamaModel` |
| `mcp/client.go` | Modify — skip "API key required" check when provider is Ollama (4 call sites) |
| `mcp/provider/ollama.go` | **New file** — Ollama client implementation |
| `api/handler_ai_model.go` | Modify — add `ollama` to default/supported lists + new `handleTestModel` handler |
| `api/server.go` | Modify — register `POST /api/models/test` route |

### Frontend (React/TS)

| File | Change type |
|------|-------------|
| `web/public/icons/ollama.svg` | **New file** — icon for the provider picker |
| `web/src/components/common/ModelIcons.tsx` | Modify — map `ollama` → icon + color |
| `web/src/components/trader/model-constants.ts` | Modify — add `ollama` entry to `AI_PROVIDER_CONFIG` |
| `web/src/types/config.ts` | Modify — add `TestModelRequest` / `TestModelResponse` types |
| `web/src/lib/api/config.ts` | Modify — add `configApi.testModel()` method (with transport encryption support) |
| `web/src/components/trader/ModelConfigModal.tsx` | Modify — Ollama-aware form validation + Test Model button in `StandardProviderConfigForm` |
| `web/src/i18n/translations.ts` | Modify — add 6 new translation keys under `modelConfig` for `en`, `zh`, `id` |

---

## Backend changes (detailed)

### 1. `mcp/providers.go`

Add a new provider constant and a default model constant. Place the `ProviderOllama` constant in the same `const` block as the other providers. **Important**: unlike other providers, Ollama has **no default base URL** — the user must supply one.

```go
const (
    // ... existing providers
    ProviderKimi     = "kimi"
    ProviderMiniMax  = "minimax"

    ProviderOllama  = "ollama"
    ProviderClaw402 = "claw402"

    // ... existing defaults

    // Default Ollama configuration (no default base URL — user must provide)
    DefaultOllamaModel = "llama3.1"
)
```

### 2. `mcp/client.go`

The base client normally rejects calls when `APIKey == ""`. Ollama doesn't need an API key, so we relax the check at **four** call sites. Find every occurrence of:

```go
if client.APIKey == "" {
    return "", fmt.Errorf("AI API key not set, please call SetAPIKey first")
}
```

and change to:

```go
if client.APIKey == "" && client.Provider != ProviderOllama {
    return "", fmt.Errorf("AI API key not set, please call SetAPIKey first")
}
```

The four functions are:
- `CallWithMessages`
- `CallWithRequest`
- `CallWithRequestFull` (returns `nil, fmt.Errorf(...)`)
- `CallWithRequestStream` (shorter error message: `"AI API key not set"`)

Keep the existing error-message wording per function.

### 3. `mcp/provider/ollama.go` (NEW FILE)

Create this file. It registers the provider at `init()` time (this is why `main.go` blank-imports `nofx/mcp/provider`). Two things are Ollama-specific:

- `SetAuthHeader` is overridden to **skip** the Authorization header when `APIKey == ""`.
- `BuildUrl` uses `/v1/chat/completions` (OpenAI-compatible endpoint that Ollama exposes).

```go
package provider

import (
    "fmt"
    "net/http"
    "strings"

    "nofx/mcp"
)

func init() {
    mcp.RegisterProvider(mcp.ProviderOllama, func(opts ...mcp.ClientOption) mcp.AIClient {
        return NewOllamaClientWithOptions(opts...)
    })
}

type OllamaClient struct {
    *mcp.Client
}

func (c *OllamaClient) BaseClient() *mcp.Client { return c.Client }

// NewOllamaClient creates Ollama client (backward compatible)
func NewOllamaClient() mcp.AIClient {
    return NewOllamaClientWithOptions()
}

// NewOllamaClientWithOptions creates Ollama client (supports options pattern)
func NewOllamaClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
    ollamaOpts := []mcp.ClientOption{
        mcp.WithProvider(mcp.ProviderOllama),
        mcp.WithModel(mcp.DefaultOllamaModel),
    }

    allOpts := append(ollamaOpts, opts...)
    baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

    ollamaClient := &OllamaClient{
        Client: baseClient,
    }

    baseClient.Hooks = ollamaClient
    return ollamaClient
}

func (c *OllamaClient) SetAPIKey(apiKey string, customURL string, customModel string) {
    c.APIKey = apiKey // May be empty — Ollama typically needs no auth

    if customURL != "" {
        c.BaseURL = customURL
        c.Log.Infof("🔧 [MCP] Ollama using BaseURL: %s", customURL)
    } else if c.BaseURL == "" {
        c.Log.Warnf("⚠️ [MCP] Ollama requires a Base URL to be set")
    }
    if customModel != "" {
        c.Model = customModel
        c.Log.Infof("🔧 [MCP] Ollama using custom Model: %s", customModel)
    } else {
        c.Log.Infof("🔧 [MCP] Ollama using default Model: %s", c.Model)
    }
}

// SetAuthHeader skips Authorization header when no API key is set
func (c *OllamaClient) SetAuthHeader(reqHeaders http.Header) {
    if c.APIKey != "" {
        c.Client.SetAuthHeader(reqHeaders)
    }
}

// BuildUrl constructs the Ollama OpenAI-compatible endpoint URL
func (c *OllamaClient) BuildUrl() string {
    if c.UseFullURL {
        return c.BaseURL
    }
    return fmt.Sprintf("%s/v1/chat/completions", strings.TrimRight(c.BaseURL, "/"))
}
```

> **Check after writing**: confirm `main.go` already has a blank import like `_ "nofx/mcp/provider"`. If not, this file's `init()` will never run.

### 4. `api/handler_ai_model.go`

Three edits in this file.

**a) Imports.** Add `"time"` to the stdlib group and `"nofx/mcp"` to the project group:

```go
import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "nofx/config"
    "nofx/crypto"
    "nofx/logger"
    "nofx/mcp"
    "nofx/security"
    "nofx/wallet"

    "github.com/gin-gonic/gin"
)
```

**b) Default model list** inside `handleGetModelConfigs`. Add `ollama` right before `claw402` (or wherever the slice ends — match existing order):

```go
{ID: "ollama", Name: "Ollama AI", Provider: "ollama", Enabled: false},
```

**c) Supported models list** inside `handleGetSupportedModels`. Add the same between `minimax` and `claw402`:

```go
{"id": "ollama", "name": "Ollama (Local)", "provider": "ollama", "defaultModel": "llama3.1"},
```

**d) New handler** — append the following after `handleUpdateModelConfigs` and before `handleGetSupportedModels`:

```go
// TestModelRequest request body for testing an AI model connection
type TestModelRequest struct {
    Provider        string `json:"provider"`
    APIKey          string `json:"api_key"`
    CustomAPIURL    string `json:"custom_api_url"`
    CustomModelName string `json:"custom_model_name"`
}

// TestModelResponse response for test model endpoint
type TestModelResponse struct {
    Success   bool   `json:"success"`
    Message   string `json:"message"`
    LatencyMs int64  `json:"latency_ms"`
}

// handleTestModel Test AI model connection with provided credentials
func (s *Server) handleTestModel(c *gin.Context) {
    cfg := config.Get()

    bodyBytes, err := c.GetRawData()
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
        return
    }

    var req TestModelRequest

    if !cfg.TransportEncryption {
        if err := json.Unmarshal(bodyBytes, &req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
            return
        }
    } else {
        var encryptedPayload crypto.EncryptedPayload
        if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil || encryptedPayload.WrappedKey == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Encrypted transmission required"})
            return
        }
        decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
            return
        }
        if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
            return
        }
    }

    if req.Provider == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Provider is required"})
        return
    }
    if req.APIKey == "" && req.Provider != mcp.ProviderOllama {
        c.JSON(http.StatusBadRequest, gin.H{"error": "API key is required"})
        return
    }

    if req.CustomAPIURL != "" {
        cleanURL := strings.TrimSuffix(req.CustomAPIURL, "#")
        if err := security.ValidateURL(cleanURL); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom API URL: %s", err.Error())})
            return
        }
    }

    client := mcp.NewAIClientByProvider(
        req.Provider,
        mcp.WithTimeout(15*time.Second),
        mcp.WithMaxRetries(1),
        mcp.WithMaxTokens(10),
    )
    if client == nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported provider: %s", req.Provider)})
        return
    }

    client.SetAPIKey(req.APIKey, req.CustomAPIURL, req.CustomModelName)

    start := time.Now()
    _, err = client.CallWithMessages("", "Say hi")
    latencyMs := time.Since(start).Milliseconds()

    if err != nil {
        errMsg := err.Error()
        userMsg := "Connection failed"
        switch {
        case strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") || strings.Contains(errMsg, "Unauthorized") || strings.Contains(errMsg, "authentication"):
            userMsg = "Invalid API key"
        case strings.Contains(errMsg, "404"):
            userMsg = "Model not found"
        case strings.Contains(errMsg, "429"):
            userMsg = "Rate limited"
        case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "Timeout"):
            userMsg = "Request timed out"
        case strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host"):
            userMsg = "Cannot reach API endpoint"
        }
        logger.Infof("❌ Model test failed for %s: %v", req.Provider, err)
        c.JSON(http.StatusOK, TestModelResponse{Success: false, Message: userMsg, LatencyMs: latencyMs})
        return
    }

    c.JSON(http.StatusOK, TestModelResponse{Success: true, Message: "Connection successful", LatencyMs: latencyMs})
}
```

Notes:
- Failures return **HTTP 200** with `success: false` so the frontend can show inline feedback without triggering error boundaries.
- The client is configured with a tight `15s` timeout, `1` retry, and `10` max tokens — we just want a ping, not a full completion.
- Transport encryption (RSA-wrapped AES payload) must be honoured — copy the pattern from neighboring handlers in the same file; the access path is `s.cryptoHandler.cryptoService.DecryptSensitiveData(...)`.

### 5. `api/server.go`

Register the new route directly after the `POST /models` route inside the protected group. Add:

```go
s.routeWithSchema(protected, "POST", "/models/test", "Test AI model connection with provided credentials",
    `Body: {"provider":"<string>","api_key":"<string>","custom_api_url":"<string, optional>","custom_model_name":"<string, optional>"}
Returns: {"success":<bool>,"message":"<string>","latency_ms":<int>}`,
    s.handleTestModel)
```

Keep it inside the same `protected` group as the other `/models*` routes so JWT middleware applies.

---

## Frontend changes (detailed)

### 1. `web/public/icons/ollama.svg` (NEW FILE)

A simple 1024×1024 white llama on dark rounded square. Any placeholder SVG works — the repo currently ships a hand-rolled one. If you don't care about styling, copy this:

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" fill="none">
  <rect width="1024" height="1024" rx="200" fill="#1a1a2e"/>
  <g transform="translate(512, 480)" fill="white">
    <ellipse cx="0" cy="-120" rx="120" ry="140"/>
    <ellipse cx="0" cy="80" rx="160" ry="180"/>
    <ellipse cx="-40" cy="-150" rx="18" ry="22" fill="#1a1a2e"/>
    <ellipse cx="40" cy="-150" rx="18" ry="22" fill="#1a1a2e"/>
    <ellipse cx="-34" cy="-156" rx="6" ry="8" fill="white"/>
    <ellipse cx="46" cy="-156" rx="6" ry="8" fill="white"/>
    <ellipse cx="-100" cy="-220" rx="35" ry="60" transform="rotate(-15)" fill="white"/>
    <ellipse cx="100" cy="-220" rx="35" ry="60" transform="rotate(15)" fill="white"/>
    <ellipse cx="0" cy="-100" rx="12" ry="8" fill="#1a1a2e"/>
  </g>
</svg>
```

### 2. `web/src/components/common/ModelIcons.tsx`

Two edits:

**a)** Add `ollama: '#FFFFFF'` to the `MODEL_COLORS` record (place next to `minimax`).

**b)** Add a `case` to the `getModelIcon` switch (between `minimax` and `claw402`):

```tsx
case 'ollama':
  iconPath = '/icons/ollama.svg'
  break
```

### 3. `web/src/components/trader/model-constants.ts`

Add an `ollama` entry inside `AI_PROVIDER_CONFIG`. Note the **empty `apiUrl`** — this is load-bearing: the modal uses it to decide whether to render the "Get API key" link, and Ollama shouldn't show one.

```ts
ollama: {
  defaultModel: 'llama3.1',
  apiUrl: '',
  apiName: 'Ollama',
},
```

### 4. `web/src/types/config.ts`

Append at the bottom of the file:

```ts
export interface TestModelRequest {
  provider: string
  api_key: string
  custom_api_url?: string
  custom_model_name?: string
}

export interface TestModelResponse {
  success: boolean
  message: string
  latency_ms: number
}
```

Make sure these are re-exported from `web/src/types/index.ts` if that barrel exists — check how the existing `CreateExchangeRequest` is exported and mirror it.

### 5. `web/src/lib/api/config.ts`

**a)** Import the new types at the top:

```ts
import type {
  // ... existing
  TestModelRequest,
  TestModelResponse,
} from '../../types'
```

**b)** Add a `testModel` method to the `configApi` object, after `updateModelConfigs`. It mirrors the transport-encryption pattern used by `updateModelConfigs`:

```ts
async testModel(request: TestModelRequest): Promise<TestModelResponse> {
  const config = await CryptoService.fetchCryptoConfig()

  if (!config.transport_encryption) {
    const result = await httpClient.post<TestModelResponse>(
      `${API_BASE}/models/test`,
      request
    )
    if (!result.success || !result.data)
      throw new Error('Failed to test model')
    return result.data
  }

  const publicKey = await CryptoService.fetchPublicKey()
  await CryptoService.initialize(publicKey)
  const userId = localStorage.getItem('user_id') || ''
  const sessionId = sessionStorage.getItem('session_id') || ''
  const encryptedPayload = await CryptoService.encryptSensitiveData(
    JSON.stringify(request),
    userId,
    sessionId
  )
  const result = await httpClient.post<TestModelResponse>(
    `${API_BASE}/models/test`,
    encryptedPayload
  )
  if (!result.success || !result.data) throw new Error('Failed to test model')
  return result.data
},
```

### 6. `web/src/components/trader/ModelConfigModal.tsx`

This is the biggest frontend edit. Three concerns:

- Ollama needs **Base URL** to be required and **API Key** to be optional (flip of the usual rule).
- A "Test Model" button that calls `api.testModel(...)` and shows inline ✓/✗ feedback.
- Small tweaks so the "Get API key" link is hidden for Ollama.

**a) Top-level `handleSubmit`** — replace the simple guard with Ollama-aware validation:

```tsx
const handleSubmit = (e: React.FormEvent) => {
  e.preventDefault()
  const isOllama = selectedModel?.provider === 'ollama'
  if (!selectedModelId) return
  if (isOllama) {
    if (!baseUrl.trim()) return
  } else {
    if (!apiKey.trim()) return
  }
  onSave(selectedModelId, apiKey.trim(), baseUrl.trim() || undefined, modelName.trim() || undefined)
}
```

**b) Inside `StandardProviderConfigForm`** (the child component rendering the form):

Add local state and an `isOllama` flag at the top of the component:

```tsx
const [testing, setTesting] = useState(false)
const [testResult, setTestResult] = useState<{ success: boolean; message: string; latencyMs: number } | null>(null)

// Reset test result when inputs change
useEffect(() => {
  setTestResult(null)
}, [apiKey, baseUrl, modelName])

const isOllama = selectedModel.provider === 'ollama'

const handleTestModel = async () => {
  if (isOllama ? !baseUrl.trim() : !apiKey.trim()) return
  setTesting(true)
  setTestResult(null)
  try {
    const result = await api.testModel({
      provider: selectedModel.provider || selectedModel.id,
      api_key: apiKey.trim(),
      custom_api_url: baseUrl.trim() || undefined,
      custom_model_name: modelName.trim() || undefined,
    })
    setTestResult({ success: result.success, message: result.message, latencyMs: result.latency_ms })
  } catch {
    setTestResult({ success: false, message: t('modelConfig.testModelFailed', language), latencyMs: 0 })
  } finally {
    setTesting(false)
  }
}
```

Make sure `useEffect` and `useState` are imported from React, and `api` is the configApi alias used elsewhere in the file. Check the existing imports before adding duplicates.

**c) In the JSX inside `StandardProviderConfigForm`**:

- Gate the "Get API key" link on `AI_PROVIDER_CONFIG[provider]?.apiUrl` instead of just `AI_PROVIDER_CONFIG[provider]` (so Ollama's empty URL suppresses the link).
- Make the API Key label show `*` only when not Ollama: `{isOllama ? 'API Key' : 'API Key *'}`.
- Change the API Key input: set `required={!isOllama}` and use `isOllama ? t('modelConfig.ollamaNoKeyHint', language) : t('enterAPIKey', language)` as placeholder.
- Change the Base URL label: `{isOllama ? \`${t('customBaseURL', language)} *\` : t('customBaseURL', language)}`.
- Change the Base URL input: set `required={isOllama}` and placeholder `{isOllama ? 'http://192.168.1.100:11434' : t('customBaseURLPlaceholder', language)}`.
- Change the Base URL hint: `{isOllama ? t('modelConfig.ollamaBaseURLHint', language) : t('leaveBlankForDefault', language)}`.
- Change the submit button `disabled`: `{!selectedModel || (isOllama ? !baseUrl.trim() : !apiKey.trim())}`.

**d) Test Model button block** — insert this right before the existing "Info Box" section:

```tsx
{/* Test Model */}
<div className="flex items-center gap-3">
  <button
    type="button"
    onClick={handleTestModel}
    disabled={(isOllama ? !baseUrl.trim() : !apiKey.trim()) || testing}
    className="px-4 py-2.5 rounded-xl text-sm font-semibold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
    style={{ background: '#2B3139', color: '#EAECEF', border: '1px solid #3B4149' }}
  >
    {testing ? t('modelConfig.testingModel', language) : t('modelConfig.testModel', language)}
  </button>
  {testResult && (
    <div className="flex items-center gap-2 text-sm">
      {testResult.success ? (
        <>
          <svg className="w-4 h-4" style={{ color: '#00E096' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
          </svg>
          <span style={{ color: '#00E096' }}>
            {t('modelConfig.testModelSuccess', language)} ({testResult.latencyMs}ms)
          </span>
        </>
      ) : (
        <>
          <svg className="w-4 h-4" style={{ color: '#F6465D' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
          <span style={{ color: '#F6465D' }}>{testResult.message}</span>
        </>
      )}
    </div>
  )}
</div>
```

### 7. `web/src/i18n/translations.ts`

Six new keys inside the `modelConfig` block for **each** language present in the file. On the current branch the file only has `en`, `zh`, and `id` sections — add the keys to every block that exists on the target fork.

```ts
testModel: 'Test Model',
testingModel: 'Testing...',
testModelSuccess: 'Connection successful',
testModelFailed: 'Connection failed',
ollamaNoKeyHint: 'Optional — Ollama usually needs no API key',
ollamaBaseURLHint: 'Enter your Ollama server address (e.g., http://192.168.1.100:11434)',
```

**Chinese (`zh`):**
```ts
testModel: '测试模型',
testingModel: '测试中...',
testModelSuccess: '连接成功',
testModelFailed: '连接失败',
ollamaNoKeyHint: '可选 — Ollama 通常不需要 API Key',
ollamaBaseURLHint: '输入 Ollama 服务器地址（例如 http://192.168.1.100:11434）',
```

**Indonesian (`id`):**
```ts
testModel: 'Tes Model',
testingModel: 'Menguji...',
testModelSuccess: 'Koneksi berhasil',
testModelFailed: 'Koneksi gagal',
ollamaNoKeyHint: 'Opsional — Ollama biasanya tidak memerlukan API key',
ollamaBaseURLHint: 'Masukkan alamat server Ollama (misal: http://192.168.1.100:11434)',
```

> ⚠️ **Known bug in the source tree**: the current working copy has a mojibake character in the Chinese `ollamaNoKeyHint` — `'可�� —'` instead of `'可选 —'`. Use the corrected value above. Do **not** copy the broken bytes.

Add the same three `modelConfig` sibling keys (`testConnection`, `testingConnection`, etc.) that already exist nearby — do not move them.

---

## Suggested order of operations when replicating

1. Backend first, verify `go build ./...` and `make lint` pass.
2. Write the Ollama provider file; confirm `main.go` blank-imports `nofx/mcp/provider`.
3. Add the frontend types + api client method.
4. Edit `ModelConfigModal.tsx` last (it depends on the types and api client existing).
5. Add translation keys; run `cd web && npm run lint:fix && npm run build` to catch missing keys.
6. Manual smoke test:
   - Add Ollama from the modal with just Base URL → Save should succeed.
   - Click "Test Model" with a bad URL → expect red message.
   - Click "Test Model" against a real Ollama instance → expect green `Connection successful (Xms)`.
   - Switch to an OpenAI-style provider → "Test Model" should refuse to fire until API key is present.

## Commit suggestion

Match the repo's Conventional Commits style. A reasonable split:

```
feat(mcp): add Ollama provider with optional API key and custom base URL
feat(api): add POST /models/test endpoint for AI model connectivity check
feat(web): add Test Model button and Ollama support in ModelConfigModal
```

Or a single bundled commit if the fork prefers fewer commits:

```
feat: add Ollama provider and Test Model button
```