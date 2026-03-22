package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// UserProfile captures what the AI has learned about a user's trading behavior.
type UserProfile struct {
	UserID          int64     `json:"user_id"`
	TotalTrades     int       `json:"total_trades"`
	WinRate         float64   `json:"win_rate"`
	AvgHoldTime     float64   `json:"avg_hold_time_hours"`
	PreferredSide   string    `json:"preferred_side"`   // "long", "short", "balanced"
	RiskTolerance   string    `json:"risk_tolerance"`   // "conservative", "moderate", "aggressive"
	FavoriteSymbols []string  `json:"favorite_symbols"`
	AvgLeverage     float64   `json:"avg_leverage"`
	BestStrategy    string    `json:"best_strategy"`
	WorstStrategy   string    `json:"worst_strategy"`
	TotalPnL        float64   `json:"total_pnl"`
	BiggestWin      float64   `json:"biggest_win"`
	BiggestLoss     float64   `json:"biggest_loss"`
	LastAnalyzed    time.Time `json:"last_analyzed"`
}

// Lesson is an insight learned from past trading.
type Lesson struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Type      string    `json:"type"`    // "win_pattern", "loss_pattern", "risk_insight", "strategy_note"
	Content   string    `json:"content"` // Natural language description
	Symbol    string    `json:"symbol,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// InitLearnerTables creates the learner-specific tables.
func (s *Store) InitLearnerTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS user_profiles (
			user_id INTEGER PRIMARY KEY,
			total_trades INTEGER DEFAULT 0,
			win_rate REAL DEFAULT 0,
			avg_hold_time REAL DEFAULT 0,
			preferred_side TEXT DEFAULT 'balanced',
			risk_tolerance TEXT DEFAULT 'moderate',
			favorite_symbols TEXT DEFAULT '',
			avg_leverage REAL DEFAULT 1,
			best_strategy TEXT DEFAULT '',
			worst_strategy TEXT DEFAULT '',
			total_pnl REAL DEFAULT 0,
			biggest_win REAL DEFAULT 0,
			biggest_loss REAL DEFAULT 0,
			last_analyzed DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS lessons (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			symbol TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ai_predictions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			predicted_action TEXT NOT NULL,
			predicted_confidence REAL,
			actual_result TEXT,
			actual_pnl REAL,
			model TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_lessons_user ON lessons(user_id, type)`,
		`CREATE INDEX IF NOT EXISTS idx_predictions_symbol ON ai_predictions(symbol, created_at)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("learner migration: %w", err)
		}
	}
	return nil
}

// SaveLesson stores a trading lesson.
func (s *Store) SaveLesson(userID int64, lessonType, content, symbol string) error {
	_, err := s.db.Exec(
		`INSERT INTO lessons (user_id, type, content, symbol) VALUES (?, ?, ?, ?)`,
		userID, lessonType, content, symbol,
	)
	return err
}

// GetLessons retrieves lessons for a user.
func (s *Store) GetLessons(userID int64, limit int) ([]Lesson, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, type, content, COALESCE(symbol,''), created_at
		 FROM lessons WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []Lesson
	for rows.Next() {
		var l Lesson
		if err := rows.Scan(&l.ID, &l.UserID, &l.Type, &l.Content, &l.Symbol, &l.CreatedAt); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, nil
}

// SavePrediction logs an AI prediction for later evaluation.
func (s *Store) SavePrediction(symbol, action string, confidence float64, model string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO ai_predictions (symbol, predicted_action, predicted_confidence, model) VALUES (?, ?, ?, ?)`,
		symbol, action, confidence, model,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ResolvePrediction updates a prediction with the actual result.
func (s *Store) ResolvePrediction(id int64, result string, pnl float64) error {
	_, err := s.db.Exec(
		`UPDATE ai_predictions SET actual_result = ?, actual_pnl = ?, resolved_at = ? WHERE id = ?`,
		result, pnl, time.Now(), id,
	)
	return err
}

// GetPredictionAccuracy returns the accuracy of AI predictions.
func (s *Store) GetPredictionAccuracy(model string) (total int, correct int, avgPnL float64, err error) {
	var pnl sql.NullFloat64
	err = s.db.QueryRow(
		`SELECT COUNT(*), SUM(CASE WHEN actual_pnl > 0 THEN 1 ELSE 0 END), AVG(actual_pnl)
		 FROM ai_predictions WHERE resolved_at IS NOT NULL AND (? = '' OR model = ?)`,
		model, model,
	).Scan(&total, &correct, &pnl)
	if pnl.Valid {
		avgPnL = pnl.Float64
	}
	return
}

// AnalyzeUserProfile builds a profile from trading history.
func (s *Store) AnalyzeUserProfile(userID int64) (*UserProfile, error) {
	trades, err := s.GetRecentTrades(1000)
	if err != nil {
		return nil, err
	}

	if len(trades) == 0 {
		return &UserProfile{UserID: userID}, nil
	}

	profile := &UserProfile{
		UserID:      userID,
		TotalTrades: len(trades),
	}

	wins := 0
	symbolCount := make(map[string]int)
	var totalPnL, bigWin, bigLoss, totalLev float64
	longCount, shortCount := 0, 0

	for _, t := range trades {
		totalPnL += t.PnL
		if t.PnL > 0 {
			wins++
		}
		if t.PnL > bigWin {
			bigWin = t.PnL
		}
		if t.PnL < bigLoss {
			bigLoss = t.PnL
		}
		symbolCount[t.Symbol]++
		if t.Side == "long" || t.Side == "buy" {
			longCount++
		} else {
			shortCount++
		}
	}

	profile.WinRate = float64(wins) / float64(len(trades)) * 100
	profile.TotalPnL = totalPnL
	profile.BiggestWin = bigWin
	profile.BiggestLoss = bigLoss
	profile.AvgLeverage = totalLev / float64(len(trades))

	if longCount > shortCount*2 {
		profile.PreferredSide = "long"
	} else if shortCount > longCount*2 {
		profile.PreferredSide = "short"
	} else {
		profile.PreferredSide = "balanced"
	}

	// Top symbols
	var favs []string
	for sym := range symbolCount {
		favs = append(favs, sym)
	}
	if len(favs) > 5 {
		favs = favs[:5]
	}
	profile.FavoriteSymbols = favs

	// Risk tolerance based on leverage and loss patterns
	if profile.BiggestLoss < -500 || profile.AvgLeverage > 10 {
		profile.RiskTolerance = "aggressive"
	} else if profile.BiggestLoss < -100 || profile.AvgLeverage > 3 {
		profile.RiskTolerance = "moderate"
	} else {
		profile.RiskTolerance = "conservative"
	}

	profile.LastAnalyzed = time.Now()
	return profile, nil
}
