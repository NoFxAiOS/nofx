# Implementation Tasks: News Source Integration (Architect-Reviewed & Fixed)

## 1. Backend Core Implementation

### 1.1 Data Structures & Interfaces (Arch Fix #1: Extensible Design)
- [ ] 1.1.1 Create `decision/news_context.go` with `NewsContext`, `Article` structs
- [ ] 1.1.2 Create `service/news/cache.go` with `NewsCache` interface (with single-flight support)
- [ ] 1.1.3 Implement `SafeInMemoryNewsCache` with `GetOrFetch()` and single-flight pattern
- [ ] 1.1.4 Create `decision/context_enricher.go` with `ContextEnricher` interface
- [ ] 1.1.5 Create `decision/news_enricher.go` implementing `ContextEnricher`
- [ ] 1.1.6 Add `Extensions map[string]interface{}` field to `decision/engine.go:Context` struct
- [ ] 1.1.7 Add `SetExtension()`, `GetExtension()` methods to Context
- [ ] 1.1.8 Unit tests: `decision/news_context_test.go` (>=95% branch coverage)
- [ ] 1.1.9 Unit tests: `service/news/cache_test.go` (single-flight concurrency test)
- [ ] 1.1.10 Unit tests: `decision/context_enricher_test.go` (interface implementation)

### 1.2 Circuit Breaker Implementation (Arch Fix #3: Cascade Failure Prevention)
- [ ] 1.2.1 Create `service/news/circuit_breaker.go` with `CircuitBreaker` struct
- [ ] 1.2.2 Implement states: closed, open, half-open (with atomic operations)
- [ ] 1.2.3 Threshold: 3 consecutive failures → open circuit
- [ ] 1.2.4 Cooldown: 60 seconds before half-open recovery attempt
- [ ] 1.2.5 Logging: circuit state changes (🔴 open, 🟢 closed, 🟡 half-open)
- [ ] 1.2.6 Unit tests: state transitions, fast-fail on open
- [ ] 1.2.7 Load test: 100 concurrent calls with circuit open (verify microsecond fail)

### 1.3 Prompt Injection Defense (Arch Fix #2: Security Critical)
- [ ] 1.3.1 Create `decision/prompt_sanitizer.go` with `sanitizeForPrompt()`
- [ ] 1.3.2 Implement control character removal (prevent hidden Unicode injection)
- [ ] 1.3.3 Implement Markdown special char escaping (`#`, `---`, `[`, `]`, etc.)
- [ ] 1.3.4 Implement newline filtering (prevent prompt section injection)
- [ ] 1.3.5 Implement truncation to 200 chars per headline
- [ ] 1.3.6 Unit tests: `decision/prompt_sanitizer_test.go`
  - [ ] 1.3.6a Test injection: "\\n---\\n# SYSTEM OVERRIDE"
  - [ ] 1.3.6b Test Unicode: zero-width characters, right-to-left marks
  - [ ] 1.3.6c Test Markdown: backticks, pipes, brackets
  - [ ] 1.3.6d Test length: truncate > 200 chars
  - [ ] 1.3.6e Test malicious: command-like sequences

### 1.4 News Enrichment with Protections
- [ ] 1.4.1 Create `decision/news_enricher.go` integrating circuit breaker + cache + sanitizer
- [ ] 1.4.2 Implement `NewsEnricher.Enrich()` with try-catch-degrade pattern
- [ ] 1.4.3 Use circuit breaker to guard Mlion API calls
- [ ] 1.4.4 Use cache with single-flight for concurrent access safety
- [ ] 1.4.5 Sanitize all article fields before storing
- [ ] 1.4.6 Implement graceful degradation: any failure → return disabled NewsContext
- [ ] 1.4.7 Add detailed logging: fetch time (ms), cache hit, breaker state
- [ ] 1.4.8 Unit tests: `decision/news_enricher_test.go`
  - [ ] 1.4.8a Test happy path: fetch + cache + enrich
  - [ ] 1.4.8b Test timeout: breaker opens after 3 failures
  - [ ] 1.4.8c Test concurrent access: single flight prevents N API calls
  - [ ] 1.4.8d Test degradation: failure doesn't throw, returns empty context

