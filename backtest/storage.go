package backtest

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nofx/store"
)

const (
	backtestsRootDir = "backtests"
)

type progressPayload struct {
	BarIndex     int     `json:"bar_index"`
	Equity       float64 `json:"equity"`
	ProgressPct  float64 `json:"progress_pct"`
	Liquidated   bool    `json:"liquidated"`
	UpdatedAtISO string  `json:"updated_at_iso"`
}

func runDir(runID string) string {
	return filepath.Join(backtestsRootDir, runID)
}

func ensureRunDir(runID string) error {
	dir := runDir(runID)
	return os.MkdirAll(dir, 0o755)
}

func checkpointPath(runID string) string {
	return filepath.Join(runDir(runID), "checkpoint.json")
}

func runMetadataPath(runID string) string {
	return filepath.Join(runDir(runID), "run.json")
}

func equityLogPath(runID string) string {
	return filepath.Join(runDir(runID), "equity.jsonl")
}

func tradesLogPath(runID string) string {
	return filepath.Join(runDir(runID), "trades.jsonl")
}

func metricsPath(runID string) string {
	return filepath.Join(runDir(runID), "metrics.json")
}

func progressPath(runID string) string {
	return filepath.Join(runDir(runID), "progress.json")
}

func decisionLogDir(runID string) string {
	return filepath.Join(runDir(runID), "decision_logs")
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

func appendJSONLine(path string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if err := writer.WriteByte('\n'); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return f.Sync()
}

// SaveCheckpoint 将检查点写入磁盘。
func SaveCheckpoint(runID string, ckpt *Checkpoint) error {
	if ckpt == nil {
		return fmt.Errorf("checkpoint is nil")
	}
	if usingDB() {
		return saveCheckpointDB(runID, ckpt)
	}
	return writeJSONAtomic(checkpointPath(runID), ckpt)
}

// LoadCheckpoint 读取最近一次检查点。
func LoadCheckpoint(runID string) (*Checkpoint, error) {
	if usingDB() {
		return loadCheckpointDB(runID)
	}
	path := checkpointPath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ckpt Checkpoint
	if err := json.Unmarshal(data, &ckpt); err != nil {
		return nil, err
	}
	return &ckpt, nil
}

// SaveRunMetadata 写入 run.json。
func SaveRunMetadata(meta *RunMetadata) error {
	if meta == nil {
		return fmt.Errorf("run metadata is nil")
	}
	if meta.Version == 0 {
		meta.Version = 1
	}
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now().UTC()
	}
	meta.UpdatedAt = time.Now().UTC()
	if usingDB() {
		return saveRunMetadataDB(meta)
	}
	return writeJSONAtomic(runMetadataPath(meta.RunID), meta)
}

// LoadRunMetadata 读取 run.json。
func LoadRunMetadata(runID string) (*RunMetadata, error) {
	if usingDB() {
		return loadRunMetadataDB(runID)
	}
	path := runMetadataPath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta RunMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func appendEquityPoint(runID string, point EquityPoint) error {
	if usingDB() {
		return appendEquityPointDB(runID, point)
	}
	return appendJSONLine(equityLogPath(runID), point)
}

func appendTradeEvent(runID string, event TradeEvent) error {
	if usingDB() {
		return appendTradeEventDB(runID, event)
	}
	return appendJSONLine(tradesLogPath(runID), event)
}

func saveMetrics(runID string, metrics *Metrics) error {
	if metrics == nil {
		return fmt.Errorf("metrics is nil")
	}
	if usingDB() {
		return saveMetricsDB(runID, metrics)
	}
	return writeJSONAtomic(metricsPath(runID), metrics)
}

func saveProgress(runID string, state *BacktestState, cfg *BacktestConfig) error {
	if state == nil || cfg == nil {
		return fmt.Errorf("state or config nil")
	}
	dur := cfg.Duration()
	progress := 0.0
	if dur > 0 {
		current := time.UnixMilli(state.BarTimestamp)
		start := time.Unix(cfg.StartTS, 0)
		if current.After(start) {
			elapsed := current.Sub(start)
			progress = float64(elapsed) / float64(dur)
		}
	}
	payload := progressPayload{
		BarIndex:    state.BarIndex,
		Equity:      state.Equity,
		ProgressPct: progress * 100,
		Liquidated:  state.Liquidated,

		UpdatedAtISO: time.Now().UTC().Format(time.RFC3339),
	}
	if usingDB() {
		return saveProgressDB(runID, payload)
	}
	return writeJSONAtomic(progressPath(runID), payload)
}

func SaveConfig(runID string, cfg *BacktestConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	persist := *cfg
	persist.AICfg.APIKey = ""
	if usingDB() {
		return saveConfigDB(runID, &persist)
	}
	if err := ensureRunDir(runID); err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(runDir(runID), "config.json"), &persist)
}

