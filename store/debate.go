package store

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	Executed   bool      `json:"executed"`              // Whether this decision was executed
	ExecutedAt time.Time `json:"executed_at,omitempty"` // When it was executed
	OrderID    string    `json:"order_id,omitempty"`    // Exchange order ID
	Error      string    `json:"error,omitempty"`       // Execution error if any
}

// DebateSession represents a debate session (API struct)
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
	FinalDecision   *DebateDecision   `json:"final_decision,omitempty"`  // Single decision (backward compat)
	FinalDecisions  []*DebateDecision `json:"final_decisions,omitempty"` // Multi-coin decisions
	AutoExecute     bool              `json:"auto_execute"`
	TraderID        string            `json:"trader_id,omitempty"` // Trader to use for auto-execute
	// OI Ranking data options
	EnableOIRanking bool      `json:"enable_oi_ranking"` // Whether to include OI ranking data
	OIRankingLimit  int       `json:"oi_ranking_limit"`  // Number of OI ranking entries (default 10)
	OIDuration      string    `json:"oi_duration"`       // Duration for OI data (1h, 4h, 24h, etc.)
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DebateSessionDB is the GORM model for debate_sessions
type DebateSessionDB struct {
	ID              string       `gorm:"column:id;primaryKey"`
	UserID          string       `gorm:"column:user_id;not null;index"`
	Name            string       `gorm:"column:name;not null"`
	StrategyID      string       `gorm:"column:strategy_id;not null"`
	Status          DebateStatus `gorm:"column:status;not null;default:pending;index"`
	Symbol          string       `gorm:"column:symbol;not null"`
	MaxRounds       int          `gorm:"column:max_rounds;default:3"`
	CurrentRound    int          `gorm:"column:current_round;default:0"`
	IntervalMinutes int          `gorm:"column:interval_minutes;default:5"`
	PromptVariant   string       `gorm:"column:prompt_variant;default:balanced"`
	FinalDecision   string       `gorm:"column:final_decision"` // JSON string
	AutoExecute     bool         `gorm:"column:auto_execute;default:false"`
	TraderID        string       `gorm:"column:trader_id"`
	EnableOIRanking bool         `gorm:"column:enable_oi_ranking;default:false"`
	OIRankingLimit  int          `gorm:"column:oi_ranking_limit;default:10"`
	OIDuration      string       `gorm:"column:oi_duration;default:1h"`
	CreatedAt       time.Time    `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time    `gorm:"column:updated_at;autoUpdateTime"`
}

func (DebateSessionDB) TableName() string {
	return "debate_sessions"
}

func (db *DebateSessionDB) toSession() *DebateSession {
	s := &DebateSession{
		ID:              db.ID,
		UserID:          db.UserID,
		Name:            db.Name,
		StrategyID:      db.StrategyID,
		Status:          db.Status,
		Symbol:          db.Symbol,
		MaxRounds:       db.MaxRounds,
		CurrentRound:    db.CurrentRound,
		IntervalMinutes: db.IntervalMinutes,
		PromptVariant:   db.PromptVariant,
		AutoExecute:     db.AutoExecute,
		TraderID:        db.TraderID,
		EnableOIRanking: db.EnableOIRanking,
		OIRankingLimit:  db.OIRankingLimit,
		OIDuration:      db.OIDuration,
		CreatedAt:       db.CreatedAt,
		UpdatedAt:       db.UpdatedAt,
	}

	// Set defaults
	if s.IntervalMinutes == 0 {
		s.IntervalMinutes = 5
	}
	if s.PromptVariant == "" {
		s.PromptVariant = "balanced"
	}
	if s.OIRankingLimit == 0 {
		s.OIRankingLimit = 10
	}
	if s.OIDuration == "" {
		s.OIDuration = "1h"
	}

	// Parse final decision
	if db.FinalDecision != "" {
		var decision DebateDecision
		if json.Unmarshal([]byte(db.FinalDecision), &decision) == nil {
			s.FinalDecision = &decision
		}
	}

	return s
}

// DebateParticipant represents an AI participant in a debate
type DebateParticipant struct {
	ID          string            `gorm:"column:id;primaryKey" json:"id"`
	SessionID   string            `gorm:"column:session_id;not null;index" json:"session_id"`
	AIModelID   string            `gorm:"column:ai_model_id;not null" json:"ai_model_id"`
	AIModelName string            `gorm:"column:ai_model_name;not null" json:"ai_model_name"`
	Provider    string            `gorm:"column:provider;not null" json:"provider"`
	Personality DebatePersonality `gorm:"column:personality;not null" json:"personality"`
	Color       string            `gorm:"column:color;not null" json:"color"`
	SpeakOrder  int               `gorm:"column:speak_order;default:0" json:"speak_order"`
	CreatedAt   time.Time         `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (DebateParticipant) TableName() string {
	return "debate_participants"
}

// DebateMessage represents a message in the debate
type DebateMessage struct {
	ID          string            `gorm:"column:id;primaryKey" json:"id"`
	SessionID   string            `gorm:"column:session_id;not null;index" json:"session_id"`
	Round       int               `gorm:"column:round;not null" json:"round"`
	AIModelID   string            `gorm:"column:ai_model_id;not null" json:"ai_model_id"`
	AIModelName string            `gorm:"column:ai_model_name;not null" json:"ai_model_name"`
	Provider    string            `gorm:"column:provider;not null" json:"provider"`
	Personality DebatePersonality `gorm:"column:personality;not null" json:"personality"`
	MessageType string            `gorm:"column:message_type;not null" json:"message_type"` // analysis/rebuttal/final/vote
	Content     string            `gorm:"column:content;not null" json:"content"`
	DecisionRaw string            `gorm:"column:decision" json:"-"`                       // JSON string in DB
	Decision    *DebateDecision   `gorm:"-" json:"decision,omitempty"`                    // Parsed for API
	Decisions   []*DebateDecision `gorm:"-" json:"decisions,omitempty"`                   // Multi-coin decisions
	Confidence  int               `gorm:"column:confidence;default:0" json:"confidence"`
	CreatedAt   time.Time         `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (DebateMessage) TableName() string {
	return "debate_messages"
}

// DebateVote represents a final vote from an AI (can contain multiple coin decisions)
type DebateVote struct {
	ID            string            `gorm:"column:id;primaryKey" json:"id"`
	SessionID     string            `gorm:"column:session_id;not null;index" json:"session_id"`
	AIModelID     string            `gorm:"column:ai_model_id;not null" json:"ai_model_id"`
	AIModelName   string            `gorm:"column:ai_model_name;not null" json:"ai_model_name"`
	Action        string            `gorm:"column:action;not null" json:"action"`   // Primary action (backward compat)
	Symbol        string            `gorm:"column:symbol;not null" json:"symbol"`   // Primary symbol (backward compat)
	Confidence    int               `gorm:"column:confidence;default:0" json:"confidence"`
	Leverage      int               `gorm:"column:leverage;default:5" json:"leverage"`
	PositionPct   float64           `gorm:"column:position_pct;default:0.2" json:"position_pct"`
	StopLossPct   float64           `gorm:"column:stop_loss_pct;default:0.03" json:"stop_loss_pct"`
	TakeProfitPct float64           `gorm:"column:take_profit_pct;default:0.06" json:"take_profit_pct"`
	Reasoning     string            `gorm:"column:reasoning" json:"reasoning"`
	Decisions     []*DebateDecision `gorm:"-" json:"decisions,omitempty"` // Multi-coin decisions
	CreatedAt     time.Time         `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (DebateVote) TableName() string {
	return "debate_votes"
}

// DebateStore handles database operations for debates
type DebateStore struct {
	db *gorm.DB
}

// NewDebateStore creates a new DebateStore
func NewDebateStore(db *gorm.DB) *DebateStore {
	return &DebateStore{db: db}
}

// InitSchema creates the debate tables using GORM AutoMigrate
func (s *DebateStore) InitSchema() error {
	return s.db.AutoMigrate(
		&DebateSessionDB{},
		&DebateParticipant{},
		&DebateMessage{},
		&DebateVote{},
	)
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
	if session.OIRankingLimit == 0 {
		session.OIRankingLimit = 10
	}
	if session.OIDuration == "" {
		session.OIDuration = "1h"
	}

	db := &DebateSessionDB{
		ID:              session.ID,
		UserID:          session.UserID,
		Name:            session.Name,
		StrategyID:      session.StrategyID,
		Status:          session.Status,
		Symbol:          session.Symbol,
		MaxRounds:       session.MaxRounds,
		CurrentRound:    session.CurrentRound,
		IntervalMinutes: session.IntervalMinutes,
		PromptVariant:   session.PromptVariant,
		AutoExecute:     session.AutoExecute,
		TraderID:        session.TraderID,
		EnableOIRanking: session.EnableOIRanking,
		OIRankingLimit:  session.OIRankingLimit,
		OIDuration:      session.OIDuration,
	}

	return s.db.Create(db).Error
}

// GetSession gets a debate session by ID
func (s *DebateStore) GetSession(id string) (*DebateSession, error) {
	var db DebateSessionDB
	if err := s.db.Where("id = ?", id).First(&db).Error; err != nil {
		return nil, err
	}
	return db.toSession(), nil
}

// GetSessionsByUser gets all debate sessions for a user
func (s *DebateStore) GetSessionsByUser(userID string) ([]*DebateSession, error) {
	var dbs []DebateSessionDB
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&dbs).Error; err != nil {
		return nil, err
	}

	sessions := make([]*DebateSession, len(dbs))
	for i, db := range dbs {
		sessions[i] = db.toSession()
	}
	return sessions, nil
}

// ListAllSessions returns all debate sessions (for cleanup on startup)
func (s *DebateStore) ListAllSessions() ([]*DebateSession, error) {
	var dbs []DebateSessionDB
	if err := s.db.Select("id, status").Find(&dbs).Error; err != nil {
		return nil, err
	}

	sessions := make([]*DebateSession, len(dbs))
	for i, db := range dbs {
		sessions[i] = &DebateSession{ID: db.ID, Status: db.Status}
	}
	return sessions, nil
}

// UpdateSessionStatus updates the status of a debate session
func (s *DebateStore) UpdateSessionStatus(id string, status DebateStatus) error {
	return s.db.Model(&DebateSessionDB{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateSessionRound updates the current round of a debate session
func (s *DebateStore) UpdateSessionRound(id string, round int) error {
	return s.db.Model(&DebateSessionDB{}).Where("id = ?", id).Update("current_round", round).Error
}

// UpdateSessionFinalDecision updates the final decision of a debate session (single decision)
func (s *DebateStore) UpdateSessionFinalDecision(id string, decision *DebateDecision) error {
	decisionJSON, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	return s.db.Model(&DebateSessionDB{}).Where("id = ?", id).Updates(map[string]interface{}{
		"final_decision": string(decisionJSON),
		"status":         DebateStatusCompleted,
	}).Error
}

// UpdateSessionFinalDecisions updates both single and multi-coin final decisions
func (s *DebateStore) UpdateSessionFinalDecisions(id string, primaryDecision *DebateDecision, allDecisions []*DebateDecision) error {
	primaryJSON, err := json.Marshal(primaryDecision)
	if err != nil {
		return err
	}
	return s.db.Model(&DebateSessionDB{}).Where("id = ?", id).Updates(map[string]interface{}{
		"final_decision": string(primaryJSON),
		"status":         DebateStatusCompleted,
	}).Error
}

// DeleteSession deletes a debate session and all related data
func (s *DebateStore) DeleteSession(id string) error {
	// Delete related data first
	s.db.Where("session_id = ?", id).Delete(&DebateParticipant{})
	s.db.Where("session_id = ?", id).Delete(&DebateMessage{})
	s.db.Where("session_id = ?", id).Delete(&DebateVote{})
	return s.db.Where("id = ?", id).Delete(&DebateSessionDB{}).Error
}

// AddParticipant adds a participant to a debate session
func (s *DebateStore) AddParticipant(participant *DebateParticipant) error {
	if participant.ID == "" {
		participant.ID = uuid.New().String()
	}
	if participant.Color == "" {
		if color, ok := PersonalityColors[participant.Personality]; ok {
			participant.Color = color
		} else {
			participant.Color = "#6B7280" // Default gray
		}
	}
	return s.db.Create(participant).Error
}

// GetParticipants gets all participants for a debate session
func (s *DebateStore) GetParticipants(sessionID string) ([]*DebateParticipant, error) {
	var participants []*DebateParticipant
	err := s.db.Where("session_id = ?", sessionID).Order("speak_order").Find(&participants).Error
	return participants, err
}

// AddMessage adds a message to a debate session
func (s *DebateStore) AddMessage(msg *DebateMessage) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Decision != nil {
		data, err := json.Marshal(msg.Decision)
		if err != nil {
			return err
		}
		msg.DecisionRaw = string(data)
	}
	return s.db.Create(msg).Error
}

// GetMessages gets all messages for a debate session
func (s *DebateStore) GetMessages(sessionID string) ([]*DebateMessage, error) {
	var messages []*DebateMessage
	err := s.db.Where("session_id = ?", sessionID).Order("round, created_at").Find(&messages).Error
	if err != nil {
		return nil, err
	}

	// Parse decision JSON
	for _, msg := range messages {
		if msg.DecisionRaw != "" {
			var decision DebateDecision
			if json.Unmarshal([]byte(msg.DecisionRaw), &decision) == nil {
				msg.Decision = &decision
			}
		}
	}
	return messages, nil
}

// GetMessagesByRound gets messages for a specific round
func (s *DebateStore) GetMessagesByRound(sessionID string, round int) ([]*DebateMessage, error) {
	var messages []*DebateMessage
	err := s.db.Where("session_id = ? AND round = ?", sessionID, round).Order("created_at").Find(&messages).Error
	if err != nil {
		return nil, err
	}

	// Parse decision JSON
	for _, msg := range messages {
		if msg.DecisionRaw != "" {
			var decision DebateDecision
			if json.Unmarshal([]byte(msg.DecisionRaw), &decision) == nil {
				msg.Decision = &decision
			}
		}
	}
	return messages, nil
}

// AddVote adds a vote to a debate session
func (s *DebateStore) AddVote(vote *DebateVote) error {
	if vote.ID == "" {
		vote.ID = uuid.New().String()
	}
	return s.db.Create(vote).Error
}

// GetVotes gets all votes for a debate session
func (s *DebateStore) GetVotes(sessionID string) ([]*DebateVote, error) {
	var votes []*DebateVote
	err := s.db.Where("session_id = ?", sessionID).Order("created_at").Find(&votes).Error
	return votes, err
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
