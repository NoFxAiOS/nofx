package backtest

import (
	"nofx/logger"
	"os"
	"sort"
	"time"
)

const maxCompletedRuns = 100

func enforceRetention(maxRuns int) {
	if maxRuns <= 0 {
		return
	}
	if usingDB() {
		enforceRetentionDB(maxRuns)
		return
	}
	idx, err := loadRunIndex()
	if err != nil {
		return
	}

	type wrapped struct {
		entry   RunIndexEntry
		updated time.Time
	}
	finalStates := map[RunState]bool{
		RunStateCompleted:  true,
		RunStateStopped:    true,
		RunStateFailed:     true,
		RunStateLiquidated: true,
	}

	candidates := make([]wrapped, 0)
	for _, entry := range idx.Runs {
		if !finalStates[entry.State] {
			continue
		}
		ts, err := time.Parse(time.RFC3339, entry.UpdatedAtISO)
		if err != nil {
			ts = time.Now()
		}
		candidates = append(candidates, wrapped{entry: entry, updated: ts})
	}
	if len(candidates) <= maxRuns {
		return
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].updated.Before(candidates[j].updated)
	})

	toRemove := len(candidates) - maxRuns
	for i := 0; i < toRemove; i++ {
		runID := candidates[i].entry.RunID
		if err := os.RemoveAll(runDir(runID)); err != nil {
			logger.Infof("failed to prune run %s: %v", runID, err)
			continue
		}
		delete(idx.Runs, runID)
	}
	if err := saveRunIndex(idx); err != nil {
		logger.Infof("failed to save index after pruning: %v", err)
	}
}

func enforceRetentionDB(maxRuns int) {
	finalStates := []RunState{
		RunStateCompleted,
		RunStateStopped,
		RunStateFailed,
		RunStateLiquidated,
	}
	query := `
		SELECT run_id FROM backtest_runs
		WHERE state IN (?, ?, ?, ?)
		ORDER BY datetime(updated_at) DESC
		LIMIT -1 OFFSET ?
	`
	rows, err := persistenceDB.Query(query,
		finalStates[0], finalStates[1], finalStates[2], finalStates[3], maxRuns)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var runID string
		if err := rows.Scan(&runID); err != nil {
			continue
		}
		if err := deleteRunDB(runID); err != nil {
			logger.Infof("failed to remove run %s: %v", runID, err)
			continue
		}
		if err := os.RemoveAll(runDir(runID)); err != nil {
			logger.Infof("failed to remove run dir %s: %v", runID, err)
		}
	}
}
