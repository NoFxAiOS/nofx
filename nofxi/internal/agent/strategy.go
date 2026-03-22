package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// StrategyRunner manages automated trading strategies.
type StrategyRunner struct {
	agent  *Agent
	logger *slog.Logger
	stopCh chan struct{}

	// Active strategies
	activeStrategies map[string]*RunningStrategy
}

// RunningStrategy represents an active automated strategy.
type RunningStrategy struct {
	ID       string
	Name     string
	Symbol   string
	Interval time.Duration
	Exchange string
	StopCh   chan struct{}
	Running  bool
}

// NewStrategyRunner creates a new strategy runner.
func NewStrategyRunner(agent *Agent, logger *slog.Logger) *StrategyRunner {
	return &StrategyRunner{
		agent:            agent,
		logger:           logger,
		stopCh:           make(chan struct{}),
		activeStrategies: make(map[string]*RunningStrategy),
	}
}

// StartStrategy begins an AI-driven trading strategy.
// The AI will periodically analyze the market and suggest/execute trades.
func (r *StrategyRunner) StartStrategy(name, symbol, exchange string, interval time.Duration) (string, error) {
	id := fmt.Sprintf("%s-%s-%d", strings.ToLower(symbol), strings.ToLower(exchange), time.Now().Unix())

	if _, exists := r.activeStrategies[id]; exists {
		return "", fmt.Errorf("strategy already running: %s", id)
	}

	strategy := &RunningStrategy{
		ID:       id,
		Name:     name,
		Symbol:   symbol,
		Interval: interval,
		Exchange: exchange,
		StopCh:   make(chan struct{}),
		Running:  true,
	}

	r.activeStrategies[id] = strategy

	go r.runStrategy(strategy)

	r.logger.Info("strategy started",
		"id", id,
		"symbol", symbol,
		"exchange", exchange,
		"interval", interval,
	)

	return id, nil
}

// StopStrategy stops a running strategy.
func (r *StrategyRunner) StopStrategy(id string) error {
	s, ok := r.activeStrategies[id]
	if !ok {
		return fmt.Errorf("strategy not found: %s", id)
	}
	close(s.StopCh)
	s.Running = false
	delete(r.activeStrategies, id)
	r.logger.Info("strategy stopped", "id", id)
	return nil
}

// StopAll stops all running strategies.
func (r *StrategyRunner) StopAll() {
	for id, s := range r.activeStrategies {
		close(s.StopCh)
		s.Running = false
		r.logger.Info("strategy stopped", "id", id)
	}
	r.activeStrategies = make(map[string]*RunningStrategy)
}

// ListStrategies returns all active strategies.
func (r *StrategyRunner) ListStrategies() []*RunningStrategy {
	result := make([]*RunningStrategy, 0, len(r.activeStrategies))
	for _, s := range r.activeStrategies {
		result = append(result, s)
	}
	return result
}

func (r *StrategyRunner) runStrategy(s *RunningStrategy) {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	// Initial analysis
	r.executeStrategyTick(s)

	for {
		select {
		case <-s.StopCh:
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.executeStrategyTick(s)
		}
	}
}

func (r *StrategyRunner) executeStrategyTick(s *RunningStrategy) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r.logger.Info("strategy tick", "id", s.ID, "symbol", s.Symbol)

	// Get AI analysis
	prompt := fmt.Sprintf(
		"You are running an automated trading strategy for %s on %s.\n"+
			"Analyze the current market and decide: should we BUY, SELL, or HOLD?\n"+
			"Consider risk management. Only trade on high confidence signals.\n"+
			"Respond with a brief analysis and your recommendation.",
		s.Symbol, s.Exchange,
	)

	analysis, err := r.agent.thinker.Analyze(ctx, prompt)
	if err != nil {
		r.logger.Error("strategy analysis failed", "id", s.ID, "error", err)
		return
	}

	r.logger.Info("strategy analysis",
		"id", s.ID,
		"action", analysis.Action,
		"confidence", analysis.Confidence,
	)

	// Only execute on high confidence
	if analysis.Confidence < 0.75 {
		r.logger.Info("strategy: confidence too low, holding", "id", s.ID)
		return
	}

	// Notify user about the signal
	if r.agent.NotifyFunc != nil {
		msg := fmt.Sprintf("🤖 *Strategy Signal: %s*\n\n"+
			"Symbol: %s\n"+
			"Action: %s\n"+
			"Confidence: %.0f%%\n\n"+
			"%s",
			s.Name, s.Symbol,
			strings.ToUpper(analysis.Action),
			analysis.Confidence*100,
			analysis.Reasoning,
		)
		for _, uid := range r.agent.config.Telegram.AllowedIDs {
			r.agent.NotifyFunc(uid, msg)
		}
	}

	// TODO: Auto-execute trades based on strategy config
	// For now, just notify. Users can enable auto-execution in Phase 4.
}

// FormatStrategyList formats active strategies for display.
func (r *StrategyRunner) FormatStrategyList() string {
	strategies := r.ListStrategies()
	if len(strategies) == 0 {
		return "📭 No active strategies.\n\nUse `/strategy start BTC 1h` to start one."
	}

	var sb strings.Builder
	sb.WriteString("🤖 *Active Strategies*\n\n")
	for _, s := range strategies {
		status := "🟢"
		if !s.Running {
			status = "🔴"
		}
		sb.WriteString(fmt.Sprintf("%s *%s* — %s on %s (every %s)\n   ID: `%s`\n\n",
			status, s.Name, s.Symbol, s.Exchange, s.Interval, s.ID))
	}
	sb.WriteString("Stop with: `/strategy stop <id>`")
	return sb.String()
}
