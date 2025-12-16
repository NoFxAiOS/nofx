package news

import (
	"strings"
	"testing"
	"time"
)

// --- Mocks ---

type MockFetcher struct {
	News     []Article
	Err      error
	MockName string
}

func (m *MockFetcher) FetchNews(category string) ([]Article, error) {
	return m.News, m.Err
}

func (m *MockFetcher) Name() string {
	if m.MockName != "" {
		return m.MockName
	}
	return "MockFetcher"
}

type MockNotifier struct {
	SentMessages []string
	Err          error
	LastThreadID int
}

func (m *MockNotifier) Send(msg string, messageThreadID int) error {
	if m.Err != nil {
		return m.Err
	}
	m.SentMessages = append(m.SentMessages, msg)
	m.LastThreadID = messageThreadID
	return nil
}

type MockStateStore struct {
	LastID        int64
	LastTimestamp int64
	GetErr        error
	UpdateErr     error
	Configs       map[string]string
}

func (m *MockStateStore) GetNewsState(category string) (int64, int64, error) {
	return m.LastID, m.LastTimestamp, m.GetErr
}

func (m *MockStateStore) UpdateNewsState(category string, id int64, timestamp int64) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	// Simulate state update
	if id > m.LastID {
		m.LastID = id
	}
	if timestamp > m.LastTimestamp {
		m.LastTimestamp = timestamp
	}
	return nil
}

func (m *MockStateStore) GetSystemConfig(key string) (string, error) {
	return m.Configs[key], nil
}

// --- Tests ---

func TestService_ProcessFetcher_Deduplication(t *testing.T) {
	// Setup
	mockStore := &MockStateStore{
		LastID:        100,
		LastTimestamp: 1000,
		Configs:       map[string]string{"telegram_news_enabled": "true"},
	}
	mockFetcher := &MockFetcher{
		News: []Article{
			{ID: 99, Datetime: 999, Headline: "Old News"},   // Should be skipped
			{ID: 100, Datetime: 1000, Headline: "Boundary"}, // Should be skipped
			{ID: 101, Datetime: 1001, Headline: "New News"}, // Should be sent
		},
	}
	mockNotifier := &MockNotifier{}

	service := NewService(mockStore)
	service.fetchers = []Fetcher{mockFetcher}
	service.topicRouter = map[string]int{"MockFetcher": 0}
	service.notifier = mockNotifier
	service.enabled = true

	// Execute
	err := service.ProcessFetcher(mockFetcher, "crypto")

	// Verify
	if err != nil {
		t.Fatalf("ProcessFetcher failed: %v", err)
	}

	if len(mockNotifier.SentMessages) != 1 {
		t.Errorf("Expected 1 message sent, got %d", len(mockNotifier.SentMessages))
	} else {
		if !strings.Contains(mockNotifier.SentMessages[0], "New News") {
			t.Errorf("Expected message to contain 'New News', got %s", mockNotifier.SentMessages[0])
		}
	}
}

func TestService_ProcessFetcher_PassThrough(t *testing.T) {
	// Setup
	mockStore := &MockStateStore{
		Configs: map[string]string{"telegram_news_enabled": "true"},
	}
	mockFetcher := &MockFetcher{
		News: []Article{
			{ID: 1, Datetime: 2000, Headline: "Random Stuff", Summary: "Nothing important"}, // Should be sent
			{ID: 2, Datetime: 2001, Headline: "Bitcoin Update", Summary: "Moon!"},           // Should be sent
			{ID: 3, Datetime: 2002, Headline: "Fed Decision", Summary: "Rates up"},          // Should be sent
		},
	}
	mockNotifier := &MockNotifier{}

	service := NewService(mockStore)
	service.fetchers = []Fetcher{mockFetcher}
	service.topicRouter = map[string]int{"MockFetcher": 0}
	service.notifier = mockNotifier
	service.enabled = true

	// Execute
	err := service.ProcessFetcher(mockFetcher, "general")

	// Verify
	if err != nil {
		t.Fatalf("ProcessFetcher failed: %v", err)
	}

	if len(mockNotifier.SentMessages) != 3 {
		t.Errorf("Expected 3 messages sent, got %d", len(mockNotifier.SentMessages))
	}
}

