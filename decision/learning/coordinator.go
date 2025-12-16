package learning

import (
	"log"
	"nofx/config"
	"nofx/decision/analysis"
	"nofx/decision/optimizer"
	"nofx/decision/reflection"
	"nofx/manager"
	"time"
)

// LearningCoordinator orchestrates the learning loop.
type LearningCoordinator struct {
	analyzer  Analyzer
	detector  Detector
	generator Generator
	executor  Executor
	db        ConfigDB
}

// NewLearningCoordinator creates a new LearningCoordinator with all dependencies.
func NewLearningCoordinator(db *config.Database, tm *manager.TraderManager, aiClient reflection.AIClient) *LearningCoordinator {
	// Initialize components
	analyzer := analysis.NewTradeAnalyzer(db)
	detector := analysis.NewPatternDetector()
	generator := reflection.NewReflectionGenerator(aiClient)

	// Optimizer needs RealTraderManager adapter
	traderManagerAdapter := optimizer.NewRealTraderManager(tm)
	opt := optimizer.NewParameterOptimizer(db, traderManagerAdapter)
	
	executor := reflection.NewReflectionExecutor(db, opt)

	return &LearningCoordinator{
		analyzer:  analyzer,
		detector:  detector,
		generator: generator,
		executor:  executor,
		db:        db,
	}
}

// RunLearningCycle executes the full learning loop for a trader.
func (lc *LearningCoordinator) RunLearningCycle(traderID string) error {
	log.Printf("🧠 Starting Learning Cycle for Trader %s", traderID)

	// 1. Analyze (Last 7 days)
	endDate := time.Now()
	startDate := endDate.Add(-7 * 24 * time.Hour)
	stats, err := lc.analyzer.AnalyzeTradesForPeriod(traderID, startDate, endDate)
	if err != nil {
		return err
	}

	// 2. Detect Patterns
	patterns := lc.detector.DetectFailurePatterns(stats)
	log.Printf("  • Detected %d failure patterns", len(patterns))

	// 3. Generate Reflections
	reflections, err := lc.generator.GenerateReflections(traderID, stats, patterns)
	if err != nil {
		return err
	}
	log.Printf("  • Generated %d reflections", len(reflections))

	// 4. Save & Execute
	for _, r := range reflections {
		// Convert to DTO
		rr := &config.ReflectionRecord{
			ID:                 r.ID,
			TraderID:           r.TraderID,
			ReflectionType:     r.ReflectionType,
			Severity:           r.Severity,
			ProblemTitle:       r.ProblemTitle,
			ProblemDescription: r.ProblemDescription,
			RootCause:          r.RootCause,
			RecommendedAction:  r.RecommendedAction,
			Priority:           r.Priority,
			IsApplied:          r.IsApplied,
			CreatedAt:          r.CreatedAt,
		}

		if err := lc.db.SaveReflection(rr); err != nil {
			log.Printf("  ❌ Failed to save reflection: %v", err)
			continue
		}

		// Auto-execute if high priority
		if r.Priority >= 8 {
			log.Printf("  ⚡ Auto-executing high priority reflection: %s", r.ProblemTitle)
			if err := lc.executor.ApplyReflection(r); err != nil {
				log.Printf("  ❌ Failed to apply reflection %s: %v", r.ID, err)
			}
		}
	}

	return nil
}

// StartScheduler starts the periodic learning process.
func (lc *LearningCoordinator) StartScheduler() {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for range ticker.C {
			log.Println("⏰ Running Scheduled Learning Cycles...")
			traders, err := lc.db.GetActiveTraders()
			if err != nil {
				log.Printf("Learning Scheduler: Failed to get traders: %v", err)
				continue
			}
			for _, t := range traders {
				if err := lc.RunLearningCycle(t.ID); err != nil {
					log.Printf("Learning Cycle failed for %s: %v", t.Name, err)
				}
			}
		}
	}()
}