### 1.5 Prompt Engineering with Injection Defense
- [ ] 1.5.1 Create `decision/news_prompt_builder.go` with `buildNewsPromptSection()`
- [ ] 1.5.2 Use `sanitizeForPrompt()` on all article fields (title, symbol, etc.)
- [ ] 1.5.3 Format: sentiment emoji (✅/➡️/⚠️), symbol, sanitized headline
- [ ] 1.5.4 Add "read-only information" annotation (set AI mindset)
- [ ] 1.5.5 Limit to 5 headlines max per decision
- [ ] 1.5.6 Update system prompt to: "Consider market sentiment from news (informational only)"
- [ ] 1.5.7 Unit tests: `decision/news_prompt_builder_test.go`
  - [ ] 1.5.7a Test empty news → empty section (no garbage)
  - [ ] 1.5.7b Test sanitization: malicious articles cleaned properly
  - [ ] 1.5.7c Test format: correct emoji/symbol/headline structure

### 1.6 Decision Engine Integration with Enricher Chain
- [ ] 1.6.1 Modify `GetFullDecisionWithCustomPrompt()` to use enricher chain
- [ ] 1.6.2 Create enricher list: initialize with `NewsEnricher` (as first instance)
- [ ] 1.6.3 Call each enricher in sequence; non-fatal errors don't stop decision
- [ ] 1.6.4 Ensure `GetExtension("news")` used to fetch data for prompt builders
- [ ] 1.6.5 Remove direct `ctx.NewsContext` field access (use extensions map only)
- [ ] 1.6.6 Integration test: `decision/decision_with_enrichers_test.go`
  - [ ] 1.6.6a Test enrichment chain runs
  - [ ] 1.6.6b Test failure isolation: news enricher fail doesn't affect decision
  - [ ] 1.6.6c Test backward compat: news disabled → decision unchanged
- [ ] 1.6.7 Performance test
  - [ ] 1.6.7a P50 latency: < 100ms (cache hit)
  - [ ] 1.6.7b P95 latency: < 500ms (cache miss + network)
  - [ ] 1.6.7c P99 latency: < 6s (timeout scenario)

### 1.7 Database Schema & Persistence (Arch Fix #1: Dedicated Table)
- [ ] 1.7.1 Create migration file: `config/migrations/YYYYMMDD_create_user_news_config.sql`
- [ ] 1.7.2 Migration creates dedicated `user_news_config` table:
  - `user_id` (PRIMARY KEY, FOREIGN KEY to users)
  - `enabled` (BOOLEAN DEFAULT false)
  - `cache_ttl_minutes` (INT DEFAULT 5)
  - `language` (VARCHAR(10) DEFAULT 'en') ← Pre-positioned for future multi-language
  - `created_at`, `updated_at` (TIMESTAMPS)
- [ ] 1.7.3 Create `NewsConfig` struct in `config/database.go`
- [ ] 1.7.4 Implement `GetUserNewsConfig(userID)` method
- [ ] 1.7.5 Implement `SaveUserNewsConfig(userID, config)` method
- [ ] 1.7.6 Unit tests: `config/database_news_test.go`
  - [ ] 1.7.6a Test CRUD: create, read, update, delete operations
  - [ ] 1.7.6b Test default values: new user gets sensible defaults
  - [ ] 1.7.6c Test migration: schema created correctly
- [ ] 1.7.7 Integration test: persistence end-to-end with test DB

### 1.8 API Endpoints (Arch Fix #4: Correct Path)
- [ ] 1.8.1 Create `HandleGetUserNewsConfig()` in `api/handlers/config.go`
  - Endpoint: `GET /api/user/news-config`
  - Returns: `{enabled: bool, cache_ttl_minutes: int, status: string}`
- [ ] 1.8.2 Create `HandleSaveUserNewsConfig()` in `api/handlers/config.go`
  - Endpoint: `POST /api/user/news-config`
  - Request: `{enabled: bool, cache_ttl_minutes: int}`
- [ ] 1.8.3 Input validation: `cache_ttl_minutes` in range [1, 60]
- [ ] 1.8.4 Error handling: return 400 on invalid input, 500 on DB error
- [ ] 1.8.5 Unit tests: `api/handlers/config_news_test.go`
  - [ ] 1.8.5a Test validation: reject TTL < 1 or > 60
  - [ ] 1.8.5b Test persistence: save → get returns same values
  - [ ] 1.8.5c Test auth: require authenticated user