func LoadConfig(runID string) (*BacktestConfig, error) {
	if usingDB() {
		return loadConfigDB(runID)
	}
	data, err := os.ReadFile(filepath.Join(runDir(runID), "config.json"))
	if err != nil {
		return nil, err
	}
	var cfg BacktestConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadEquityPoints(runID string) ([]EquityPoint, error) {
	if usingDB() {
		return loadEquityPointsDB(runID)
	}
	points, err := loadJSONLines[EquityPoint](equityLogPath(runID))
	if err != nil {
		return nil, err
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp < points[j].Timestamp
	})
	return points, nil
}

func LoadTradeEvents(runID string) ([]TradeEvent, error) {
	if usingDB() {
		return loadTradeEventsDB(runID)
	}
	events, err := loadJSONLines[TradeEvent](tradesLogPath(runID))
	if err != nil {
		return nil, err
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp == events[j].Timestamp {
			return events[i].Symbol < events[j].Symbol
		}
		return events[i].Timestamp < events[j].Timestamp
	})
	return events, nil
}

func LoadMetrics(runID string) (*Metrics, error) {
	if usingDB() {
		return loadMetricsDB(runID)
	}
	data, err := os.ReadFile(metricsPath(runID))
	if err != nil {
		return nil, err
	}
	var metrics Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, err
	}
	return &metrics, nil
}

func LoadRunIDs() ([]string, error) {
	if usingDB() {
		return loadRunIDsDB()
	}
	entries, err := os.ReadDir(backtestsRootDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	runIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			runIDs = append(runIDs, entry.Name())
		}
	}
	sort.Strings(runIDs)
	return runIDs, nil
}

func loadJSONLines[T any](path string) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []T{}, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	var result []T
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
func PersistMetrics(runID string, metrics *Metrics) error {
	return saveMetrics(runID, metrics)
}

func LoadDecisionTrace(runID string, cycle int) (*store.DecisionRecord, error) {
	if usingDB() {
		return loadDecisionTraceDB(runID, cycle)
	}
	dir := decisionLogDir(runID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		path string
		info os.DirEntry
	}
	cands := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "decision_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		cands = append(cands, candidate{path: filepath.Join(dir, name), info: entry})
	}
	sort.Slice(cands, func(i, j int) bool {
		infoI, _ := cands[i].info.Info()
		infoJ, _ := cands[j].info.Info()
		if infoI == nil || infoJ == nil {
			return cands[i].path > cands[j].path
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	for _, cand := range cands {
		data, err := os.ReadFile(cand.path)
		if err != nil {
			continue
		}
		var record store.DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		if cycle <= 0 || record.CycleNumber == cycle {
			return &record, nil
		}
	}
	return nil, fmt.Errorf("decision trace not found for run %s cycle %d", runID, cycle)
}

func LoadDecisionRecords(runID string, limit, offset int) ([]*store.DecisionRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if usingDB() {
		return loadDecisionRecordsDB(runID, limit, offset)
	}
	dir := decisionLogDir(runID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []*store.DecisionRecord{}, nil
		}
		return nil, err
	}
	type fileEntry struct {
		path string
		info os.DirEntry
	}
	files := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "decision_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		files = append(files, fileEntry{path: filepath.Join(dir, name), info: entry})
	}
	sort.Slice(files, func(i, j int) bool {
		infoI, _ := files[i].info.Info()
		infoJ, _ := files[j].info.Info()
		if infoI == nil || infoJ == nil {
			return files[i].path > files[j].path
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
	if offset >= len(files) {
		return []*store.DecisionRecord{}, nil
	}
	end := offset + limit
	if end > len(files) {
		end = len(files)
	}
	records := make([]*store.DecisionRecord, 0, end-offset)
	for _, file := range files[offset:end] {
		data, err := os.ReadFile(file.path)
		if err != nil {
			continue
		}
		var record store.DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		records = append(records, &record)
	}
	return records, nil
}

func CreateRunExport(runID string) (string, error) {
	if usingDB() {
		return createRunExportDB(runID)
	}
	root := runDir(runID)
	if _, err := os.Stat(root); err != nil {
		return "", err
	}
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.zip", runID))
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	zipWriter := zip.NewWriter(tmpFile)
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(writer, src); err != nil {
			src.Close()
			return err
		}
		src.Close()
		return nil
	})
	if err != nil {
		zipWriter.Close()
		return "", err
	}
	if err := zipWriter.Close(); err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

func persistDecisionRecord(runID string, record *store.DecisionRecord) {
	if !usingDB() || record == nil {
		return
	}
	_ = saveDecisionRecordDB(runID, record)
}
