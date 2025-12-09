# Feature Proposal: Telegram Topic-Based News Routing

**Date:** December 9, 2025
**Status:** Proposed
**Author:** Claude Code
**Priority:** Medium
**Complexity:** Low-Medium

---

## 1. Executive Summary

Currently, the Telegram news notification service sends all news to a single configurable topic. The requirement is to **route news to a specific topic** (`https://t.me/monnaire_capital_research/2`) while maintaining:
- ✅ No impact on other features
- ✅ Clean, maintainable code (KISS principle)
- ✅ High cohesion, low coupling
- ✅ Comprehensive test coverage

---

## 2. Problem Statement

### Current Behavior
- All news articles are sent to a single `telegram_message_thread_id` configured in system config
- No routing logic based on article type, category, or destination
- Tight coupling between news processing and Telegram notification details

### Requirements
1. **Specific Destination**: Route news to topic ID `2` in `monnaire_capital_research` group
2. **No Side Effects**: Other features (trading, system config, etc.) must remain unaffected
3. **Code Quality**:
   - Follow KISS principle (simple, clear, minimal complexity)
   - Maintain high cohesion within the news service
   - Keep low coupling with other services
4. **Testing**: Unit tests validate routing logic and all edge cases

---

## 3. Design Approach

### 3.1 Architecture Decision

**Pattern: Notification Route Adapter**

Instead of hardcoding the topic ID, introduce a lightweight abstraction:

```
┌─────────────────────┐
│  News Service       │
│                     │
│  • Fetches news     │
│  • Processes AI     │
│  • Publishes via    │
│    Route Adapter    │
└──────────┬──────────┘
           │
           ▼
┌──────────────────────────┐
│  NotificationRoute       │
│  (Interface)             │
│                          │
│  Send(msg, metadata)     │
└──────────┬───────────────┘
           │
    ┌──────┴──────┐
    ▼             ▼
┌────────┐   ┌──────────────┐
│ Telegram│  │ Future Routes│
│ (Topic2)│  │ (Email, etc) │
└────────┘   └──────────────┘
```

### 3.2 Implementation Strategy

#### Phase 1: Introduce Route Interface
- Create `service/news/route.go` with `NotificationRoute` interface
- Implement `TelegramTopicRoute` for specific topic routing
- Keep it **single-responsibility**: only handle destination logic

#### Phase 2: Refactor News Service
- Remove direct dependency on hardcoded topic ID
- Inject `NotificationRoute` into Service
- News service only knows about routing interface, not implementation details

#### Phase 3: Configuration Management
- Simplify config: `telegram_news_route_type` = "topic"
- Store topic specifics in config: `telegram_topic_id`, `telegram_chat_id`
- Future-proof for other route types (email, webhook, etc.)

---

## 4. Detailed Design

### 4.1 New Interface: `NotificationRoute`

**File:** `service/news/route.go`

```go
package news

// NotificationRoute defines how to route a news notification
type NotificationRoute interface {
	// Send sends the notification through this route
	// Returns error if sending fails
	Send(ctx context.Context, message string, metadata *NotificationMetadata) error

	// Type returns the route type for logging and monitoring
	Type() string
}

// NotificationMetadata carries optional metadata about the notification
type NotificationMetadata struct {
	ArticleID    int64  // For deduplication tracking
	Category     string // crypto, general, etc.
	Source       string // Reuters, Bloomberg, etc.
	Sentiment    string // POSITIVE, NEGATIVE, NEUTRAL
	IsAIProcessed bool  // Whether AI processing was applied
}

// TelegramTopicRoute sends messages to a specific Telegram topic
type TelegramTopicRoute struct {
	client   *http.Client
	botToken string
	chatID   string
	topicID  int
	logger   Logger // For structured logging
}

// NewTelegramTopicRoute creates a route for Telegram topic messaging
func NewTelegramTopicRoute(botToken, chatID string, topicID int) *TelegramTopicRoute {
	return &TelegramTopicRoute{
		botToken: botToken,
		chatID:   chatID,
		topicID:  topicID,
		client:   &http.Client{Timeout: 10 * time.Second},
		logger:   NewLogger("TelegramTopicRoute"),
	}
}

func (t *TelegramTopicRoute) Type() string {
	return "telegram_topic"
}

// Send implements NotificationRoute
func (t *TelegramTopicRoute) Send(ctx context.Context, message string, metadata *NotificationMetadata) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	payload := map[string]interface{}{
		"chat_id":                  t.chatID,
		"text":                     message,
		"parse_mode":               "HTML",
		"disable_web_page_preview": false,
		"message_thread_id":        t.topicID,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
	}

	t.logger.Debugf("Message sent to Telegram topic %d (article_id=%d, category=%s)",
		t.topicID, metadata.ArticleID, metadata.Category)

	return nil
}
```

