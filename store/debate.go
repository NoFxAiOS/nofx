package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DebateStatus represents the status of a debate session
type DebateStatus string

const (
	DebateStatusPending   DebateStatus = "pending"
	DebateStatusRunning   DebateStatus = "running"
	DebateStatusVoting    DebateStatus = "voting"
	DebateStatusCompleted DebateStatus = "completed"
	DebateStatusCancelled DebateStatus = "cancelled"
)

// DebatePersonality represents AI personality types
type DebatePersonality string

const (
	PersonalityBull        DebatePersonality = "bull"         // Aggressive Bull - looks for long opportunities
	PersonalityBear        DebatePersonality = "bear"         // Cautious Bear - skeptical, focuses on risks
	PersonalityAnalyst     DebatePersonality = "analyst"      // Data Analyst - pure technical analysis
	PersonalityContrarian  DebatePersonality = "contrarian"   // Contrarian - challenges majority opinion
	PersonalityRiskManager DebatePersonality = "risk_manager" // Risk Manager - focuses on position sizing
)

// PersonalityColors maps personalities to colors for UI
var PersonalityColors = map[DebatePersonality]string{
	PersonalityBull:        "#22C55E", // Green
	PersonalityBear:        "#EF4444", // Red
	PersonalityAnalyst:     "#3B82F6", // Blue
	PersonalityContrarian:  "#F59E0B", // Amber
	PersonalityRiskManager: "#8B5CF6", // Purple
}

// PersonalityEmojis maps personalities to emojis
var PersonalityEmojis = map[DebatePersonality]string{
	PersonalityBull:        "🐂",
	PersonalityBear:        "🐻",
	PersonalityAnalyst:     "📊",
	PersonalityContrarian:  "🔄",
	PersonalityRiskManager: "🛡️",
}