func TestService_ProcessFetcher_StateUpdates(t *testing.T) {
	// Setup
	mockStore := &MockStateStore{
		LastID:  0,
		Configs: map[string]string{"telegram_news_enabled": "true"},
	}
	mockFetcher := &MockFetcher{
		News: []Article{
			{ID: 10, Datetime: 100},
			{ID: 20, Datetime: 200},
		},
	}
	mockNotifier := &MockNotifier{}

	service := NewService(mockStore)
	service.fetchers = []Fetcher{mockFetcher}
	service.topicRouter = map[string]int{"MockFetcher": 0}
	service.notifier = mockNotifier
	service.enabled = true

	// Execute
	service.ProcessFetcher(mockFetcher, "crypto")

	// Verify Store Update
	if mockStore.LastID != 20 {
		t.Errorf("Expected LastID to be updated to 20, got %d", mockStore.LastID)
	}
}

func TestFormatMessage(t *testing.T) {
	ts := time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC).Unix()
	expectedTimeStr := time.Unix(ts, 0).Format("15:04")

	article := Article{
		ID:       123,
		Headline: "Bitcoin hits $100k",
		Summary:  "It finally happened.",
		URL:      "https://example.com/btc",
		Datetime: ts,
		Category: "crypto",
		Source:   "Test",
	}

	msg := formatMessage(article)

	expectedContains := []string{
		"Bitcoin hits $100k",
		expectedTimeStr,
		"#CRYPTO",
		"It finally happened.",
		"https://example.com/btc",
	}

	for _, sub := range expectedContains {
		if !strings.Contains(msg, sub) {
			t.Errorf("Expected message to contain %q, but it didn't. Msg: %s", sub, msg)
		}
	}
}

// TestService_CrossCategoryDeduplication 测试跨分类去重功能
func TestService_CrossCategoryDeduplication(t *testing.T) {
	// Setup - 模拟同一条新闻在crypto和general两个分类中都出现
	mockStore := &MockStateStore{
		LastID:        0,
		LastTimestamp: 0,
		Configs:       map[string]string{"telegram_news_enabled": "true"},
	}

	// 这条新闻(ID=100)在两个分类中都会被fetcher返回
	sameArticle := Article{
		ID:       100,
		Headline: "Bitcoin News",
		Summary:  "Shared across categories",
		Datetime: time.Now().Unix(),
		Category: "crypto",
		Source:   "Reuters",
	}

	// 模拟fetcher：两个分类都返回相同的文章ID
	mockFetcher := &MockFetcher{
		News: []Article{sameArticle},
		Err:  nil,
	}

	mockNotifier := &MockNotifier{}

	service := NewService(mockStore)
	service.fetchers = []Fetcher{mockFetcher}
	service.topicRouter = map[string]int{"MockFetcher": 0}
	service.notifier = mockNotifier
	service.enabled = true

	// Execute - 处理两个分类
	// ProcessFetcher is called per category manually here for the test logic,
	// emulating loop in processAllCategories

	t.Logf("Processing crypto category...")

	err := service.ProcessFetcher(mockFetcher, "crypto")
	if err != nil {
		t.Fatalf("ProcessFetcher(crypto) failed: %v", err)
	}

	t.Logf("Processing general category...")
	err = service.ProcessFetcher(mockFetcher, "general")
	if err != nil {
		t.Fatalf("ProcessFetcher(general) failed: %v", err)
	}

	// Verify - 应该只发送一次消息（来自第一个分类）
	if len(mockNotifier.SentMessages) != 1 {
		t.Errorf("Expected 1 message sent (deduplication), got %d", len(mockNotifier.SentMessages))
		for i, msg := range mockNotifier.SentMessages {
			t.Logf("Message %d: %s", i, msg)
		}
	} else {
		t.Logf("✓ Cross-category deduplication works: only 1 message sent despite appearing in 2 categories")
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