### 4.2 Refactored News Service

**File:** `service/news/service.go` (changes)

```go
// Service 新闻服务
type Service struct {
	store         StateStore
	fetcher       Fetcher
	route         NotificationRoute  // ✨ NEW: Route abstraction instead of direct notifier
	aiProcessor   AIProcessor
	enabled       bool
	sentArticleIDs map[int64]bool    // Global deduplication
}

// NewService creates news service with dependency injection
func NewService(store StateStore, route NotificationRoute) *Service {
	return &Service{
		store:          store,
		route:          route,  // ✨ Injected dependency
		sentArticleIDs: make(map[int64]bool),
	}
}

// ProcessCategory processes news for a category
func (s *Service) ProcessCategory(category string) error {
	// ... existing logic ...

	// When sending, use the route:
	metadata := &NotificationMetadata{
		ArticleID:     int64(a.ID),
		Category:      category,
		Source:        a.Source,
		Sentiment:     a.Sentiment,
		IsAIProcessed: a.AIProcessed,
	}

	if err := s.route.Send(ctx, msg, metadata); err != nil {
		log.Printf("❌ Send failed: %v", err)
		continue
	}

	// ... rest of logic ...
}
```

### 4.3 Configuration Changes

**File:** `config/database.go` (in `initDefaultData`)

```go
// Initialize system config with new route settings
systemConfigs := map[string]string{
	// ... existing configs ...

	// ✨ NEW: Telegram route configuration
	"telegram_news_route_type":    "topic",        // Route type: "topic", "webhook", etc.
	"telegram_chat_id":            "-1002678075016", // Group ID
	"telegram_topic_id":           "2",             // Specific topic/thread ID
	"telegram_bot_token":          "...",

	// DEPRECATED but kept for backward compatibility:
	// "telegram_message_thread_id": "2",
}
```

---

## 5. Benefits

| Aspect | Benefit |
|--------|---------|
| **Maintainability** | Single-responsibility interface; easy to understand routing logic |
| **Extensibility** | New route types (email, webhook, Slack) need only implement interface |
| **Testability** | Mock route in tests; test notification logic independently from Telegram API |
| **No Side Effects** | News service doesn't directly access config; routing is dependency-injected |
| **KISS Principle** | Simple, focused interfaces; no unnecessary abstractions |

---

## 6. Implementation Phases

### Phase 1: Foundation (1-2 hours)
- [ ] Create `service/news/route.go` with interfaces
- [ ] Implement `TelegramTopicRoute`
- [ ] Add comprehensive tests for route implementations
- [ ] **Effort:** Low | **Risk:** Low

### Phase 2: Integration (1-2 hours)
- [ ] Update `Service` to use route injection
- [ ] Refactor `ProcessCategory` to pass metadata
- [ ] Update service initialization in `main.go` and tests
- [ ] **Effort:** Medium | **Risk:** Low

### Phase 3: Cleanup (30 mins)
- [ ] Update configuration defaults
- [ ] Remove old direct notifier usage
- [ ] Document new routing system
- [ ] **Effort:** Low | **Risk:** Very Low

### Phase 4: Testing & Validation (2-3 hours)
- [ ] Unit tests for `TelegramTopicRoute`
- [ ] Integration tests for news → route flow
- [ ] Test cross-category deduplication with routes
- [ ] **Effort:** Medium | **Risk:** Low

