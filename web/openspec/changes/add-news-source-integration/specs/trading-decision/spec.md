## MODIFIED Requirements

### Requirement: Full Trading Decision with Context Integration
The system SHALL generate full trading decisions that consider all available context including market data, candidate coins, positions, and optionally market news sentiment.

**Previous behavior:** Decisions based on market data, volume, OI, and candidate coins only.

**Modified behavior:** Decisions now optionally include recent news sentiment and headline context when news integration is enabled.

#### Scenario: Decision with news context enabled
- **WHEN** user has enabled news integration
- **AND** trading decision engine is called
- **THEN** system fetches recent news articles (up to 5 most recent)
- **AND** system calculates sentiment average from articles
- **AND** system includes news summary in the user prompt to AI
- **AND** AI decision incorporates sentiment and headline information
- **AND** decision output includes reasoning that may reference news

#### Scenario: Decision without news context (disabled)
- **WHEN** user has disabled news integration
- **OR** news fetch fails or times out
- **THEN** system skips news fetch entirely
- **AND** decision proceeds with market data only
- **AND** user prompt and AI reasoning reference no news
- **AND** decision quality and format unchanged from before

#### Scenario: News fetch graceful degradation
- **WHEN** Mlion API is unavailable or times out (> 5 seconds)
- **THEN** news enrichment is abandoned (fail-fast)
- **AND** decision continues immediately with available market data
- **AND** decision completes within normal SLA (same as without news)
- **AND** logging records the failure for debugging

#### Scenario: News cache is used for efficiency
- **WHEN** multiple decisions are made within 5 minutes
- **AND** news integration is enabled
- **THEN** only the first decision fetches from Mlion API
- **AND** subsequent decisions reuse cached articles (within TTL)
- **AND** API quota is preserved, performance improved

---

## ADDED Requirements

### Requirement: News Context in Decision Engine
The system SHALL enrich trading decisions with news context that includes articles, sentiment, and categories.

#### Scenario: News context structure
- **WHEN** decision engine builds context for AI
- **THEN** context includes `NewsContext` object with:
  - `articles`: list of recent articles (ID, title, content, sentiment, symbol, time)
  - `sentiment_avg`: average sentiment score (-1.0 to +1.0)
  - `top_categories`: list of news categories (governance, regulation, security, partnership, etc.)
  - `fetched_at`: Unix timestamp of fetch
  - `enabled`: boolean indicating if news integration is active
- **AND** news context is immutable after creation
- **AND** news context always has valid defaults (empty if disabled)

#### Scenario: AI receives news in prompt
- **WHEN** news integration is enabled
- **THEN** user prompt includes dedicated "Latest Market News & Sentiment" section
- **AND** section includes sentiment score and top headlines
- **AND** section uses emojis for sentiment clarity (✅ positive, ➡️ neutral, ⚠️ negative)
- **AND** section includes coin symbols for relevance filtering by AI

#### Scenario: System prompt instructs news consideration
- **WHEN** system prompt is generated for AI
- **THEN** system prompt includes instruction: "Consider recent market sentiment from news articles"
- **AND** instruction is clear and actionable
- **AND** instruction does not override AI's judgment (optional consideration)

---

## ADDED Requirements

### Requirement: News Caching for Performance
The system SHALL cache news articles to avoid redundant API calls and respect rate limits.

#### Scenario: In-memory cache with TTL
- **WHEN** news is fetched from Mlion API
- **THEN** articles are stored in in-memory cache
- **AND** cache entry includes fetch timestamp
- **AND** cache respects TTL setting (default 5 minutes, user-configurable 1-60 minutes)
- **AND** when TTL expires, next fetch queries Mlion API again

#### Scenario: Cache hit within TTL
- **WHEN** news enrichment is called within TTL window
- **THEN** cached articles are returned immediately
- **AND** no API call is made
- **AND** performance is sub-millisecond
- **AND** logging records "cache hit" for observability

#### Scenario: Cache miss (stale or empty)
- **WHEN** cache is empty or TTL expired
- **THEN** system fetches from Mlion API
- **AND** response is cached for TTL duration
- **AND** next call within TTL uses cache

---

## ADDED Requirements

### Requirement: News Fetch Resilience
The system SHALL handle news API failures gracefully without disrupting trading decisions.

#### Scenario: API timeout (> 5 seconds)
- **WHEN** Mlion API does not respond within 5 seconds
- **THEN** request is cancelled (fail-fast)
- **AND** decision continues immediately
- **AND** empty news context is used (news marked as disabled for that decision)
- **AND** error is logged with timestamp and context

#### Scenario: API error response
- **WHEN** Mlion API returns error (5xx, invalid key, rate limit)
- **THEN** error is caught and logged
- **AND** decision proceeds without news
- **AND** user is not notified (graceful degradation)
- **AND** no retry is attempted for this decision

#### Scenario: Network unreachable
- **WHEN** network connection fails during news fetch
- **THEN** error is caught (network error)
- **AND** decision proceeds immediately
- **AND** no retry is attempted
- **AND** error is logged for monitoring

---

## ADDED Requirements

### Requirement: Decision Logging with News Context
The system SHALL log all decision-related data including news context for debugging and analysis.

#### Scenario: Decision log includes news metrics
- **WHEN** decision is made with news enabled
- **THEN** decision log includes:
  - `news_enabled`: boolean
  - `news_articles_count`: number of articles included
  - `news_sentiment_avg`: sentiment score
  - `news_fetch_ms`: milliseconds to fetch
  - `news_cache_hit`: boolean (was cache used?)
- **AND** log includes article headlines for CoT trace
- **AND** log is human-readable and parseable

#### Scenario: Decision log on news fetch failure
- **WHEN** news fetch fails
- **THEN** log includes:
  - `news_error`: error message
  - `news_fetch_ms`: time spent before timeout
  - `fallback`: "market_data_only"
- **AND** decision still proceeds and completes normally

