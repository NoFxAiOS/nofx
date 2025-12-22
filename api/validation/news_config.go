package validation

// News Source Configuration Constants
const (
	// Valid news sources
	NewsSourceMlion    = "mlion"
	NewsSourceTwitter  = "twitter"
	NewsSourceReddit   = "reddit"
	NewsSourceTelegram = "telegram"

	// Fetch interval constraints (in minutes)
	MinFetchInterval = 1
	MaxFetchInterval = 1440

	// Article count constraints
	MinArticleCount = 1
	MaxArticleCount = 100

	// Sentiment threshold constraints
	MinSentimentThreshold = -1.0
	MaxSentimentThreshold = 1.0
)

// ValidNewsSources returns the list of valid news sources
var ValidNewsSources = []string{
	NewsSourceMlion,
	NewsSourceTwitter,
	NewsSourceReddit,
	NewsSourceTelegram,
}

// ValidateNewsConfig validates all news configuration parameters
type NewsConfigValidationErrors struct {
	NewsSources             *string
	AutoFetchIntervalMinutes *string
	MaxArticlesPerFetch     *string
	SentimentThreshold      *string
}

// ValidateNewsConfigRequest validates news configuration parameters
func ValidateNewsConfigRequest(
	newsSources string,
	autoFetchInterval int,
	maxArticles int,
	sentimentThreshold float64,
) *NewsConfigValidationErrors {
	errors := &NewsConfigValidationErrors{}

	// Validate news sources
	if newsSources == "" {
		msg := "必须至少选择一个新闻源"
		errors.NewsSources = &msg
	}

	// Validate fetch interval
	if autoFetchInterval < MinFetchInterval || autoFetchInterval > MaxFetchInterval {
		msg := "抓取间隔必须在1-1440分钟之间"
		errors.AutoFetchIntervalMinutes = &msg
	}

	// Validate article count
	if maxArticles < MinArticleCount || maxArticles > MaxArticleCount {
		msg := "每次抓取的最大文章数必须在1-100之间"
		errors.MaxArticlesPerFetch = &msg
	}

	// Validate sentiment threshold
	if sentimentThreshold < MinSentimentThreshold || sentimentThreshold > MaxSentimentThreshold {
		msg := "情绪阈值必须在-1.0到1.0之间"
		errors.SentimentThreshold = &msg
	}

	// Check if there are any errors
	if errors.NewsSources == nil &&
		errors.AutoFetchIntervalMinutes == nil &&
		errors.MaxArticlesPerFetch == nil &&
		errors.SentimentThreshold == nil {
		return nil
	}

	return errors
}

// IsValidNewsSource checks if a given source is in the valid list
func IsValidNewsSource(source string) bool {
	for _, valid := range ValidNewsSources {
		if valid == source {
			return true
		}
	}
	return false
}
