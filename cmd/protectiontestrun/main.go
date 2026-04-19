package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"nofx/kernel"
	"nofx/mcp"
	"nofx/market"
	"nofx/store"
)

type fixturePayload struct {
	PromptVariant string               `json:"prompt_variant"`
	Config        store.StrategyConfig `json:"config"`
}

type aiModelRow struct {
	ID              string
	Provider        string
	APIKey          string
	CustomAPIURL    string
	CustomModelName string
}

type runtimeOverride struct {
	Provider    string
	APIKey      string
	BaseURL     string
	CustomModel string
}

func main() {
	fixtureName := os.Getenv("NOFX_TEST_FIXTURE")
	if fixtureName == "" {
		fixtureName = "protection-test-run-fixture.json"
	}
	fixturePath := filepath.Join("docs", "fixtures", fixtureName)
	blob, err := os.ReadFile(fixturePath)
	if err != nil {
		log.Fatalf("read fixture failed: %v", err)
	}
	var fixture fixturePayload
	if err := json.Unmarshal(blob, &fixture); err != nil {
		log.Fatalf("unmarshal fixture failed: %v", err)
	}
	if fixture.PromptVariant == "" {
		fixture.PromptVariant = "balanced"
	}

	override := loadRuntimeOverride()

	var model *aiModelRow
	if override != nil {
		model = &aiModelRow{
			ID:              "runtime-override",
			Provider:        override.Provider,
			APIKey:          override.APIKey,
			CustomAPIURL:    override.BaseURL,
			CustomModelName: override.CustomModel,
		}
	} else {
		model, err = loadEnabledModel(filepath.Join("data", "data.db"), "claude")
		if err != nil {
			log.Fatalf("load enabled model failed: %v", err)
		}
	}

	engine := kernel.NewStrategyEngine(&fixture.Config)
	candidates, err := engine.GetCandidateCoins()
	if err != nil {
		log.Fatalf("GetCandidateCoins failed: %v", err)
	}

	timeframes := fixture.Config.Indicators.Klines.SelectedTimeframes
	primary := fixture.Config.Indicators.Klines.PrimaryTimeframe
	count := fixture.Config.Indicators.Klines.PrimaryCount
	if len(timeframes) == 0 {
		timeframes = []string{"15m", "1h", "4h"}
	}
	if primary == "" {
		primary = timeframes[0]
	}
	if count <= 0 {
		count = 60
	}

	marketDataMap := make(map[string]*market.Data)
	for _, coin := range candidates {
		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primary, count)
		if err != nil {
			fmt.Printf("warn: market data failed for %s: %v\n", coin.Symbol, err)
			continue
		}
		marketDataMap[coin.Symbol] = data
	}

	symbols := make([]string, 0, len(candidates))
	for _, c := range candidates {
		symbols = append(symbols, c.Symbol)
	}

	ctx := &kernel.Context{
		CurrentTime:    time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes: 0,
		CallCount:      1,
		Account: kernel.AccountInfo{
			TotalEquity:      1000,
			AvailableBalance: 1000,
			UnrealizedPnL:    0,
			TotalPnL:         0,
			TotalPnLPct:      0,
			MarginUsed:       0,
			MarginUsedPct:    0,
			PositionCount:    0,
		},
		Positions:          []kernel.PositionInfo{},
		CandidateCoins:     candidates,
		PromptVariant:      fixture.PromptVariant,
		MarketDataMap:      marketDataMap,
		QuantDataMap:       engine.FetchQuantDataBatch(symbols),
		OIRankingData:      engine.FetchOIRankingData(),
		NetFlowRankingData: engine.FetchNetFlowRankingData(),
		PriceRankingData:   engine.FetchPriceRankingData(),
	}

	systemPrompt := engine.BuildSystemPrompt(1000, fixture.PromptVariant)
	userPrompt := engine.BuildUserPrompt(ctx)

	client := mcp.NewAIClientByProvider(model.Provider)
	if client == nil {
		client = mcp.NewClient()
	}
	client.SetAPIKey(model.APIKey, model.CustomAPIURL, model.CustomModelName)

	response, err := client.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		log.Fatalf("real AI call failed: %v", err)
	}

	parsed, parseErr := kernel.ParseAndValidateAIDecisionsWithStrategy(response, &fixture.Config)
	parseErrText := ""
	if parseErr != nil {
		parseErrText = parseErr.Error()
	}

	result := map[string]any{
		"model_id":          model.ID,
		"provider":          model.Provider,
		"custom_model_name": model.CustomModelName,
		"prompt_variant":    fixture.PromptVariant,
		"candidate_count":   len(candidates),
		"system_prompt":     systemPrompt,
		"user_prompt":       userPrompt,
		"ai_response":       response,
		"parsed_decisions":  parsed,
		"parse_error":       parseErrText,
	}

	outPath := filepath.Join("docs", "fixtures", "protection-test-run-last-result.json")
	out, _ := json.MarshalIndent(result, "", "  ")
	if err := os.WriteFile(outPath, out, 0644); err != nil {
		log.Fatalf("write result failed: %v", err)
	}
	fmt.Println(outPath)
}

func loadRuntimeOverride() *runtimeOverride {
	provider := os.Getenv("NOFX_TEST_PROVIDER")
	apiKey := os.Getenv("NOFX_TEST_API_KEY")
	baseURL := os.Getenv("NOFX_TEST_BASE_URL")
	customModel := os.Getenv("NOFX_TEST_MODEL")
	if provider == "" || apiKey == "" {
		return nil
	}
	return &runtimeOverride{
		Provider:    provider,
		APIKey:      apiKey,
		BaseURL:     baseURL,
		CustomModel: customModel,
	}
}

func loadEnabledModel(dbPath, provider string) (*aiModelRow, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	row := db.QueryRow(`SELECT id, provider, api_key, custom_api_url, custom_model_name FROM ai_models WHERE enabled = 1 AND provider = ? LIMIT 1`, provider)
	m := &aiModelRow{}
	if err := row.Scan(&m.ID, &m.Provider, &m.APIKey, &m.CustomAPIURL, &m.CustomModelName); err != nil {
		return nil, err
	}
	return m, nil
}
