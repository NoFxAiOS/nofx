package agent

import (
	"context"
	"fmt"
	"log/slog"
	"nofx/safe"
	"strings"
	"sync"
	"time"
)

type Scheduler struct {
	agent    *Agent
	logger   *slog.Logger
	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewScheduler(a *Agent, l *slog.Logger) *Scheduler {
	return &Scheduler{agent: a, logger: l, stopCh: make(chan struct{})}
}

func (s *Scheduler) Start(ctx context.Context) {
	safe.GoNamed("agent-scheduler", func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		lastReport := time.Time{}
		lastCheck := time.Time{}
		for {
			select {
			case <-ctx.Done(): return
			case <-s.stopCh: return
			case now := <-ticker.C:
				// Daily report at 21:00
				if now.Hour() == 21 && now.Sub(lastReport) > 12*time.Hour {
					s.dailyReport()
					lastReport = now
				}
				// Position risk check every 4h
				if now.Sub(lastCheck) > 4*time.Hour {
					s.riskCheck()
					lastCheck = now
				}
				// Clean expired pending trades every hour.
				if now.Minute() == 0 {
					if s.agent.pending != nil {
						s.agent.pending.CleanExpired()
					}
				}
			}
		}
	})
}

func (s *Scheduler) Stop() { s.stopOnce.Do(func() { close(s.stopCh) }) }

func (s *Scheduler) dailyReport() {
	if s.agent.traderManager == nil { return }

	traders := s.agent.traderManager.GetAllTraders()
	if len(traders) == 0 { return }

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 *NOFXi 每日报告 — %s*\n\n", time.Now().Format("2006-01-02")))

	totalPnL := 0.0
	for _, t := range traders {
		info, err := t.GetAccountInfo()
		if err != nil { continue }
		equity := toFloat(info["total_equity"])
		pnl := toFloat(info["unrealized_pnl"])
		sb.WriteString(fmt.Sprintf("• %s: $%.2f (P/L: $%.2f)\n", t.GetName(), equity, pnl))
		totalPnL += pnl
	}
	e := "📈"
	if totalPnL < 0 { e = "📉" }
	sb.WriteString(fmt.Sprintf("\n%s Total P/L: $%.2f", e, totalPnL))

	s.agent.notifyAll(sb.String())
}

func (s *Scheduler) riskCheck() {
	if s.agent.traderManager == nil { return }

	var alerts []string
	for _, t := range s.agent.traderManager.GetAllTraders() {
		positions, err := t.GetPositions()
		if err != nil { continue }
		for _, p := range positions {
			pnl := toFloat(p["unrealizedPnl"])
			size := toFloat(p["size"])
			if size == 0 { continue }
			entry := toFloat(p["entryPrice"])
			if entry > 0 {
				pnlPct := (pnl / (entry * size)) * 100
				if pnlPct < -5 {
					alerts = append(alerts, fmt.Sprintf("⚠️ *%s* %s: %.1f%% ($%.2f)",
						p["symbol"], p["side"], pnlPct, pnl))
				}
			}
		}
	}
	if len(alerts) > 0 {
		s.agent.notifyAll("🚨 *持仓风险提醒*\n\n" + strings.Join(alerts, "\n"))
	}
}
