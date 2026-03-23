# NOFXi Self-Drive Tasks

## Completed

### 2026-03-23 09:37 — Security Hardening Batch
- [DONE] Fix health endpoint returning `"time": null` → now returns proper RFC3339 timestamp
- [DONE] Add login rate limiting (5 attempts / 15min, 5min block) to prevent brute-force
- [DONE] Move `/api/reset-password` behind auth middleware (was unauthenticated — critical vuln)
- [DONE] Add response body size limits on proxy/external API calls (prevent memory exhaustion)
- [DONE] Sanitize error messages in agent chat endpoint (was leaking internal error details)
- [DONE] Record login success/failure for rate limiter tracking
- [DONE] Sanitize markdown link URLs in frontend (prevent javascript: XSS)

### 2026-03-23 09:52 — CORS & Response Limits
- [DONE] Add configurable CORS origins (`CORS_ALLOWED_ORIGINS` env var) — replaces wildcard `*`
- [DONE] Add `io.LimitReader` to sentinel.go (was unbounded, now 256KB)
- [DONE] Fix news `seen` map eviction: keep half instead of clearing all (was losing dedup state)
- [DONE] Agent chat endpoint already behind auth middleware (verified, was falsely flagged as pending)

### 2026-03-23 10:07 — Context Propagation, Component Extraction, Security & Reliability
- [DONE] Propagate `c.Request.Context()` in all kline API handlers (was `context.Background()`)
  — client disconnects now cancel upstream calls to CoinAnk, Alpaca, TwelveData, Hyperliquid
- [DONE] Extract `MessageRenderer.tsx` from `AgentChatPage.tsx` (1009 → 825 lines)
  — renderMessageContent + renderInline moved to `components/agent/MessageRenderer.tsx`
- [DONE] Remove `/api/crypto/decrypt` from public routes — was a security hole allowing
  anyone to decrypt ciphertext without authentication (internal callers use service directly)
- [DONE] Add `safe.Go` / `safe.GoNamed` panic recovery wrapper (`safe/go.go`)
  — 31 goroutines had zero `recover()` calls; a single panic would crash the entire process
- [DONE] Apply `safe.GoNamed` to all trader launch goroutines (StartAll, RestoreRunning,
  LoadSingleTrader auto-start, API start/restart handlers)

## Pending

### 2026-03-23 10:22 — Complete Panic Recovery Coverage
- [DONE] Apply `safe.GoNamed` to all 27 remaining bare goroutines across 21 files:
  — 9 exchange order_sync (OKX, Hyperliquid, Aster, Bybit, KuCoin, Gate, Bitget, Lighter, Binance×2)
  — Drawdown monitor, Brain news+briefs, Sentinel, Scheduler
  — x402 + MCP stream idle watchdogs, Rate limiter cleanup
  — 3 telemetry sends, CoinAnk WS handler, API server goroutine
  — Manual defer/recover for Telegram bot (sends error to user) and trader data fetch (prevents deadlock)
- [DONE] Zero bare `go func()` remaining outside safe.go itself
- Build verified ✅, pushed to feat/nofxi

## Pending

### Security
- [PENDING] Investigate GitHub Dependabot's 21 reported vulnerabilities (13 high, 7 moderate, 1 low)

### Code Quality
- [PENDING] `context.Background()` used in ~69 exchange/trader calls — should propagate request context for proper cancellation (partially done: kline handlers fixed, trader/exchange calls remain)
- [PENDING] `AgentChatPage.tsx` is 825 lines — could still extract WelcomeScreen and ChatMessages components

### Performance
- [PENDING] `gatherContext` in agent.go iterates all traders and positions on every message — consider caching (low priority: only triggered per user message)

### Features
- [PENDING] Agent chat has fake streaming (word-by-word setTimeout) — implement real SSE streaming
- [PENDING] Add WebSocket support for real-time position/balance updates instead of polling
