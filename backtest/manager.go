package backtest

import (
	"context"
	"errors"
	"fmt"
	"nofx/logger"
	"os"
	"sort"
	"strings"
	"sync"

	"nofx/mcp"
	"nofx/store"
)

type Manager struct {
	mu         sync.RWMutex
	runners    map[string]*Runner
	metadata   map[string]*RunMetadata
	cancels    map[string]context.CancelFunc
	mcpClient  mcp.AIClient
	aiResolver AIConfigResolver
}

type AIConfigResolver func(*BacktestConfig) error

func NewManager(defaultClient mcp.AIClient) *Manager {
	return &Manager{
		runners:   make(map[string]*Runner),
		metadata:  make(map[string]*RunMetadata),
		cancels:   make(map[string]context.CancelFunc),
		mcpClient: defaultClient,
	}
}

func (m *Manager) SetAIResolver(resolver AIConfigResolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aiResolver = resolver
}

func (m *Manager) Start(ctx context.Context, cfg BacktestConfig) (*Runner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := m.resolveAIConfig(&cfg); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	if existing, ok := m.runners[cfg.RunID]; ok {
		state := existing.Status()
		if state == RunStateRunning || state == RunStatePaused {
			m.mu.Unlock()
			return nil, fmt.Errorf("run %s is already active", cfg.RunID)
		}
	}
	m.mu.Unlock()

	persistCfg := cfg
	persistCfg.AICfg.APIKey = ""
	if err := SaveConfig(cfg.RunID, &persistCfg); err != nil {
		return nil, err
	}

	runner, err := NewRunner(cfg, m.client())
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(ctx)

	m.mu.Lock()
	if _, exists := m.runners[cfg.RunID]; exists {
		m.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("run %s is already active", cfg.RunID)
	}
	m.runners[cfg.RunID] = runner
	m.cancels[cfg.RunID] = cancel
	meta := runner.CurrentMetadata()
	m.metadata[cfg.RunID] = meta
	m.mu.Unlock()

	if err := runner.Start(runCtx); err != nil {
		cancel()
		m.mu.Lock()
		delete(m.runners, cfg.RunID)
		delete(m.cancels, cfg.RunID)
		delete(m.metadata, cfg.RunID)
		m.mu.Unlock()
		runner.releaseLock()
		return nil, err
	}

	m.storeMetadata(cfg.RunID, meta)
	m.launchWatcher(cfg.RunID, runner)
	return runner, nil
}

func (m *Manager) client() mcp.AIClient {
	if m.mcpClient != nil {
		return m.mcpClient
	}
	return mcp.New()
}

func (m *Manager) GetRunner(runID string) (*Runner, bool) {
	m.mu.RLock()
	runner, ok := m.runners[runID]
	m.mu.RUnlock()
	return runner, ok
}

func (m *Manager) ListRuns() ([]*RunMetadata, error) {
	m.mu.RLock()
	localCopy := make(map[string]*RunMetadata, len(m.metadata))
	for k, v := range m.metadata {
		localCopy[k] = v
	}
	m.mu.RUnlock()

	runIDs, err := LoadRunIDs()
	if err != nil {
		return nil, err
	}

	ordered := make([]string, 0, len(runIDs))
	if entries, err := listIndexEntries(); err == nil {
		seen := make(map[string]bool, len(runIDs))
		for _, entry := range entries {
			if contains(runIDs, entry.RunID) {
				ordered = append(ordered, entry.RunID)
				seen[entry.RunID] = true
			}
		}
		for _, id := range runIDs {
			if !seen[id] {
				ordered = append(ordered, id)
			}
		}
	} else {
		ordered = append(ordered, runIDs...)
	}

	metas := make([]*RunMetadata, 0, len(runIDs))
	for _, runID := range ordered {
		if meta, ok := localCopy[runID]; ok {
			metas = append(metas, meta)
			continue
		}
		meta, err := LoadRunMetadata(runID)
		if err == nil {
			metas = append(metas, meta)
		}
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})

	return metas, nil
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func (m *Manager) Pause(runID string) error {
	runner, ok := m.GetRunner(runID)
	if !ok {
		return fmt.Errorf("run %s not found", runID)
	}
	runner.Pause()
	m.refreshMetadata(runID)
	return nil
}

