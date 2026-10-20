package kernel

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nofx/logger"
	"nofx/market"
)

// FactorEnsembleRef is opt-in research evidence only. No gate, score,
// candidate filter, or execution decision reads it.
type FactorEnsembleRef struct {
	AsOf     string                      `json:"as_of"`
	Horizons map[string]FactorHorizonRef `json:"horizons"`
}

type FactorHorizonRef struct {
	Pct     float64 `json:"pct"`
	Z       float64 `json:"z"`
	Factors int     `json:"factors"`
	Forward string  `json:"forward"`
}

const factorEnsembleMaxFileSize = 8 << 20

type factorEnsembleScores struct {
	Version  int    `json:"version"`
	Market   string `json:"market"`
	AsOf     string `json:"as_of"`
	Horizons map[string]struct {
		FactorCount int                        `json:"factor_count"`
		Forward     map[string]int             `json:"forward"`
		Scores      map[string]json.RawMessage `json:"scores"`
	} `json:"horizons"`
	asOfDate time.Time
}

type factorEnsembleFileCache struct {
	mu       sync.Mutex
	path     string
	mtime    time.Time
	scores   *factorEnsembleScores // immutable after publication
	lastWarn time.Time
}

var liveFactorEnsembleCache factorEnsembleFileCache

func factorEnsembleScoresPath() string {
	dir := os.Getenv("FACTOR_MINING_DIR")
	if dir == "" {
		dir = "data/factor_mining"
	}
	return filepath.Join(dir, "crypto", "live_scores.json")
}

// load accepts a path so tests can use isolated files. Invalid snapshots are
// cached too; a changed mtime retries them without serving the previous file.
func (c *factorEnsembleFileCache) load(path string) *factorEnsembleScores {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		c.path, c.mtime, c.scores = "", time.Time{}, nil
		c.warnLocked(path, err)
		return nil
	}
	if c.path == path && c.mtime.Equal(info.ModTime()) {
		return c.scores
	}
	c.path, c.mtime, c.scores = path, info.ModTime(), nil
	if !info.Mode().IsRegular() || info.Size() > factorEnsembleMaxFileSize {
		c.warnLocked(path, fmt.Errorf("expected a regular file of at most 8 MiB"))
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		c.warnLocked(path, err)
		return nil
	}
	defer f.Close()
	// The nightly writer can replace the file between Stat and Open.
	info, err = f.Stat()
	if err != nil {
		c.warnLocked(path, err)
		return nil
	}
	c.mtime = info.ModTime()
	if !info.Mode().IsRegular() || info.Size() > factorEnsembleMaxFileSize {
		c.warnLocked(path, fmt.Errorf("expected a regular file of at most 8 MiB"))
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(f, factorEnsembleMaxFileSize+1))
	if err != nil {
		c.warnLocked(path, err)
		return nil
	}
	if len(raw) > factorEnsembleMaxFileSize {
		c.warnLocked(path, fmt.Errorf("file exceeds 8 MiB"))
		return nil
	}
	var scores factorEnsembleScores
	if err := json.Unmarshal(raw, &scores); err != nil {
		c.warnLocked(path, err)
		return nil
	}
	if scores.Version != 1 || scores.Market != "crypto" {
		c.warnLocked(path, fmt.Errorf("expected version 1 and market crypto"))
		return nil
	}
	scores.asOfDate, err = time.Parse(time.DateOnly, scores.AsOf)
	if err != nil {
		c.warnLocked(path, err)
		return nil
	}
	c.scores = &scores
	return c.scores
}

// Called only with c.mu held; all failure types share one hourly budget.
func (c *factorEnsembleFileCache) warnLocked(path string, err error) {
	now := time.Now()
	if c.lastWarn.IsZero() || now.Sub(c.lastWarn) >= time.Hour {
		c.lastWarn = now
		logger.Warnf("[factor-ensemble] Ignoring %s: %v", path, err)
	}
}

func computeFactorEnsemble(data *market.Data, now time.Time) *FactorEnsembleRef {
	if data == nil {
		return nil
	}
	scores := liveFactorEnsembleCache.load(factorEnsembleScoresPath())
	if scores == nil {
		return nil
	}
	// Recheck age even on cache hits: an unchanged file expires at UTC midnight.
	utc := now.UTC()
	today := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	if scores.asOfDate.Before(today.AddDate(0, 0, -3)) {
		return nil
	}
	ref := &FactorEnsembleRef{AsOf: scores.AsOf, Horizons: make(map[string]FactorHorizonRef)}
	for horizon, h := range scores.Horizons {
		raw, ok := h.Scores[data.Symbol]
		if !ok {
			continue
		}
		// Decode separately so overflow in one score cannot discard other
		// horizons. Pointers distinguish missing/null values from genuine zero.
		var score struct {
			Pct *float64 `json:"pct"`
			Z   *float64 `json:"z"`
		}
		if json.Unmarshal(raw, &score) != nil || score.Pct == nil || score.Z == nil {
			continue
		}
		pct, z := *score.Pct, *score.Z
		if math.IsNaN(pct) || math.IsInf(pct, 0) || math.IsNaN(z) || math.IsInf(z, 0) || pct < 0 || pct > 1 {
			continue
		}
		verdicts := make([]string, 0, len(h.Forward))
		for verdict := range h.Forward {
			verdicts = append(verdicts, verdict)
		}
		sort.Strings(verdicts)
		parts := make([]string, 0, len(verdicts))
		for _, verdict := range verdicts {
			parts = append(parts, fmt.Sprintf("%s×%d", verdict, h.Forward[verdict]))
		}
		ref.Horizons[horizon] = FactorHorizonRef{
			Pct: pct, Z: z, Factors: h.FactorCount, Forward: strings.Join(parts, ","),
		}
	}
	if len(ref.Horizons) == 0 {
		return nil
	}
	return ref
}
