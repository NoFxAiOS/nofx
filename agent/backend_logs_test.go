package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nofx/store"
)

func TestReadBackendLogEntriesReturnsRecentErrorLines(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(tmp) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatalf("MkdirAll(data) error = %v", err)
	}
	logPath := filepath.Join("data", "nofx_2099-01-01.log")
	content := strings.Join([]string{
		"04-19 13:00:00 [INFO] api/server.go:590 API server starting",
		"04-19 13:00:01 [ERRO] api/server.go:600 invalid signature for okx account",
		"04-19 13:00:02 [ERRO] agent/tools.go:123 model update failed: missing api key",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	path, entries, err := readBackendLogEntries(10, "model", true)
	if err != nil {
		t.Fatalf("readBackendLogEntries() error = %v", err)
	}
	if !strings.Contains(path, "nofx_2099-01-01.log") {
		t.Fatalf("unexpected log path: %s", path)
	}
	if len(entries) != 1 || !strings.Contains(entries[0], "missing api key") {
		t.Fatalf("unexpected filtered entries: %#v", entries)
	}
}

func TestToolGetBackendLogsRequiresOwnedTrader(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(tmp) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatalf("MkdirAll(data) error = %v", err)
	}
	logPath := filepath.Join("data", "nofx_2099-01-01.log")
	content := strings.Join([]string{
		"04-19 13:00:00 [INFO] api/server.go:590 API server starting",
		"04-19 13:00:01 [ERRO] trader/runtime.go:88 trader_id=trader-owned strategy execution failed",
		"04-19 13:00:02 [ERRO] trader/runtime.go:89 trader_id=trader-other strategy execution failed",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	a := newTestAgentWithStore(t)
	if err := a.store.Trader().Create(&store.Trader{
		ID:             "trader-owned",
		UserID:         "user-1",
		Name:           "Owned Trader",
		AIModelID:      "model-1",
		ExchangeID:     "exchange-1",
		StrategyID:     "strategy-1",
		InitialBalance: 1000,
	}); err != nil {
		t.Fatalf("create owned trader: %v", err)
	}
	if err := a.store.Trader().Create(&store.Trader{
		ID:             "trader-other",
		UserID:         "user-2",
		Name:           "Other Trader",
		AIModelID:      "model-2",
		ExchangeID:     "exchange-2",
		StrategyID:     "strategy-2",
		InitialBalance: 1000,
	}); err != nil {
		t.Fatalf("create other trader: %v", err)
	}

	resp := a.toolGetBackendLogs("user-1", `{"trader_id":"trader-owned","limit":5}`)
	var okResult struct {
		TraderID string   `json:"trader_id"`
		Entries  []string `json:"entries"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(resp), &okResult); err != nil {
		t.Fatalf("unmarshal owned response: %v\nraw=%s", err, resp)
	}
	if okResult.TraderID != "trader-owned" || okResult.Count != 1 {
		t.Fatalf("unexpected owned response: %+v", okResult)
	}
	if len(okResult.Entries) != 1 || !strings.Contains(okResult.Entries[0], "trader-owned") {
		t.Fatalf("unexpected owned entries: %#v", okResult.Entries)
	}

	resp = a.toolGetBackendLogs("user-1", `{"trader_id":"trader-other","limit":5}`)
	var denied struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp), &denied); err != nil {
		t.Fatalf("unmarshal denied response: %v\nraw=%s", err, resp)
	}
	if denied.Error != "trader not found for current user" {
		t.Fatalf("unexpected denied response: %+v", denied)
	}
}