---

## 7. Testing Strategy

### Unit Tests: Route Layer
```go
// tests/telegram_topic_route_test.go
func TestTelegramTopicRoute_Send(t *testing.T)           // Happy path
func TestTelegramTopicRoute_Send_APIError(t *testing.T)  // API failures
func TestTelegramTopicRoute_Send_Timeout(t *testing.T)   // Network timeout
func TestTelegramTopicRoute_Type(t *testing.T)           // Type identification
```

### Integration Tests: News + Route
```go
// service/news/service_route_test.go
func TestService_RouteMetadata(t *testing.T)             // Metadata correctness
func TestService_RouteFailure_Fallback(t *testing.T)     // Graceful degradation
func TestService_MultiCategory_SameRoute(t *testing.T)   // Routing consistency
```

### E2E Test (Manual)
```bash
# Send test article to configured route
curl -X POST http://localhost:8080/api/news/test \
  -H "Content-Type: application/json" \
  -d '{"headline":"Test News","category":"crypto"}'

# Verify message appears in https://t.me/monnaire_capital_research/2
```

---

## 8. Migration Path (Zero Downtime)

1. **Deploy new code** with route support alongside old notifier
2. **Gradually switch** traffic to new route via feature flag
3. **Monitor** both systems for 1-2 cycles
4. **Deprecate** old notifier after validation
5. **Cleanup** old code in next sprint

---

## 9. Code Quality Checklist

- ✅ **KISS**: Single-responsibility interface, clear naming
- ✅ **Cohesion**: Route logic isolated in own module
- ✅ **Coupling**: News service depends only on interface, not implementation
- ✅ **Testing**: >90% coverage for route and integration
- ✅ **Documentation**: Clear interface contracts, usage examples
- ✅ **Error Handling**: Specific error types, context propagation
- ✅ **Logging**: Structured logging at route boundary
- ✅ **Backward Compatibility**: Old config still works initially

---

## 10. Success Criteria

1. ✅ News only sent to topic ID `2` in target group
2. ✅ No breaking changes to other features
3. ✅ Code passes all existing + new tests
4. ✅ Route implementation <100 lines of code
5. ✅ Zero configuration needed for KISS principle
6. ✅ Support for future routing types without modifying core news logic

---

## 11. Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Telegram API changes | High | Version lock dependencies; monitor API changelog |
| Configuration confusion | Medium | Clear defaults; document migration |
| Deduplication breakage | Medium | Existing tests cover this; add route-specific tests |

---

## 12. Future Enhancements

- [ ] Multi-route sending (send to email + Telegram simultaneously)
- [ ] Route filtering (send only crypto news to one route, general to another)
- [ ] Rate limiting per route
- [ ] Analytics: track delivery success by route
- [ ] Slack/Discord/Email route implementations

---

## Appendix A: API Reference

### NotificationRoute Interface
```go
type NotificationRoute interface {
	Send(ctx context.Context, message string, metadata *NotificationMetadata) error
	Type() string
}

type NotificationMetadata struct {
	ArticleID    int64
	Category     string
	Source       string
	Sentiment    string
	IsAIProcessed bool
}
```

### Configuration Keys
```yaml
telegram_news_route_type: "topic"           # Route type identifier
telegram_chat_id: "-1002678075016"          # Telegram group ID
telegram_topic_id: "2"                      # Topic/thread ID within group
telegram_bot_token: "123:ABC..."            # Bot authentication token
```

---

## Appendix B: File Changes Summary

```
NEW FILES:
  service/news/route.go                      # Route interface + TelegramTopicRoute impl
  service/news/route_test.go                 # Comprehensive route tests

MODIFIED FILES:
  service/news/service.go                    # Add route dependency, refactor Send
  service/news/service_test.go               # Update tests for route injection
  config/database.go                         # Update config defaults
  main.go                                    # Initialize route before service

DOCUMENTATION:
  openspec/telegram-topic-routing.md         # This proposal
  docs/routing-system.md                     # User guide for routing configuration
```