- [ ] 1.8.6 Integration test: API calls work end-to-end with test DB

### 1.9 Feature Flag & Configuration
- [ ] 1.9.1 Add `news_source_enabled` BOOLEAN field to system_config table (default false)
- [ ] 1.9.2 Create `IsNewsSourceEnabled()` helper function in decision engine
- [ ] 1.9.3 Check global flag in `NewsEnricher.IsEnabled()` method
- [ ] 1.9.4 Log per-decision: "[NEWS ENABLED]" or "[NEWS DISABLED]"
- [ ] 1.9.5 Unit test: feature flag on/off behavior

### 1.10 Structured Logging & Metrics
- [ ] 1.10.1 Use structured logging (zap or logrus, not fmt.Printf)
- [ ] 1.10.2 Log per decision
  - `news_enabled`: boolean
  - `articles_count`: int
  - `sentiment_avg`: float64
  - `cache_hit`: boolean
  - `fetch_duration_ms`: int64
  - `circuit_breaker_state`: string
- [ ] 1.10.3 Expose Prometheus metrics:
  - `news_fetch_duration_seconds` (histogram)
  - `news_cache_hits_total` (counter)
  - `news_api_errors_total` (counter)
  - `news_circuit_breaker_state` (gauge: 0=closed, 1=open, 2=half-open)
- [ ] 1.10.4 Unit test: log output format validation

---

## 2. Frontend Implementation

