package backtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	lockFileName          = "lock"
	lockHeartbeatInterval = 2 * time.Second
	lockStaleAfter        = 10 * time.Second
)

// RunLockInfo represents the lock file structure for a backtest run.
type RunLockInfo struct {
	RunID         string    `json:"run_id"`
	PID           int       `json:"pid"`
	Host          string    `json:"host"`
	StartedAt     time.Time `json:"started_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

func lockFilePath(runID string) string {
	return filepath.Join(runDir(runID), lockFileName)
}

func loadRunLock(runID string) (*RunLockInfo, error) {
	path := lockFilePath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info RunLockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func saveRunLock(info *RunLockInfo) error {
	if info == nil {
		return fmt.Errorf("lock info nil")
	}
	return writeJSONAtomic(lockFilePath(info.RunID), info)
}

func deleteRunLock(runID string) error {
	err := os.Remove(lockFilePath(runID))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func lockIsStale(info *RunLockInfo) bool {
	if info == nil {
		return true
	}
	return time.Since(info.LastHeartbeat) > lockStaleAfter
}

func acquireRunLock(runID string) (*RunLockInfo, error) {
	if err := ensureRunDir(runID); err != nil {
		return nil, err
	}

	if existing, err := loadRunLock(runID); err == nil {
		if !lockIsStale(existing) {
			return nil, fmt.Errorf("run %s is locked by pid %d", runID, existing.PID)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	host, _ := os.Hostname()
	info := &RunLockInfo{
		RunID:         runID,
		PID:           os.Getpid(),
		Host:          host,
		StartedAt:     time.Now().UTC(),
		LastHeartbeat: time.Now().UTC(),
	}

	if err := saveRunLock(info); err != nil {
		return nil, err
	}
	return info, nil
}

func updateRunLockHeartbeat(info *RunLockInfo) error {
	if info == nil {
		return fmt.Errorf("lock info nil")
	}
	info.LastHeartbeat = time.Now().UTC()
	return saveRunLock(info)
}
