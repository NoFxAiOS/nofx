# x402 Streaming Payment Architecture

## Overview

NOFX calls AI models (DeepSeek, GPT, Claude, etc.) through the claw402 gateway, using the [x402 protocol](https://github.com/coinbase/x402) to pay per request with USDC on Base L2.

This document describes the full implementation of the SSE streaming call mode, including client, server, and billing logic.

## Why Streaming Is Needed

```
NOFX (client)  ──→  Cloudflare (100s idle limit)  ──→  claw402 (gateway)  ──→  AI upstream
```

- DeepSeek inference takes 60–180 seconds (up to 5 minutes)
- Cloudflare enforces a **100-second hard limit** on idle connections, returning 520/EOF on timeout
- Non-streaming mode: the client receives no data until inference completes — Cloudflare disconnects after 100 seconds
- Streaming mode: the first byte arrives within seconds, subsequent chunks flow continuously, keeping Cloudflare alive

## End-to-End Request Flow

```
                       NOFX Client                           claw402 Gateway                    AI Upstream
                            │                                    │                                │
  ── Phase 1: Payment ──────────────────────────────────────────────────────────────────────────────
                            │                                    │                                │
  1. POST /api/v1/ai/...   │ ─── body + stream:true ──────────→ │                                │
     (no payment header)    │                                    │                                │
                            │ ←── 402 + Payment-Required ────── │                                │
                            │     (base64 JSON: price/chain/asset)                                │
                            │                                    │                                │
  2. EIP-712 signing        │                                    │                                │
     (USDC TransferWithAuth)│                                    │                                │
                            │                                    │                                │
  3. POST + X-Payment hdr  │ ─── body + signature ────────────→ │                                │
                            │                                    │ ── verify signature → Facilitator
                            │                                    │ ←── OK ──────────── Facilitator
                            │                                    │ ── settle USDC ───→ Facilitator
                            │                                    │ ←── tx hash ─────── Facilitator
                            │                                    │                                │
  ── Phase 2: Streaming Response ───────────────────────────────────────────────────────────────────
                            │                                    │                                │
                            │ ←── 200 OK ────────────────────── │ ─── POST stream:true ────────→ │
                            │ ←── data: {"choices":[...]} ───── │ ←── SSE chunk ────────────────  │
                            │ ←── data: {"choices":[...]} ───── │ ←── SSE chunk ────────────────  │
                            │ ←── ... (continuous) ──────────── │ ←── ... ─────────────────────── │
                            │ ←── data: [DONE] ──────────────── │ ←── data: [DONE] ────────────── │
```

## Client Implementation (NOFX)

### File Structure

| File | Responsibility |
|------|----------------|
| `mcp/payment/claw402.go` | Claw402Client — model routing, wallet management |
| `mcp/payment/x402.go` | x402 payment flow core — DoX402RequestStream, X402CallStream |
| `mcp/client.go` | ParseSSEStream — shared SSE parsing function |

### Call Chain

```
Claw402Client.Call()
  └→ X402CallStream()                          // x402.go:380
       ├→ Build request body + inject stream:true
       ├→ DoX402RequestStream()                // x402.go:239
       │    ├→ Send initial request (no payment header)
       │    ├→ Receive 402 → parse Payment-Required header
       │    ├→ signFn() → EIP-712 signature
       │    └→ Send retry request with X-Payment header → return open *http.Response
       │
       ├→ Start idle timeout watchdog (90s with no data → disconnect)
       ├→ TeeReader: simultaneous SSE parsing + raw byte buffering
       ├→ ParseSSEStream()                     // client.go:703
       │    ├→ bufio.Scanner line-by-line read
       │    ├→ Parse "data: {...}" → OpenAI chunk format
       │    └→ Accumulate text + call onChunk callback
       │
       └→ Fallback: if SSE yields nothing, try JSON parsing on buffered bodyBuf
```

### Request Identification

Every request carries an `X-Client-ID: nofx` header (`x402.go:473`), allowing claw402 to identify the request source for logging and monitoring.

### Model Routing

`claw402ModelEndpoints` maps user-friendly model names to API paths:

```go
"deepseek"     → "/api/v1/ai/deepseek/chat"
"gpt-5.4"      → "/api/v1/ai/openai/chat/5.4"
"claude-opus"  → "/api/v1/ai/anthropic/messages/opus"
"qwen-max"     → "/api/v1/ai/qwen/chat/max"
// ... more
```

Anthropic endpoints (containing `/anthropic/`) automatically switch to the Messages API wire format.

## Server Implementation (claw402)

### Core Problem: ginmw Is Incompatible with SSE

Coinbase's standard Gin middleware `ginmw.PaymentMiddlewareFromConfig` internally works as follows:

```
1. Wrap c.Writer with responseCapture (all writes go to buffer)
2. c.Next() — handler runs, SSE chunks all go into buffer
3. Settle payment after handler completes
4. Write buffered content to client only after successful settlement
```

Problems:
- SSE chunks are buffered — the client receives no data for minutes
- Cloudflare disconnects after 100 seconds → 520 error
- Handler runs too long (5 min), settlement context expires

### Solution: streamAwareX402Middleware

Dual-path design (`internal/gateway/x402.go`):

```go
func streamAwareX402Middleware(streamServer, standardMW) {
    return func(c *gin.Context) {
        if !isStreamingBody(c) {
            standardMW(c)   // Non-streaming → standard ginmw (battle-tested)
            return
        }
        // Streaming → custom path
    }
}
```

#### Non-Streaming Path

Delegates entirely to `ginmw.PaymentMiddlewareFromConfig` with no custom logic.

#### Streaming Path (Pre-Settlement)

```
1. isStreamingBody(c) — read body to check for {"stream": true}, restore body
2. streamServer.RequiresPayment(reqCtx) — does this route require payment?
3. streamServer.ProcessHTTPRequest() — verify X-Payment signature
4. handleStreamingPayment():
   a. ProcessSettlement() — settle USDC on-chain (collect payment first)
   b. c.Next() — pass to HandleAPIKeyStream
   c. SSE chunks write directly to c.Writer (no responseCapture buffer)
```

Key differences:

| | Standard ginmw (non-streaming) | Custom path (streaming) |
|---|---|---|
| Settlement timing | **After** handler completes | **Before** handler starts |
| Response buffer | `responseCapture` buffers everything | No buffer, writes directly to client |
| Timeout risk | Slow handler causes context expiry | Settlement uses `context.Background()` |
| SSE compatible | No | Yes |

## Billing Logic

### x402 Protocol Flow

x402 is an HTTP 402 payment protocol proposed by Coinbase. Core roles:

- **Resource Server** (claw402) — provides paid APIs
- **Client** (NOFX) — consumer, holds an EVM wallet
- **Facilitator** (Coinbase CDP) — verifies signatures, executes on-chain settlement

### Payment Signing (EIP-712)

Client signature type: USDC `TransferWithAuthorization`

```
1. Receive Payment-Required header from 402 response (base64 JSON)
2. Decode to get:
   - scheme: "exact"
   - network: "eip155:8453" (Base L2)
   - amount: USDC amount (e.g., "3000" = $0.003)
   - asset: USDC contract address
   - payTo: claw402 recipient address
3. Sign with wallet private key using EIP-712, authorizing USDC transfer from user wallet to payTo
4. Place signature in X-Payment + Payment-Signature headers
```

### Pricing Models

Each AI model route has its own price configured in claw402:

| Mode | Description | Example |
|------|-------------|---------|
| Fixed price | Specified directly via `user_price` field | `$0.003` per request |
| Token-based dynamic pricing | Calculated from request token count | `$0.001` per 1K tokens |
| Dispatch fallback | Default price for SDK-compatible routes | `$0.01` per request |

```go
// Fixed price
price := fmt.Sprintf("$%s", route.UserPrice)

// Dynamic pricing
price = DynamicPriceFunc(func(ctx, reqCtx) (Price, error) {
    return resolveDynamicPrice(ctx, reqCtx, rule)
})
```

### Retry Logic and Double-Charge Prevention

```go
const X402MaxPaymentRetries = 5
const X402RetryBaseWait     = 3 * time.Second
```

- **5xx errors** → exponential backoff retry (3s, 6s, 9s...), no re-signing (same payment authorization)
- **Another 402** → previous signature expired, re-sign and retry (on-chain authorization auto-invalidates, **no double charge**)
- **4xx (non-402)** → non-retryable, fail immediately
- Outer retry is set to 1 (`WithMaxRetries(1)`) to prevent outer retries from causing duplicate payments

### Settlement Timing: Streaming vs Non-Streaming

| | Non-Streaming | Streaming |
|---|---|---|
| Settlement timing | After receiving full response | Before streaming begins |
| Risk | Low (content confirmed before charge) | Slightly higher (charge before seeing content) |
| Necessity | Standard mode | Must charge first, otherwise SSE is buffered |

## Timeout Configuration

| Location | Timeout | Purpose |
|----------|---------|---------|
| NOFX `X402Timeout` | 5 min | HTTP client overall timeout |
| NOFX `x402StreamIdleTimeout` | 90s | SSE idle disconnect (prevent hangs) |
| NOFX `CallWithRequestStream` idle | 60s | Idle timeout for non-x402 streaming |
| claw402 `ResponseHeaderTimeout` | 120s | Wait for first byte from AI upstream |
| claw402 `streamingHTTP.Timeout` | 0 (unlimited) | SSE stream can last indefinitely |
| claw402 `standardMW WithTimeout` | 10 min | Non-streaming ginmw overall timeout |
| claw402 `x402PaymentTimeout` | 30s | Payment verification/settlement timeout |

## SSE Fault Tolerance

### TeeReader Dual Parsing

```go
var bodyBuf bytes.Buffer
tee := io.TeeReader(resp.Body, &bodyBuf)
text, sseErr := ParseSSEStream(tee, onChunk, onLine)

if text != "" {
    return text, nil  // SSE succeeded
}
// SSE yielded nothing → try JSON parsing on bodyBuf (server may have returned non-streaming JSON)
jsonText, _ := ParseMCPResponse(bodyBuf.Bytes())
```

### Idle Timeout Watchdog

```go
go func() {
    t := time.NewTimer(90s)
    for {
        select {
        case <-t.C:
            cancel()  // timeout → cancel context → close TCP → body.Read() returns error
        case <-resetCh:
            t.Reset(90s)  // received SSE line → reset timer
        }
    }
}()
```

Every incoming SSE line resets the timer. If no data arrives for 90 seconds, the context is cancelled and the TCP connection is closed, preventing indefinite blocking.

## Related Files

### NOFX (Client)
- `mcp/payment/claw402.go` — Claw402Client entry point
- `mcp/payment/x402.go` — x402 payment flow (DoX402Request, DoX402RequestStream, X402CallStream)
- `mcp/payment/x402_sign.go` — EIP-712 signing implementation
- `mcp/client.go` — ParseSSEStream, CallWithRequestStream

### claw402 (Server)
- `internal/gateway/x402.go` — x402 middleware (streamAwareX402Middleware)
- `internal/gateway/proxy/stream.go` — SSE proxy (HandleAPIKeyStream)
- `internal/config/` — Route configuration (pricing, model mapping)
