# News Source Integration into Trading Decision System

## Why

Currently, the trading decision engine only considers market data (price, volume, OI) and candidate coins from signal sources. Market sentiment and real-time news are critical signals for informed trading decisions, especially in volatile crypto markets. Integrating the Mlion news API allows traders to:

1. Make decisions based on recent news sentiment and market events
2. Avoid trades during high-volatility news periods
3. Capture momentum from positive sentiment shifts

This enhancement fills a critical gap in the decision context without disrupting existing market-data flows.

## What Changes

- **Add News Source Configuration Panel** in traders page, below signal source configuration
  - Allow users to enable/disable news source integration
  - Configure Mlion API parameters (language, timezone, hot-news filter)

- **Store News Source Configuration** in database alongside signal source config

- **Enhance Trading Decision Context** to include recent news articles
  - Fetch latest news articles from Mlion before decision
  - Include news sentiment summary in user prompt

- **Extend System Prompt** to instruct AI to consider news sentiment when making decisions

- **Add Comprehensive Tests** (unit, integration, e2e) with 100% code coverage for new features

- **No Breaking Changes**: All additions are backward-compatible; existing traders continue working without news integration

## Impact

### Affected Capabilities
- `trading-decision` - Modified to include news context
- `news-source-config` - New capability for configuration management

### Affected Code (Backend)
- `api/handlers/config.go` - Add news source config endpoints
- `config/database.go` - Add database schema for news config
- `decision/engine.go` - Enhance Context struct and decision building
- `decision/prompt_manager.go` - Extend system prompt template with news instructions
- `service/news/mlion.go` - Already exists, will be integrated

### Affected Code (Frontend)
- `web/src/components/AITradersPage.tsx` - Add news configuration panel and state management
- `web/src/utils/api.ts` - Add news config API calls

### Risk Mitigation
- News integration is **optional per trader** - can be disabled
- AI fallback: if news fetch fails, decision continues normally
- Rate limiting on news API (1 call per decision, cached for 5 min)
- No impact on existing traders without news config enabled

### Performance Impact
- Minimal: +1 API call per decision (async fetch)
- News articles cached/re-used within same decision cycle
- Memory footprint: ~100KB per decision for article summaries

## Testing & Quality

- Unit tests for news integration in decision engine
- Integration tests for config persistence
- E2E tests for UI configuration flow
- Mock Mlion API for testing
- Performance test: decision latency within 500ms additional

## Deployment Plan

1. Deploy database schema migration (additive, no data loss)
2. Deploy backend changes with feature flag (news integration disabled by default)
3. Deploy frontend changes (UI available, disabled until enabled in admin)
4. Enable feature flag in production after 24h validation
5. Monitor decision latency and error rates