// DebateSession represents a debate session
type DebateSession struct {
	ID              string            `json:"id"`
	UserID          string            `json:"user_id"`
	Name            string            `json:"name"`
	StrategyID      string            `json:"strategy_id"`
	Status          DebateStatus      `json:"status"`
	Symbol          string            `json:"symbol"`           // Primary symbol (for backward compat, may be empty for multi-coin)
	MaxRounds       int               `json:"max_rounds"`
	CurrentRound    int               `json:"current_round"`
	IntervalMinutes int               `json:"interval_minutes"` // Debate interval (5, 15, 30, 60 minutes)
	PromptVariant   string            `json:"prompt_variant"`   // balanced/aggressive/conservative/scalping
	FinalDecision   *DebateDecision   `json:"final_decision,omitempty"`   // Single decision (backward compat)
	FinalDecisions  []*DebateDecision `json:"final_decisions,omitempty"`  // Multi-coin decisions
	AutoExecute     bool              `json:"auto_execute"`
	TraderID        string            `json:"trader_id,omitempty"` // Trader to use for auto-execute
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// DebateDecision represents a trading decision from the debate
type DebateDecision struct {
	Action          string  `json:"action"`            // open_long/open_short/close_long/close_short/hold/wait
	Symbol          string  `json:"symbol"`            // Trading pair
	Confidence      int     `json:"confidence"`        // 0-100
	Leverage        int     `json:"leverage"`          // Recommended leverage
	PositionPct     float64 `json:"position_pct"`      // Position size as percentage of equity (0.0-1.0)
	PositionSizeUSD float64 `json:"position_size_usd"` // Position size in USD (calculated from pct)
	StopLoss        float64 `json:"stop_loss"`         // Stop loss price
	TakeProfit      float64 `json:"take_profit"`       // Take profit price
	Reasoning       string  `json:"reasoning"`         // Brief reasoning

	// Execution tracking
	Executed   bool      `json:"executed"`             // Whether this decision was executed
	ExecutedAt time.Time `json:"executed_at,omitempty"` // When it was executed
	OrderID    string    `json:"order_id,omitempty"`    // Exchange order ID
	Error      string    `json:"error,omitempty"`       // Execution error if any
}

// DebateParticipant represents an AI participant in a debate
type DebateParticipant struct {
	ID          string            `json:"id"`
	SessionID   string            `json:"session_id"`
	AIModelID   string            `json:"ai_model_id"`
	AIModelName string            `json:"ai_model_name"`
	Provider    string            `json:"provider"`
	Personality DebatePersonality `json:"personality"`
	Color       string            `json:"color"`
	SpeakOrder  int               `json:"speak_order"`
	CreatedAt   time.Time         `json:"created_at"`
}

// DebateMessage represents a message in the debate
type DebateMessage struct {
	ID          string            `json:"id"`
	SessionID   string            `json:"session_id"`
	Round       int               `json:"round"`
	AIModelID   string            `json:"ai_model_id"`
	AIModelName string            `json:"ai_model_name"`
	Provider    string            `json:"provider"`
	Personality DebatePersonality `json:"personality"`
	MessageType string            `json:"message_type"` // analysis/rebuttal/final/vote
	Content     string            `json:"content"`
	Decision    *DebateDecision   `json:"decision,omitempty"`   // Single decision (backward compat)
	Decisions   []*DebateDecision `json:"decisions,omitempty"`  // Multi-coin decisions
	Confidence  int               `json:"confidence"`
	CreatedAt   time.Time         `json:"created_at"`
}

// DebateVote represents a final vote from an AI (can contain multiple coin decisions)
type DebateVote struct {
	ID            string            `json:"id"`
	SessionID     string            `json:"session_id"`
	AIModelID     string            `json:"ai_model_id"`
	AIModelName   string            `json:"ai_model_name"`
	Action        string            `json:"action"`           // Primary action (backward compat)
	Symbol        string            `json:"symbol"`           // Primary symbol (backward compat)
	Confidence    int               `json:"confidence"`
	Leverage      int               `json:"leverage"`
	PositionPct   float64           `json:"position_pct"`
	StopLossPct   float64           `json:"stop_loss_pct"`
	TakeProfitPct float64           `json:"take_profit_pct"`
	Reasoning     string            `json:"reasoning"`
	Decisions     []*DebateDecision `json:"decisions,omitempty"` // Multi-coin decisions
	CreatedAt     time.Time         `json:"created_at"`
}

// DebateStore handles database operations for debates
type DebateStore struct {
	db *sql.DB
}

// NewDebateStore creates a new DebateStore
func NewDebateStore(db *sql.DB) *DebateStore {
	return &DebateStore{db: db}
}

// InitSchema creates the debate tables
func (s *DebateStore) InitSchema() error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS debate_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			strategy_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			symbol TEXT NOT NULL,
			max_rounds INTEGER DEFAULT 3,
			current_round INTEGER DEFAULT 0,
			interval_minutes INTEGER DEFAULT 5,
			prompt_variant TEXT DEFAULT 'balanced',
			final_decision TEXT,
			auto_execute BOOLEAN DEFAULT 0,
			trader_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_debate_sessions_user_id ON debate_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_debate_sessions_status ON debate_sessions(status)`,

		`CREATE TABLE IF NOT EXISTS debate_participants (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			ai_model_name TEXT NOT NULL,
			provider TEXT NOT NULL,
			personality TEXT NOT NULL,
			color TEXT NOT NULL,
			speak_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_debate_participants_session ON debate_participants(session_id)`,

		`CREATE TABLE IF NOT EXISTS debate_messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			round INTEGER NOT NULL,
			ai_model_id TEXT NOT NULL,
			ai_model_name TEXT NOT NULL,
			provider TEXT NOT NULL,
			personality TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content TEXT NOT NULL,
			decision TEXT,
			confidence INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_debate_messages_session ON debate_messages(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_debate_messages_round ON debate_messages(session_id, round)`,

		`CREATE TABLE IF NOT EXISTS debate_votes (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			ai_model_name TEXT NOT NULL,
			action TEXT NOT NULL,
			symbol TEXT NOT NULL,
			confidence INTEGER DEFAULT 0,
			leverage INTEGER DEFAULT 5,
			position_pct REAL DEFAULT 0.2,
			stop_loss_pct REAL DEFAULT 0.03,
			take_profit_pct REAL DEFAULT 0.06,
			reasoning TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_debate_votes_session ON debate_votes(session_id)`,

		// Trigger to update updated_at
		`CREATE TRIGGER IF NOT EXISTS update_debate_sessions_timestamp
		 AFTER UPDATE ON debate_sessions
		 FOR EACH ROW
		 BEGIN
			UPDATE debate_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
		 END`,
	}

	for _, schema := range schemas {
		if _, err := s.db.Exec(schema); err != nil {
			return fmt.Errorf("failed to create debate schema: %w", err)
		}
	}

	// Migrate: Add new columns to existing tables (ignore errors if columns already exist)
	migrations := []string{
		`ALTER TABLE debate_sessions ADD COLUMN interval_minutes INTEGER DEFAULT 5`,
		`ALTER TABLE debate_sessions ADD COLUMN prompt_variant TEXT DEFAULT 'balanced'`,
		`ALTER TABLE debate_sessions ADD COLUMN trader_id TEXT`,
		`ALTER TABLE debate_votes ADD COLUMN leverage INTEGER DEFAULT 5`,
		`ALTER TABLE debate_votes ADD COLUMN position_pct REAL DEFAULT 0.2`,
		`ALTER TABLE debate_votes ADD COLUMN stop_loss_pct REAL DEFAULT 0.03`,
		`ALTER TABLE debate_votes ADD COLUMN take_profit_pct REAL DEFAULT 0.06`,
	}

	for _, migration := range migrations {
		// Ignore errors - column may already exist
		s.db.Exec(migration)
	}

	return nil
}

// CreateSession creates a new debate session
func (s *DebateStore) CreateSession(session *DebateSession) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	session.Status = DebateStatusPending
	session.CurrentRound = 0
	if session.IntervalMinutes == 0 {
		session.IntervalMinutes = 5
	}
	if session.PromptVariant == "" {
		session.PromptVariant = "balanced"
	}
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO debate_sessions (id, user_id, name, strategy_id, status, symbol, max_rounds, current_round, interval_minutes, prompt_variant, auto_execute, trader_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.Name, session.StrategyID, session.Status,
		session.Symbol, session.MaxRounds, session.CurrentRound, session.IntervalMinutes, session.PromptVariant,
		session.AutoExecute, session.TraderID, session.CreatedAt, session.UpdatedAt,
	)
	return err
}

// GetSession gets a debate session by ID
func (s *DebateStore) GetSession(id string) (*DebateSession, error) {
	var session DebateSession
	var finalDecisionJSON sql.NullString
	var traderID sql.NullString
	var intervalMinutes sql.NullInt64
	var promptVariant sql.NullString

	// Try new schema first
	err := s.db.QueryRow(`
		SELECT id, user_id, name, strategy_id, status, symbol, max_rounds, current_round,
		       interval_minutes, prompt_variant, final_decision, auto_execute, trader_id, created_at, updated_at
		FROM debate_sessions WHERE id = ?`, id,
	).Scan(
		&session.ID, &session.UserID, &session.Name, &session.StrategyID,
		&session.Status, &session.Symbol, &session.MaxRounds, &session.CurrentRound,
		&intervalMinutes, &promptVariant,
		&finalDecisionJSON, &session.AutoExecute, &traderID, &session.CreatedAt, &session.UpdatedAt,
	)

	// Fallback to basic schema if new columns don't exist
	if err != nil {
		err = s.db.QueryRow(`
			SELECT id, user_id, name, strategy_id, status, symbol, max_rounds, current_round,
			       final_decision, auto_execute, created_at, updated_at
			FROM debate_sessions WHERE id = ?`, id,
		).Scan(
			&session.ID, &session.UserID, &session.Name, &session.StrategyID,
			&session.Status, &session.Symbol, &session.MaxRounds, &session.CurrentRound,
			&finalDecisionJSON, &session.AutoExecute, &session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Set defaults for new fields
		session.IntervalMinutes = 5
		session.PromptVariant = "balanced"
	} else {
		// Set defaults for nullable fields
		session.IntervalMinutes = 5
		if intervalMinutes.Valid {
			session.IntervalMinutes = int(intervalMinutes.Int64)
		}
		session.PromptVariant = "balanced"
		if promptVariant.Valid {
			session.PromptVariant = promptVariant.String
		}
		if traderID.Valid {
			session.TraderID = traderID.String
		}
	}

	if finalDecisionJSON.Valid && finalDecisionJSON.String != "" {
		var decision DebateDecision
		if err := json.Unmarshal([]byte(finalDecisionJSON.String), &decision); err == nil {
			session.FinalDecision = &decision
		}
	}

	return &session, nil
}

// GetSessionsByUser gets all debate sessions for a user
func (s *DebateStore) GetSessionsByUser(userID string) ([]*DebateSession, error) {
	// First try the new schema with all columns
	rows, err := s.db.Query(`
		SELECT id, user_id, name, strategy_id, status, symbol, max_rounds, current_round,
		       interval_minutes, prompt_variant, final_decision, auto_execute, trader_id, created_at, updated_at
		FROM debate_sessions WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)

	// If query fails (likely due to missing columns), try basic query
	if err != nil {
		return s.getSessionsByUserBasic(userID)
	}
	defer rows.Close()

	var sessions []*DebateSession
	for rows.Next() {
		var session DebateSession
		var finalDecisionJSON sql.NullString
		var traderID sql.NullString
		var intervalMinutes sql.NullInt64
		var promptVariant sql.NullString

		if err := rows.Scan(
			&session.ID, &session.UserID, &session.Name, &session.StrategyID,
			&session.Status, &session.Symbol, &session.MaxRounds, &session.CurrentRound,
			&intervalMinutes, &promptVariant,
			&finalDecisionJSON, &session.AutoExecute, &traderID, &session.CreatedAt, &session.UpdatedAt,
		); err != nil {
			return nil, err
		}

		// Set defaults for nullable fields
		session.IntervalMinutes = 5
		if intervalMinutes.Valid {
			session.IntervalMinutes = int(intervalMinutes.Int64)
		}
		session.PromptVariant = "balanced"
		if promptVariant.Valid {
			session.PromptVariant = promptVariant.String
		}

		if finalDecisionJSON.Valid && finalDecisionJSON.String != "" {
			var decision DebateDecision
			if err := json.Unmarshal([]byte(finalDecisionJSON.String), &decision); err == nil {
				session.FinalDecision = &decision
			}
		}
		if traderID.Valid {
			session.TraderID = traderID.String
		}

		sessions = append(sessions, &session)
	}
	return sessions, nil
}

// ListAllSessions returns all debate sessions (for cleanup on startup)
func (s *DebateStore) ListAllSessions() ([]*DebateSession, error) {
	rows, err := s.db.Query(`SELECT id, status FROM debate_sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*DebateSession
	for rows.Next() {
		var session DebateSession
		if err := rows.Scan(&session.ID, &session.Status); err != nil {
			return nil, err
		}
		sessions = append(sessions, &session)
	}
	return sessions, nil
}

// getSessionsByUserBasic is a fallback for old schema without new columns
func (s *DebateStore) getSessionsByUserBasic(userID string) ([]*DebateSession, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, strategy_id, status, symbol, max_rounds, current_round,
		       final_decision, auto_execute, created_at, updated_at
		FROM debate_sessions WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*DebateSession
	for rows.Next() {
		var session DebateSession
		var finalDecisionJSON sql.NullString

		if err := rows.Scan(
			&session.ID, &session.UserID, &session.Name, &session.StrategyID,
			&session.Status, &session.Symbol, &session.MaxRounds, &session.CurrentRound,
			&finalDecisionJSON, &session.AutoExecute, &session.CreatedAt, &session.UpdatedAt,
		); err != nil {
			return nil, err
		}

		// Set defaults for new fields
		session.IntervalMinutes = 5
		session.PromptVariant = "balanced"

		if finalDecisionJSON.Valid && finalDecisionJSON.String != "" {
			var decision DebateDecision
			if err := json.Unmarshal([]byte(finalDecisionJSON.String), &decision); err == nil {
				session.FinalDecision = &decision
			}
		}

		sessions = append(sessions, &session)
	}
	return sessions, nil
}

// UpdateSessionStatus updates the status of a debate session
func (s *DebateStore) UpdateSessionStatus(id string, status DebateStatus) error {
	_, err := s.db.Exec(`UPDATE debate_sessions SET status = ? WHERE id = ?`, status, id)
	return err
}

// UpdateSessionRound updates the current round of a debate session
func (s *DebateStore) UpdateSessionRound(id string, round int) error {
	_, err := s.db.Exec(`UPDATE debate_sessions SET current_round = ? WHERE id = ?`, round, id)
	return err
}

// UpdateSessionFinalDecision updates the final decision of a debate session (single decision)
func (s *DebateStore) UpdateSessionFinalDecision(id string, decision *DebateDecision) error {
	decisionJSON, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE debate_sessions SET final_decision = ?, status = ? WHERE id = ?`,
		string(decisionJSON), DebateStatusCompleted, id)
	return err
}

// UpdateSessionFinalDecisions updates both single and multi-coin final decisions
func (s *DebateStore) UpdateSessionFinalDecisions(id string, primaryDecision *DebateDecision, allDecisions []*DebateDecision) error {
	// Always store primary decision as a single object (for backward compat)
	// This ensures GetSession can deserialize it correctly
	primaryJSON, err := json.Marshal(primaryDecision)
	if err != nil {
		return err
	}

	// Update final_decision with primary decision and set status to completed
	_, err = s.db.Exec(`UPDATE debate_sessions SET final_decision = ?, status = ? WHERE id = ?`,
		string(primaryJSON), DebateStatusCompleted, id)
	return err
}

// DeleteSession deletes a debate session and all related data
func (s *DebateStore) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM debate_sessions WHERE id = ?`, id)
	return err
}

// AddParticipant adds a participant to a debate session
func (s *DebateStore) AddParticipant(participant *DebateParticipant) error {
	if participant.ID == "" {
		participant.ID = uuid.New().String()
	}
	participant.CreatedAt = time.Now()

	// Set color based on personality if not provided
	if participant.Color == "" {
		if color, ok := PersonalityColors[participant.Personality]; ok {
			participant.Color = color
		} else {
			participant.Color = "#6B7280" // Default gray
		}
	}

	_, err := s.db.Exec(`
		INSERT INTO debate_participants (id, session_id, ai_model_id, ai_model_name, provider, personality, color, speak_order, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		participant.ID, participant.SessionID, participant.AIModelID, participant.AIModelName,
		participant.Provider, participant.Personality, participant.Color, participant.SpeakOrder, participant.CreatedAt,
	)
	return err
}

// GetParticipants gets all participants for a debate session
func (s *DebateStore) GetParticipants(sessionID string) ([]*DebateParticipant, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, ai_model_id, ai_model_name, provider, personality, color, speak_order, created_at
		FROM debate_participants WHERE session_id = ? ORDER BY speak_order`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []*DebateParticipant
	for rows.Next() {
		var p DebateParticipant
		if err := rows.Scan(
			&p.ID, &p.SessionID, &p.AIModelID, &p.AIModelName,
			&p.Provider, &p.Personality, &p.Color, &p.SpeakOrder, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		participants = append(participants, &p)
	}
	return participants, nil
}

// AddMessage adds a message to a debate session
func (s *DebateStore) AddMessage(msg *DebateMessage) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.CreatedAt = time.Now()

	var decisionJSON sql.NullString
	if msg.Decision != nil {
		data, err := json.Marshal(msg.Decision)
		if err != nil {
			return err
		}
		decisionJSON = sql.NullString{String: string(data), Valid: true}
	}

	_, err := s.db.Exec(`
		INSERT INTO debate_messages (id, session_id, round, ai_model_id, ai_model_name, provider, personality, message_type, content, decision, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, msg.Round, msg.AIModelID, msg.AIModelName,
		msg.Provider, msg.Personality, msg.MessageType, msg.Content,
		decisionJSON, msg.Confidence, msg.CreatedAt,
	)
	return err
}

// GetMessages gets all messages for a debate session
func (s *DebateStore) GetMessages(sessionID string) ([]*DebateMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, round, ai_model_id, ai_model_name, provider, personality, message_type, content, decision, confidence, created_at
		FROM debate_messages WHERE session_id = ? ORDER BY round, created_at`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*DebateMessage
	for rows.Next() {
		var msg DebateMessage
		var decisionJSON sql.NullString

		if err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.Round, &msg.AIModelID, &msg.AIModelName,
			&msg.Provider, &msg.Personality, &msg.MessageType, &msg.Content,
			&decisionJSON, &msg.Confidence, &msg.CreatedAt,
		); err != nil {
			return nil, err
		}

		if decisionJSON.Valid && decisionJSON.String != "" {
			var decision DebateDecision
			if err := json.Unmarshal([]byte(decisionJSON.String), &decision); err == nil {
				msg.Decision = &decision
			}
		}

		messages = append(messages, &msg)
	}
	return messages, nil
}

// GetMessagesByRound gets messages for a specific round
func (s *DebateStore) GetMessagesByRound(sessionID string, round int) ([]*DebateMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, round, ai_model_id, ai_model_name, provider, personality, message_type, content, decision, confidence, created_at
		FROM debate_messages WHERE session_id = ? AND round = ? ORDER BY created_at`, sessionID, round,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*DebateMessage
	for rows.Next() {
		var msg DebateMessage
		var decisionJSON sql.NullString

		if err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.Round, &msg.AIModelID, &msg.AIModelName,
			&msg.Provider, &msg.Personality, &msg.MessageType, &msg.Content,
			&decisionJSON, &msg.Confidence, &msg.CreatedAt,
		); err != nil {
			return nil, err
		}

		if decisionJSON.Valid && decisionJSON.String != "" {
			var decision DebateDecision
			if err := json.Unmarshal([]byte(decisionJSON.String), &decision); err == nil {
				msg.Decision = &decision
			}
		}

		messages = append(messages, &msg)
	}
	return messages, nil
}