### 2.1 News Configuration Component
- [ ] 2.1.1 Create `web/src/components/NewsSourceModal.tsx`
- [ ] 2.1.2 Component shows: enabled toggle, TTL slider (1-60), save button
- [ ] 2.1.3 Input validation: TTL in range [1, 60] minutes, prevent invalid input
- [ ] 2.1.4 Styling: match signal source modal (dark theme, Binance colors, #1E2329)
- [ ] 2.1.5 Translations: add keys to `translation.json`
  - `newsSourceConfig`, `newsSourceDesc`, `cacheTTL`, `newsEnabled`
  - Both EN and ZH
- [ ] 2.1.6 Unit test: `__tests__/NewsSourceModal.test.tsx`
  - [ ] 2.1.6a Render: component displays all fields
  - [ ] 2.1.6b State changes: toggle and slider work
  - [ ] 2.1.6c Validation: reject invalid TTL values

### 2.2 Integration into Traders Page
- [ ] 2.2.1 Modify `AITradersPage.tsx`:
  - Add state: `showNewsSourceModal`, `userNewsConfig`
  - Add button in config section: "📰 News Source Config"
- [ ] 2.2.2 Position below signal source modal (consistent layout)
- [ ] 2.2.3 Button shows enabled/disabled status
- [ ] 2.2.4 Click → open `NewsSourceModal`
- [ ] 2.2.5 Test: modal opens/closes, state synced

### 2.3 API Integration
- [ ] 2.3.1 Add to `web/src/utils/api.ts`:
  - `getUserNewsConfig()` - GET `/api/user/news-config`
  - `saveUserNewsConfig(enabled, ttl)` - POST `/api/user/news-config`
- [ ] 2.3.2 Add loading states during API calls (disable button, show spinner)
- [ ] 2.3.3 Error handling: show toast on failure, provide retry button
- [ ] 2.3.4 Integration test: `__tests__/api.news.integration.test.ts`

### 2.4 UI State Management
- [ ] 2.4.1 Load config on page mount (via `useEffect`)
- [ ] 2.4.2 Display current settings in modal
- [ ] 2.4.3 Show saving indicator during API call
- [ ] 2.4.4 Disable save button until user makes changes
- [ ] 2.4.5 Test: state sync, loading states, error handling

### 2.5 Responsiveness & Accessibility
- [ ] 2.5.1 Modal responsive on mobile (< 768px width)
- [ ] 2.5.2 Keyboard navigation: Tab through inputs, Enter to save, Esc to close
- [ ] 2.5.3 ARIA labels on inputs and buttons
- [ ] 2.5.4 Focus trap inside modal
- [ ] 2.5.5 Test: accessibility audit, keyboard navigation test

### 2.6 E2E Testing
- [ ] 2.6.1 Create `web/e2e/news-config.e2e.ts`
- [ ] 2.6.2 Test happy path:
  - Click "News Config" button
  - Modal appears
  - Toggle enabled checkbox
  - Change TTL slider
  - Click save
  - Verify success message
  - Refresh page → settings persist
- [ ] 2.6.3 Test error handling: failed save → show error → retry succeeds
- [ ] 2.6.4 Test state consistency after changes

---

## 3. Testing & Quality Assurance

### 3.1 Unit Test Coverage (Target: >=95% branch coverage)
- [ ] 3.1.1 Backend unit tests: `decision/`, `service/news/`, `config/`
- [ ] 3.1.2 Frontend unit tests: components, utils (target >=90%)
- [ ] 3.1.3 Test both happy path and error cases
- [ ] 3.1.4 Mock external dependencies: Mlion API, DB, HTTP
- [ ] 3.1.5 Generate coverage reports: `go test ./... -covermode=atomic`
- [ ] 3.1.6 Generate coverage reports: `npm test -- --coverage --collectCoverageFrom`
- [ ] 3.1.7 CI gate: block PRs if coverage < targets

### 3.2 Integration Tests
- [ ] 3.2.1 Test news enrichment with mock Mlion API
- [ ] 3.2.2 Test decision flow including enrichment chain
- [ ] 3.2.3 Test API endpoints with test database
- [ ] 3.2.4 Test cache behavior: TTL expiry, single-flight concurrency
- [ ] 3.2.5 Test circuit breaker: state transitions, fast-fail
- [ ] 3.2.6 Test feature flag: enabled/disabled states
- [ ] 3.2.7 All tests pass on CI/CD pipeline

### 3.3 Performance Testing
- [ ] 3.3.1 Benchmark: cache hit lookup (target: <1ms)
- [ ] 3.3.2 Benchmark: news fetch latency (target: <300ms)
- [ ] 3.3.3 Benchmark: prompt building (target: <50ms)
- [ ] 3.3.4 Full decision latency:
  - P50 < 100ms (cache hit scenario)
  - P95 < 500ms (cache miss, network latency)
  - P99 < 6s (timeout scenarios)
- [ ] 3.3.5 Load test: 100 concurrent decisions with news enrichment

### 3.4 Security Testing (Additional to 3.1)
- [ ] 3.4.1 Prompt injection test cases:
  - Headline with "\\n---\\n# OVERRIDE"
  - Headline with zero-width Unicode
  - Headline with Markdown formatting
  - Headline > 1000 chars
- [ ] 3.4.2 Database security: test encrypted storage if added
- [ ] 3.4.3 API security: test authentication, authorization, rate limiting
- [ ] 3.4.4 Third-party data trust: verify all external data is sanitized

### 3.5 Backward Compatibility Testing
- [ ] 3.5.1 Existing traders without news config continue working
- [ ] 3.5.2 Existing decisions unchanged (news disabled by default)
- [ ] 3.5.3 Migration: old data schema not corrupted, new users get defaults
- [ ] 3.5.4 Rollback: disable flag → system works without news, no crashes

### 3.6 Manual Testing Checklist
- [ ] 3.6.1 Enable news config → decision includes news sentiment
- [ ] 3.6.2 Disable news config → decision unchanged, no news in prompt
- [ ] 3.6.3 Mlion API down → decision continues with market data (degrades gracefully)
- [ ] 3.6.4 News fetch timeout (>5s) → decision completes within SLA
- [ ] 3.6.5 Cache works: same news used for multiple decisions (within 5 min TTL)
- [ ] 3.6.6 Circuit breaker: 3 failures → circuit opens → fast fail on subsequent calls
- [ ] 3.6.7 Circuit recovery: wait 60s → half-open retry succeeds → circuit closes

---

## 4. Documentation & Rollout

### 4.1 Code Documentation
- [ ] 4.1.1 Add godoc comments to all public functions/types
- [ ] 4.1.2 Document `NewsContext`: fields, semantics, lifecycle
- [ ] 4.1.3 Document `ContextEnricher` interface: contract, implementation guide
- [ ] 4.1.4 Document `CircuitBreaker`: states, failure scenarios, recovery
- [ ] 4.1.5 Document API endpoints: request/response format, errors, examples
- [ ] 4.1.6 Update README.md: news integration feature overview

### 4.2 Configuration Documentation
- [ ] 4.2.1 Document system config: `news_source_enabled` flag, when to use
- [ ] 4.2.2 Document user config: `news_enabled`, `cache_ttl_minutes`, defaults
- [ ] 4.2.3 Database schema: migration steps, rollback procedure
- [ ] 4.2.4 Provide deployment checklist (feature flag, validation, monitoring)

### 4.3 Deployment Guide
- [ ] 4.3.1 Pre-deployment: database backup, rollback procedure documented
- [ ] 4.3.2 Deployment steps:
  - Run migration: create `user_news_config` table
  - Deploy backend code (feature flag disabled by default)
  - Deploy frontend code
  - Run sanity tests
- [ ] 4.3.3 Staging validation (24h):
  - News integration disabled by default
  - Run 10% of traders with news enabled
  - Monitor latency, error rates, news API quota
  - If issues, rollback by disabling feature flag
- [ ] 4.3.4 Monitoring: key metrics to watch (latency p99, error rate, circuit breaker state)

### 4.4 Gradual Rollout Plan
- [ ] 4.4.1 Day 1: Enable feature flag for 5% of traders
- [ ] 4.4.2 Day 2: Monitor metrics, gradually increase to 25%
- [ ] 4.4.3 Day 3-4: Monitor metrics, gradually increase to 100%
- [ ] 4.4.4 Rollback plan: disable flag immediately if issues detected
- [ ] 4.4.5 Post-rollout: monitor latency, error rates, news API quota for 1 week

---

## Validation Checklist (Before Release)

**Code Quality:**
- [ ] All unit tests pass with >=95% coverage (backend), >=90% (frontend)
- [ ] All integration tests pass
- [ ] All E2E tests pass on staging
- [ ] Code review approved (architecture consistency, SOLID principles)
- [ ] Security review passed (prompt injection prevention, input validation)

**Functionality:**
- [ ] News integration gracefully degrades on Mlion API errors (returns empty context)
- [ ] Circuit breaker prevents cascade failures (fast fail when Mlion down)
- [ ] Cache single-flight prevents stampede (100 concurrent → 1 API call)
- [ ] News enrichment is optional per trader (can be disabled)
- [ ] No breaking changes to existing traders/decisions

**Performance:**
- [ ] Performance benchmarks within targets:
  - Cache hit: P50 < 100ms, P95 < 500ms
  - Full decision latency: P99 < 6s
  - Load test: 100 concurrent decisions succeed
- [ ] Decision latency not degraded when news disabled

**Database:**
- [ ] Migration tested and reversible (can rollback)
- [ ] New users get sensible defaults for news config
- [ ] Existing users can migrate without data loss
- [ ] Schema design: dedicated table (not mixed with signal sources)

**Deployment:**
- [ ] Feature flag works (on/off toggling verified)
- [ ] Staging validation: 24h with 5-10% traffic passed
- [ ] Monitoring dashboards created (latency, errors, circuit breaker state)
- [ ] Rollback procedure documented and tested

---

## Summary of Architect-Identified Fixes Applied

| Issue | Status | Fix |
|-------|--------|-----|
| **#1 Schema mixing** | ✅ FIXED | Dedicated `user_news_config` table |
| **#2 Prompt injection** | ✅ FIXED | `sanitizeForPrompt()` with escaping + truncation |
| **#3 Cascade failures** | ✅ FIXED | Circuit breaker with 3-fail threshold + 60s cooldown |
| **#4 Cache stampede** | ✅ FIXED | Single-flight pattern in `SafeInMemoryCache` |
| **#5 Context extension** | ✅ FIXED | `ContextEnricher` interface + `Extensions` map |
| **#6 API paths** | ✅ FIXED | User-level `/api/user/news-config` endpoints |
| **#7 Performance metrics** | ✅ FIXED | P50/P95/P99 targets defined, P99 < 6s |
| **#8 Structured logging** | ✅ FIXED | Prometheus metrics + structured logging |
| **#9 Test edge cases** | ✅ FIXED | Injection, Unicode, concurrency tests added |
| **#10 Multi-language** | ✅ FIXED | `language` field pre-added to schema |

---

**Total Estimated Tasks:** 100+
**Completion requires all above tasks checked ✅ before release**
