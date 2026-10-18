package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"nofx/logger"
)

func factorMiningTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := &Server{}
	r.GET("/api/factor-mining/overview", s.handleFactorMiningOverview)
	r.GET("/api/factor-mining/runs", s.handleFactorMiningRuns)
	return r
}

func writeFactorMiningTestFile(t *testing.T, dir, name, data string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func factorMiningTestGet(t *testing.T, r *gin.Engine, path string) map[string]json.RawMessage {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func assertFactorMiningJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	decode := func(data []byte) any {
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		var value any
		if err := d.Decode(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if !reflect.DeepEqual(decode(got), decode([]byte(want))) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestFactorMiningOverviewComplete(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FACTOR_MINING_DIR", dir)
	files := map[string]string{
		"status.json":          `{"running":true,"future":{"large_id":9007199254740993}}`,
		"crypto/zoo.json":      `{"factors":[{"expr":"Mean($close,5)","new_metric":0.1234567890123456789}]}`,
		"us/zoo.json":          `{"factors":[],"schema_version":42}`,
		"crypto/ensemble.json": `{"horizons":{"3":{"curve":{"val":[["2026-10-18",1.25]]}}}}`,
		"us/ensemble.json":     `{"future_field":[null,true,"值"]}`,
	}
	for name, data := range files {
		writeFactorMiningTestFile(t, dir, name, data)
	}
	body := factorMiningTestGet(t, factorMiningTestRouter(), "/api/factor-mining/overview")
	assertFactorMiningJSON(t, body["status"], files["status.json"])
	for _, group := range []string{"zoo", "ensemble"} {
		var markets map[string]json.RawMessage
		if err := json.Unmarshal(body[group], &markets); err != nil {
			t.Fatal(err)
		}
		for _, market := range []string{"crypto", "us"} {
			assertFactorMiningJSON(t, markets[market], files[market+"/"+group+".json"])
		}
	}
}

func TestFactorMiningOverviewMissing(t *testing.T) {
	t.Setenv("FACTOR_MINING_DIR", filepath.Join(t.TempDir(), "not-created"))
	body := factorMiningTestGet(t, factorMiningTestRouter(), "/api/factor-mining/overview")
	assertFactorMiningJSON(t, body["status"], `null`)
	assertFactorMiningJSON(t, body["zoo"], `{"crypto":null,"us":null}`)
	assertFactorMiningJSON(t, body["ensemble"], `{"crypto":null,"us":null}`)
}

func TestFactorMiningOverviewCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FACTOR_MINING_DIR", dir)
	var logs bytes.Buffer
	oldOutput := logger.Log.Out
	logger.Log.SetOutput(&logs)
	t.Cleanup(func() { logger.Log.SetOutput(oldOutput) })
	files := map[string]string{
		"status.json":          `{"partial":`,
		"crypto/zoo.json":      `{"ok":true} {"extra":true}`,
		"us/zoo.json":          "",
		"crypto/ensemble.json": `not JSON`,
		"us/ensemble.json":     `{"intact":true}`,
	}
	for name, data := range files {
		writeFactorMiningTestFile(t, dir, name, data)
	}
	body := factorMiningTestGet(t, factorMiningTestRouter(), "/api/factor-mining/overview")
	assertFactorMiningJSON(t, body["status"], `null`)
	assertFactorMiningJSON(t, body["zoo"], `{"crypto":null,"us":null}`)
	assertFactorMiningJSON(t, body["ensemble"], `{"crypto":null,"us":{"intact":true}}`)
	if count := strings.Count(logs.String(), "Invalid JSON in"); count != 4 {
		t.Fatalf("want one warning per corrupt file, got %d: %s", count, logs.String())
	}
}

func TestFactorMiningRunsLimitAndOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FACTOR_MINING_DIR", dir)
	var data strings.Builder
	for i := 0; i < 550; i++ {
		at := time.Date(2026, 10, 1, 0, i, 0, 0, time.UTC).Format(time.RFC3339)
		fmt.Fprintf(&data, "{\"run_id\":%d,\"started_at\":%q}\n", i, at)
	}
	writeFactorMiningTestFile(t, dir, "runs.jsonl", data.String())
	r := factorMiningTestRouter()
	for _, tc := range []struct {
		query string
		count int
	}{
		{"", 50}, {"?limit=1", 1}, {"?limit=3", 3}, {"?limit=500", 500},
		{"?limit=999", 500}, {"?limit=0", 50}, {"?limit=-2", 50},
		{"?limit=bad", 50}, {"?limit=1.5", 50}, {"?limit=", 50},
		{"?limit=999999999999999999999999999999", 50},
	} {
		t.Run(tc.query, func(t *testing.T) {
			body := factorMiningTestGet(t, r, "/api/factor-mining/runs"+tc.query)
			var runs []map[string]json.RawMessage
			if err := json.Unmarshal(body["runs"], &runs); err != nil {
				t.Fatal(err)
			}
			if len(runs) != tc.count {
				t.Fatalf("got %d runs, want %d", len(runs), tc.count)
			}
			for i, run := range runs {
				if string(run["run_id"]) != fmt.Sprint(549-i) {
					t.Fatalf("run %d: got %s, want %d", i, run["run_id"], 549-i)
				}
			}
		})
	}
}

func TestFactorMiningRunsBadLinesAndTail(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FACTOR_MINING_DIR", dir)
	r := factorMiningTestRouter()
	for _, ending := range []string{"", "\n", "\r\n"} {
		t.Run(fmt.Sprintf("ending=%q", ending), func(t *testing.T) {
			data := "{\"run_id\":0}\n{\"run_id\":1}\nbad\n{\"run_id\":2}\n{\"partial\":" + ending
			writeFactorMiningTestFile(t, dir, "runs.jsonl", data)
			body := factorMiningTestGet(t, r, "/api/factor-mining/runs?limit=3")
			assertFactorMiningJSON(t, body["runs"], `[{"run_id":2}]`)
			body = factorMiningTestGet(t, r, "/api/factor-mining/runs?limit=500")
			assertFactorMiningJSON(t, body["runs"], `[{"run_id":2},{"run_id":1},{"run_id":0}]`)
		})
	}
	// Valid records larger than bufio.Scanner's default token limit survive.
	largeRun := `{"future":"` + strings.Repeat("x", 70<<10) + `","run_id":9007199254740993}`
	writeFactorMiningTestFile(t, dir, "runs.jsonl", largeRun+"\n")
	body := factorMiningTestGet(t, r, "/api/factor-mining/runs")
	assertFactorMiningJSON(t, body["runs"], "["+largeRun+"]")
}

func TestFactorMiningRunsUnavailable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FACTOR_MINING_DIR", dir)
	r := factorMiningTestRouter()
	for _, data := range []string{"missing", "", "bad\n{", "\n\r\n"} {
		t.Run(data, func(t *testing.T) {
			if data != "missing" {
				writeFactorMiningTestFile(t, dir, "runs.jsonl", data)
			}
			body := factorMiningTestGet(t, r, "/api/factor-mining/runs")
			assertFactorMiningJSON(t, body["runs"], `[]`)
		})
	}
}

func TestFactorMiningFileSizeLimit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FACTOR_MINING_DIR", dir)
	r := factorMiningTestRouter()
	// The oversized payload is valid JSON, so only the size check can reject it.
	oversized := `"` + strings.Repeat("x", factorMiningMaxFileSize-1) + `"`
	writeFactorMiningTestFile(t, dir, "status.json", oversized)
	writeFactorMiningTestFile(t, dir, "crypto/zoo.json", oversized)
	writeFactorMiningTestFile(t, dir, "runs.jsonl", oversized)
	body := factorMiningTestGet(t, r, "/api/factor-mining/overview")
	assertFactorMiningJSON(t, body["status"], `null`)
	assertFactorMiningJSON(t, body["zoo"], `{"crypto":null,"us":null}`)
	body = factorMiningTestGet(t, r, "/api/factor-mining/runs")
	assertFactorMiningJSON(t, body["runs"], `[]`)
	// Exactly 8 MiB is allowed.
	atLimit := `"` + strings.Repeat("x", factorMiningMaxFileSize-2) + `"`
	writeFactorMiningTestFile(t, dir, "status.json", atLimit)
	body = factorMiningTestGet(t, r, "/api/factor-mining/overview")
	if len(body["status"]) != factorMiningMaxFileSize {
		t.Fatalf("8 MiB JSON was not preserved: got %d bytes", len(body["status"]))
	}
}

func TestFactorMiningDirectoryAndFixedPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("FACTOR_MINING_DIR", "")
	writeFactorMiningTestFile(t, "data/factor_mining", "status.json", `{"directory":"default"}`)
	writeFactorMiningTestFile(t, "data/factor_mining", "runs.jsonl", `{"run_id":"default"}`)
	r := factorMiningTestRouter()
	body := factorMiningTestGet(t, r, "/api/factor-mining/overview")
	assertFactorMiningJSON(t, body["status"], `{"directory":"default"}`)
	body = factorMiningTestGet(t, r, "/api/factor-mining/runs")
	assertFactorMiningJSON(t, body["runs"], `[{"run_id":"default"}]`)
	dir := t.TempDir()
	t.Setenv("FACTOR_MINING_DIR", dir)
	writeFactorMiningTestFile(t, dir, "status.json", `{"directory":"override"}`)
	writeFactorMiningTestFile(t, dir, "runs.jsonl", `{"run_id":"override"}`)
	query := "?path=../../status.json&dir=../../&market=../../&file=../../status.json"
	body = factorMiningTestGet(t, r, "/api/factor-mining/overview"+query)
	assertFactorMiningJSON(t, body["status"], `{"directory":"override"}`)
	body = factorMiningTestGet(t, r, "/api/factor-mining/runs"+query)
	assertFactorMiningJSON(t, body["runs"], `[{"run_id":"override"}]`)
}

func TestFactorMiningRoutesRequireLogin(t *testing.T) {
	t.Setenv("FACTOR_MINING_DIR", t.TempDir())
	srv, _, token, _, _ := idorFixture(t)
	// Use the actual server registration so moving either route out of the
	// protected group would fail this test.
	oldRegistry := append([]RouteDoc(nil), routeRegistry...)
	t.Cleanup(func() { routeRegistry = oldRegistry })
	srv.router = gin.New()
	srv.cryptoHandler = NewCryptoHandler(nil)
	srv.setupRoutes()
	for _, path := range []string{"/api/factor-mining/overview", "/api/factor-mining/runs"} {
		for _, header := range []string{"", "invalid", "Bearer invalid", "Bearer " + token} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			rec := httptest.NewRecorder()
			srv.router.ServeHTTP(rec, req)
			want := http.StatusUnauthorized
			if header == "Bearer "+token {
				want = http.StatusOK
			}
			if rec.Code != want {
				t.Fatalf("GET %s: got %d, want %d: %s", path, rec.Code, want, rec.Body.String())
			}
		}
	}
}
