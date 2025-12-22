## ADDED Requirements

### Requirement: News Source Configuration
The system SHALL allow users to enable/disable integration of real-time market news from the Mlion API into their trading decisions.

#### Scenario: User enables news integration
- **WHEN** user navigates to traders page and clicks "📰 News Source Config" button
- **THEN** news configuration modal appears showing current settings
- **AND** user sees toggle for "News Integration Enabled"
- **AND** user sees slider for "Cache TTL" (range 1-60 minutes, default 5)
- **AND** user can modify settings and click "Save"

#### Scenario: Configuration persists
- **WHEN** user enables news integration and clicks "Save"
- **THEN** configuration is stored in database
- **AND** subsequent page loads show the saved settings
- **AND** all traders using this user account inherit the setting (or per-trader override if enabled)

#### Scenario: News integration is optional
- **WHEN** news configuration is disabled
- **THEN** trading decisions proceed normally using only market data
- **AND** no news fetch is attempted
- **AND** performance is identical to decisions without news (no penalty)

#### Scenario: Feature can be globally toggled
- **WHEN** system administrator sets `news_source_enabled = false` in system config
- **THEN** news enrichment is skipped for all traders regardless of user setting
- **AND** feature flag allows safe rollback without data loss
- **AND** can be re-enabled by setting `news_source_enabled = true`

---

## ADDED Requirements

### Requirement: News Source API Configuration Endpoints
The system SHALL provide HTTP endpoints for users to get and save their news source configuration.

#### Scenario: Get user's news configuration
- **WHEN** client calls `GET /api/trader/config/news`
- **THEN** server returns `{news_enabled: boolean, cache_ttl_minutes: int}`
- **AND** response code is 200 OK
- **AND** user must be authenticated

#### Scenario: Save user's news configuration
- **WHEN** client calls `POST /api/trader/config/news` with body `{news_enabled: boolean, cache_ttl_minutes: int}`
- **THEN** server validates input: `cache_ttl_minutes` in range [1, 60]
- **AND** server stores config in `user_signal_sources` table
- **AND** response code is 200 OK with confirmation
- **AND** subsequent GET returns the saved values

#### Scenario: Validation failure on invalid TTL
- **WHEN** client sends `cache_ttl_minutes: 0` or `cache_ttl_minutes: 120`
- **THEN** server returns 400 Bad Request
- **AND** error message specifies valid range: "TTL must be between 1 and 60 minutes"
- **AND** no changes are persisted

---

## ADDED Requirements

### Requirement: News Source Schema Extension
The system SHALL store news source configuration in the database alongside existing signal source configuration.

#### Scenario: Schema extension is additive
- **WHEN** migration is applied to existing database
- **THEN** three columns are added to `user_signal_sources` table:
  - `news_enabled` (BOOLEAN, default false)
  - `news_cache_ttl_minutes` (INT, default 5)
- **AND** existing data remains unchanged
- **AND** migration is reversible (can rollback if needed)

#### Scenario: Backward compatibility
- **WHEN** user has not configured news source
- **THEN** `news_enabled` defaults to false
- **AND** `news_cache_ttl_minutes` defaults to 5
- **AND** old traders without config continue working as before

