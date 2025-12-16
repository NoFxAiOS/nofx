package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"nofx/config"
	"nofx/database"

	_ "github.com/mattn/go-sqlite3"
)

func setupLearningTest(t *testing.T) (*LearningHandler, *gin.Engine, *config.Database) {
	// Use parseTime=true to handle time.Time scanning
	db, err := sql.Open("sqlite3", ":memory:?parseTime=true")
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}

	// Create necessary tables (Split statements for safety)
	_, err = db.Exec(`
		CREATE TABLE trade_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			entry_price REAL,
			exit_price REAL,
			profit_pct REAL,
			leverage INTEGER,
			holding_time_seconds INTEGER,
			margin_mode TEXT,
			created_at TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create trade_records: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE learning_reflections (
            id TEXT PRIMARY KEY,
            trader_id TEXT NOT NULL,
            reflection_type VARCHAR(50),
            severity VARCHAR(20),
            problem_title TEXT NOT NULL,
            problem_description TEXT,
            root_cause TEXT,
            recommended_action TEXT,
            priority INTEGER DEFAULT 0,
            is_applied BOOLEAN DEFAULT FALSE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
	`)
	if err != nil {
		t.Fatalf("Failed to create learning_reflections: %v", err)
	}

	database := config.NewTestDatabase(db)
	h := NewLearningHandler(database)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	return h, r, database
}

func TestHandleGetAnalysis(t *testing.T) {
	h, r, dbConf := setupLearningTest(t)
	r.GET("/api/traders/:id/analysis", h.HandleGetAnalysis)

	// Insert Test Data
	repo := database.NewTradeRepository(dbConf.GetDB())
	now := time.Now()

	// Trade 1: Win
	err := repo.InsertTradeRecord(database.TradeRecord{
		TraderID:           "trader_1",
		Symbol:             "BTCUSDT",
		EntryPrice:         50000,
		ExitPrice:          55000,
		ProfitPct:          10.0,
		Leverage:           5,
		HoldingTimeSeconds: 3600,
		MarginMode:         "cross",
		CreatedAt:          now.Add(-1 * time.Hour),
	})
	assert.NoError(t, err)

	// Trade 2: Loss
	err = repo.InsertTradeRecord(database.TradeRecord{
		TraderID:           "trader_1",
		Symbol:             "BTCUSDT",
		EntryPrice:         50000,
		ExitPrice:          45000,
		ProfitPct:          -10.0,
		Leverage:           5,
		HoldingTimeSeconds: 3600,
		MarginMode:         "cross",
		CreatedAt:          now.Add(-30 * time.Minute),
	})
	assert.NoError(t, err)

	// Execute Request
	req, _ := http.NewRequest("GET", "/api/traders/trader_1/analysis?period=1d", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.Nil(t, err)

	// Check core stats
	// JSON numbers are float64
	assert.Equal(t, 2.0, result["TotalTrades"])
	assert.Equal(t, 1.0, result["WinningTrades"])
	assert.Equal(t, 1.0, result["LosingTrades"])
	assert.Equal(t, 50.0, result["WinRate"])
	assert.Equal(t, 1.0, result["ProfitFactor"])
}