func (m *Manager) Resume(runID string) error {
	if runID == "" {
		return fmt.Errorf("run_id is required")
	}

	runner, ok := m.GetRunner(runID)
	if ok {
		runner.Resume()
		m.refreshMetadata(runID)
		return nil
	}

	cfg, err := LoadConfig(runID)
	if err != nil {
		return err
	}
	cfgCopy := *cfg
	if err := cfgCopy.Validate(); err != nil {
		return err
	}
	if err := m.resolveAIConfig(&cfgCopy); err != nil {
		return err
	}

	restored, err := NewRunner(cfgCopy, m.client())
	if err != nil {
		return err
	}
	if err := restored.RestoreFromCheckpoint(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	if _, exists := m.runners[runID]; exists {
		m.mu.Unlock()
		cancel()
		return fmt.Errorf("run %s is already active", runID)
	}
	m.runners[runID] = restored
	m.cancels[runID] = cancel
	m.metadata[runID] = restored.CurrentMetadata()
	m.mu.Unlock()

	if err := restored.Start(ctx); err != nil {
		cancel()
		m.mu.Lock()
		delete(m.runners, runID)
		delete(m.cancels, runID)
		delete(m.metadata, runID)
		m.mu.Unlock()
		restored.releaseLock()
		return err
	}

	m.storeMetadata(runID, restored.CurrentMetadata())
	m.launchWatcher(runID, restored)
	return nil
}

func (m *Manager) Stop(runID string) error {
	runner, ok := m.GetRunner(runID)
	if ok {
		runner.Stop()
		err := runner.Wait()
		m.refreshMetadata(runID)
		return err
	}
	meta, err := m.LoadMetadata(runID)
	if err != nil {
		return err
	}
	if meta.State == RunStateStopped || meta.State == RunStateCompleted {
		return nil
	}
	meta.State = RunStateStopped
	m.storeMetadata(runID, meta)
	return nil
}

func (m *Manager) Wait(runID string) error {
	runner, ok := m.GetRunner(runID)
	if !ok {
		return fmt.Errorf("run %s not found", runID)
	}
	err := runner.Wait()
	m.refreshMetadata(runID)
	return err
}

func (m *Manager) UpdateLabel(runID, label string) (*RunMetadata, error) {
	meta, err := m.LoadMetadata(runID)
	if err != nil {
		return nil, err
	}
	clean := strings.TrimSpace(label)
	metaCopy := *meta
	metaCopy.Label = clean
	m.storeMetadata(runID, &metaCopy)
	return &metaCopy, nil
}

func (m *Manager) Delete(runID string) error {
	runner, ok := m.GetRunner(runID)
	if ok {
		runner.Stop()
		_ = runner.Wait()
	}
	m.mu.Lock()
	if cancel, ok := m.cancels[runID]; ok {
		cancel()
		delete(m.cancels, runID)
	}
	delete(m.runners, runID)
	delete(m.metadata, runID)
	m.mu.Unlock()
	if err := removeFromRunIndex(runID); err != nil {
		return err
	}
	if err := deleteRunLock(runID); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (m *Manager) LoadMetadata(runID string) (*RunMetadata, error) {
	runner, ok := m.GetRunner(runID)
	if ok {
		meta := runner.CurrentMetadata()
		m.storeMetadata(runID, meta)
		return meta, nil
	}
	meta, err := LoadRunMetadata(runID)
	if err != nil {
		return nil, err
	}
	m.storeMetadata(runID, meta)
	return meta, nil
}

func (m *Manager) LoadEquity(runID string, timeframe string, limit int) ([]EquityPoint, error) {
	points, err := LoadEquityPoints(runID)
	if err != nil {
		return nil, err
	}
	if timeframe != "" {
		points, err = ResampleEquity(points, timeframe)
		if err != nil {
			return nil, err
		}
	}
	points = AlignEquityTimestamps(points)
	points = LimitEquityPoints(points, limit)
	return points, nil
}

func (m *Manager) LoadTrades(runID string, limit int) ([]TradeEvent, error) {
	events, err := LoadTradeEvents(runID)
	if err != nil {
		return nil, err
	}
	return LimitTradeEvents(events, limit), nil
}

func (m *Manager) GetMetrics(runID string) (*Metrics, error) {
	return LoadMetrics(runID)
}

func (m *Manager) Cleanup(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runners, runID)
	if cancel, ok := m.cancels[runID]; ok {
		cancel()
		delete(m.cancels, runID)
	}
}

func (m *Manager) Status(runID string) *StatusPayload {
	runner, ok := m.GetRunner(runID)
	if !ok {
		return nil
	}
	payload := runner.StatusPayload()
	m.storeMetadata(runID, runner.CurrentMetadata())
	return &payload
}

func (m *Manager) launchWatcher(runID string, runner *Runner) {
	go func() {
		if err := runner.Wait(); err != nil {
			logger.Infof("backtest run %s finished with error: %v", runID, err)
		}
		runner.PersistMetadata()
		meta := runner.CurrentMetadata()
		m.storeMetadata(runID, meta)

		m.mu.Lock()
		if cancel, ok := m.cancels[runID]; ok {
			cancel()
			delete(m.cancels, runID)
		}
		delete(m.runners, runID)
		m.mu.Unlock()
	}()
}

func (m *Manager) refreshMetadata(runID string) {
	runner, ok := m.GetRunner(runID)
	if !ok {
		return
	}
	meta := runner.CurrentMetadata()
	m.storeMetadata(runID, meta)
}

func (m *Manager) storeMetadata(runID string, meta *RunMetadata) {
	if meta == nil {
		return
	}
	m.mu.Lock()
	if existing, ok := m.metadata[runID]; ok {
		if meta.Label == "" && existing.Label != "" {
			meta.Label = existing.Label
		}
		if meta.LastError == "" && existing.LastError != "" {
			meta.LastError = existing.LastError
		}
	}
	m.metadata[runID] = meta
	m.mu.Unlock()
	_ = SaveRunMetadata(meta)
	if err := updateRunIndex(meta, nil); err != nil {
		logger.Infof("failed to update run index for %s: %v", runID, err)
	}
}

func (m *Manager) resolveAIConfig(cfg *BacktestConfig) error {
	if cfg == nil {
		return fmt.Errorf("ai config missing")
	}
	provider := strings.TrimSpace(cfg.AICfg.Provider)
	apiKey := strings.TrimSpace(cfg.AICfg.APIKey)
	if provider != "" && !strings.EqualFold(provider, "inherit") && apiKey != "" {
		return nil
	}

	m.mu.RLock()
	resolver := m.aiResolver
	m.mu.RUnlock()
	if resolver == nil {
		if apiKey == "" {
			return fmt.Errorf("AI配置缺少密钥且未配置解析器")
		}
		return nil
	}
	return resolver(cfg)
}

func (m *Manager) GetTrace(runID string, cycle int) (*store.DecisionRecord, error) {
	return LoadDecisionTrace(runID, cycle)
}

func (m *Manager) ExportRun(runID string) (string, error) {
	return CreateRunExport(runID)
}

// RestoreRunsFromDisk 扫描 backtests 目录并恢复现有 run 的元数据（服务重启场景）。
func (m *Manager) RestoreRuns() error {
	runIDs, err := LoadRunIDs()
	if err != nil {
		return err
	}
	for _, runID := range runIDs {
		meta, err := LoadRunMetadata(runID)
		if err != nil {
			logger.Infof("skip run %s: %v", runID, err)
			continue
		}
		if meta.State == RunStateRunning {
			lock, err := loadRunLock(runID)
			if err != nil || lockIsStale(lock) {
				if err := deleteRunLock(runID); err != nil {
					logger.Infof("failed to cleanup lock for %s: %v", runID, err)
				}
				meta.State = RunStatePaused
				if err := SaveRunMetadata(meta); err != nil {
					logger.Infof("failed to mark %s paused: %v", runID, err)
				}
			}
		}
		m.mu.Lock()
		m.metadata[runID] = meta
		m.mu.Unlock()
		if err := updateRunIndex(meta, nil); err != nil {
			logger.Infof("failed to sync index for %s: %v", runID, err)
		}
	}
	return nil
}

// RestoreRunsFromDisk 保留旧方法名，兼容历史调用。
func (m *Manager) RestoreRunsFromDisk() error {
	return m.RestoreRuns()
}