// AddVote adds a vote to a debate session
func (s *DebateStore) AddVote(vote *DebateVote) error {
	if vote.ID == "" {
		vote.ID = uuid.New().String()
	}
	vote.CreatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO debate_votes (id, session_id, ai_model_id, ai_model_name, action, symbol, confidence, leverage, position_pct, stop_loss_pct, take_profit_pct, reasoning, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		vote.ID, vote.SessionID, vote.AIModelID, vote.AIModelName,
		vote.Action, vote.Symbol, vote.Confidence, vote.Leverage, vote.PositionPct, vote.StopLossPct, vote.TakeProfitPct, vote.Reasoning, vote.CreatedAt,
	)
	return err
}

// GetVotes gets all votes for a debate session
func (s *DebateStore) GetVotes(sessionID string) ([]*DebateVote, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, ai_model_id, ai_model_name, action, symbol, confidence, leverage, position_pct, stop_loss_pct, take_profit_pct, reasoning, created_at
		FROM debate_votes WHERE session_id = ? ORDER BY created_at`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var votes []*DebateVote
	for rows.Next() {
		var vote DebateVote
		if err := rows.Scan(
			&vote.ID, &vote.SessionID, &vote.AIModelID, &vote.AIModelName,
			&vote.Action, &vote.Symbol, &vote.Confidence, &vote.Leverage, &vote.PositionPct, &vote.StopLossPct, &vote.TakeProfitPct, &vote.Reasoning, &vote.CreatedAt,
		); err != nil {
			return nil, err
		}
		votes = append(votes, &vote)
	}
	return votes, nil
}

// DebateSessionWithDetails combines session with participants and messages
type DebateSessionWithDetails struct {
	*DebateSession
	Participants []*DebateParticipant `json:"participants"`
	Messages     []*DebateMessage     `json:"messages"`
	Votes        []*DebateVote        `json:"votes"`
}

// GetSessionWithDetails gets a session with all related data
func (s *DebateStore) GetSessionWithDetails(id string) (*DebateSessionWithDetails, error) {
	session, err := s.GetSession(id)
	if err != nil {
		return nil, err
	}

	participants, err := s.GetParticipants(id)
	if err != nil {
		return nil, err
	}

	messages, err := s.GetMessages(id)
	if err != nil {
		return nil, err
	}

	votes, err := s.GetVotes(id)
	if err != nil {
		return nil, err
	}

	return &DebateSessionWithDetails{
		DebateSession: session,
		Participants:  participants,
		Messages:      messages,
		Votes:         votes,
	}, nil
}
