package backtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const runIndexFile = "index.json"

type RunIndexEntry struct {
	RunID          string   `json:"run_id"`
	State          RunState `json:"state"`
	Symbols        []string `json:"symbols"`
	DecisionTF     string   `json:"decision_tf"`
	StartTS        int64    `json:"start_ts"`
	EndTS          int64    `json:"end_ts"`
	EquityLast     float64  `json:"equity_last"`
	MaxDrawdownPct float64  `json:"max_dd_pct"`
	CreatedAtISO   string   `json:"created_at"`
	UpdatedAtISO   string   `json:"updated_at"`
}

type RunIndex struct {
	Runs      map[string]RunIndexEntry `json:"runs"`
	UpdatedAt string                   `json:"updated_at"`
}

func runIndexPath() string {
	return filepath.Join(backtestsRootDir, runIndexFile)
}

func loadRunIndex() (*RunIndex, error) {
	if usingDB() {
		entries, err := listIndexEntriesDB()
		if err != nil {
			return nil, err
		}
		idx := &RunIndex{
			Runs:      make(map[string]RunIndexEntry),
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		for _, entry := range entries {
			idx.Runs[entry.RunID] = entry
		}
		return idx, nil
	}
	path := runIndexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &RunIndex{Runs: make(map[string]RunIndexEntry)}, nil
		}
		return nil, err
	}
	var idx RunIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	if idx.Runs == nil {
		idx.Runs = make(map[string]RunIndexEntry)
	}
	return &idx, nil
}

func saveRunIndex(idx *RunIndex) error {
	if usingDB() {
		return nil
	}
	if idx == nil {
		return fmt.Errorf("index is nil")
	}
	idx.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONAtomic(runIndexPath(), idx)
}

func updateRunIndex(meta *RunMetadata, cfg *BacktestConfig) error {
	if usingDB() {
		enforceRetention(maxCompletedRuns)
		return nil
	}
	if meta == nil {
		return fmt.Errorf("meta nil")
	}
	if cfg == nil {
		var err error
		cfg, err = LoadConfig(meta.RunID)
		if err != nil {
			return err
		}
	}

	idx, err := loadRunIndex()
	if err != nil {
		return err
	}

	entry := RunIndexEntry{
		RunID:          meta.RunID,
		State:          meta.State,
		Symbols:        append([]string(nil), cfg.Symbols...),
		DecisionTF:     meta.Summary.DecisionTF,
		StartTS:        cfg.StartTS,
		EndTS:          cfg.EndTS,
		EquityLast:     meta.Summary.EquityLast,
		MaxDrawdownPct: meta.Summary.MaxDrawdownPct,
		CreatedAtISO:   meta.CreatedAt.Format(time.RFC3339),
		UpdatedAtISO:   meta.UpdatedAt.Format(time.RFC3339),
	}

	if idx.Runs == nil {
		idx.Runs = make(map[string]RunIndexEntry)
	}
	idx.Runs[meta.RunID] = entry
	if err := saveRunIndex(idx); err != nil {
		return err
	}
	enforceRetention(maxCompletedRuns)
	return nil
}

func removeFromRunIndex(runID string) error {
	if usingDB() {
		if err := deleteRunDB(runID); err != nil {
			return err
		}
		return os.RemoveAll(runDir(runID))
	}
	idx, err := loadRunIndex()
	if err != nil {
		return err
	}
	if idx.Runs == nil {
		return nil
	}
	delete(idx.Runs, runID)
	return saveRunIndex(idx)
}

func listIndexEntries() ([]RunIndexEntry, error) {
	if usingDB() {
		return listIndexEntriesDB()
	}
	idx, err := loadRunIndex()
	if err != nil {
		return nil, err
	}
	entries := make([]RunIndexEntry, 0, len(idx.Runs))
	for _, entry := range idx.Runs {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UpdatedAtISO > entries[j].UpdatedAtISO
	})
	return entries, nil
}
