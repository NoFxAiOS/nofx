# Codebase Audit Report
**Date:** 2025-12-13
**Project:** nofx (AI Trading System)

## 1. Architecture Overview

The project is a **modular Monolith** written in Go, designed for automated cryptocurrency trading driven by AI models (DeepSeek, Qwen).

*   **Core Components:**
    *   **API Server (`api/`):** Exposes endpoints for frontend/management, using `Gin`.
    *   **Trader Manager (`manager/`):** Orchestrates multiple `AutoTrader` instances.
    *   **Auto Trader (`trader/`):** The autonomous agent that executes the trading loop (Scan -> Analyze -> Decide -> Execute).
    *   **Decision Engine (`decision/`):** The "brain" that aggregates market data, constructs prompts, calls AI models, and validates decisions.
    *   **Market Data (`market/`):** Handles real-time data fetching (WebSocket/API) from exchanges.
    *   **Database (`database/`):** Abstraction layer supporting both PostgreSQL (Neon) and SQLite.

*   **Design Patterns:**
    *   **Interface-based Abstraction:** The `Trader` interface allows swapping exchanges (Binance, OKX, Hyperliquid) without changing core logic.
    *   **Dependency Injection:** Configuration and Database instances are passed down to components.
    *   **Event Loop:** The trading logic runs on a `time.Ticker` loop within each `AutoTrader`.

## 2. Code Quality & Standards

*   **Go Idioms:** The code generally follows good Go practices.
    *   Use of `context` for timeouts and cancellation.
    *   `sync.RWMutex` for thread-safe map access in `TraderManager`.
    *   Proper error wrapping (`fmt.Errorf("...: %w", err)`).
*   **Structure:** The package organization (`api`, `config`, `database`, `decision`, `trader`) is logical and separates concerns effectively.
*   **Documentation:** Functions are well-commented, often with Chinese documentation explaining the business logic, which aids maintainability for the target team.
*   **Error Handling:** Most errors are logged. Critical errors (like DB connection failure in startup) cause a fatal exit, which is appropriate. In the trading loop, errors are logged, allowing the system to retry in the next cycle, ensuring resilience.

## 3. Security Audit

*   **Credentials Management:**
    *   **Strengths:** API keys and secrets are loaded from the database or environment variables, not hardcoded in the source logic. `bcrypt` is used for password hashing.
    *   **Weakness:** `syncConfigToDatabase` in `main.go` reads a local `config.json`. If this file contains secrets and is accidentally committed or left on a server, it poses a risk.
    *   **Recommendation:** Ensure `config.json` is strictly git-ignored and restrict file permissions. Prefer environment variables for all secrets in production.
*   **Database:**
    *   **SQL Injection:** Parameterized queries (`$1` / `?`) are used consistently across `database/` and `auth/`. **Risk is Low.**
    *   **Connection:** The logic attempts to connect to Neon (Postgres) and falls back to SQLite.
        *   **Risk:** If the production DB (Neon) is unreachable, the app might silently (log warning only) switch to a local empty SQLite DB, potentially confusing the state.
*   **API Security:**
    *   JWT is used for authentication (`auth/` package).
    *   Input validation relies heavily on `Gin` binding and basic checks.

## 4. Performance Analysis

*   **Concurrency:**
    *   `TraderManager` uses goroutines to fetch data for multiple traders in parallel (`getConcurrentTraderData`), significantly reducing latency for the "Competition/Leaderboard" endpoints.
    *   The `AutoTrader` loop runs sequentially per trader. This is safe and avoids race conditions within a single trader's context.
*   **Database:**
    *   `BatchInsertTradeRecords` is implemented, showing attention to write performance.
    *   Connection pooling is handled by the `sql` driver default.
*   **AI Integration:**
    *   The `Decision` engine makes external HTTP calls to AI providers. This is the primary latency bottleneck.
    *   **Optimization:** The code caches competition data for 30 seconds to avoid slamming the DB/calculations.
*   **Resource Usage:**
    *   Market data is fetched for "Candidate Coins". If the candidate list grows large, this could hit exchange rate limits. The current limit (Top 20) is a reasonable safeguard.

## 5. Potential Defects & Risks

1.  **Dual Database "Silent" Fallback:**
    *   **Issue:** In `database.go`, if Neon fails, it falls back to SQLite. In a production container (like Docker/K8s), the local SQLite DB is ephemeral. If the app restarts and falls back to SQLite, all user data and trading history might appear lost until Neon is back.
    *   **Fix:** In production mode (`GoEnv=production`), disable the automatic fallback to SQLite. Fail hard if the primary DB is unreachable.

2.  **Configuration Race/Overwrite:**
    *   **Issue:** `main.go` syncs `config.json` to the DB on *every* startup. If an old `config.json` exists, it might overwrite newer settings changed via the UI/API that were saved to the DB.
    *   **Fix:** Only sync if the DB is empty or add a version flag.

3.  **Prompt Injection / Hallucination:**
    *   **Issue:** The system relies entirely on the AI's JSON output. While there is a `validateDecision` function, LLMs can sometimes output malformed JSON or subtle logic errors.
    *   **Fix:** The `fixMissingQuotes` function is a good band-aid, but robust JSON repair libraries or retry logic (asking AI to fix its format) would be more reliable.

## 6. Recommendations

1.  **Strict DB Mode:** Add a flag to `NewDatabase` to disable SQLite fallback in production environments.
2.  **Config Management:** Deprecate `config.json` for secrets. Use it only for non-sensitive defaults. Load secrets exclusively from Environment Variables.
3.  **Rate Limiting:** Ensure the `market/` package respects exchange API rate limits, especially when fetching data for the candidate coin list.
4.  **Testing:** The project has `_test.go` files, but a CI pipeline (Github Actions) running `go test ./...` on every push is highly recommended to catch regressions.
