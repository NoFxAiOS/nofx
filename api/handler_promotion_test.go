package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"nofx/logger"
)

func promotionTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := &Server{}
	r.GET("/api/promotion/report", s.handlePromotionReport)
	return r
}

func promotionTestGet(t *testing.T, r *gin.Engine, query string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/promotion/report"+query, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET report: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", rec.Header().Get("Content-Type"))
	}
	return rec
}

func TestPromotionReportVerbatim(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMOTION_DIR", dir)
	data := " {\n\"generated_at\":\"2026-10-20T00:00:00Z\",\"pairs\":[{\"id\":\"候选\",\"verdict\":\"ready\",\"future\":9007199254740993,\"precise\":0.1234567890123456789}]}\n"
	writeFactorMiningTestFile(t, dir, "report.json", data)
	if got := promotionTestGet(t, promotionTestRouter(), "").Body.String(); got != data {
		t.Fatalf("report was changed: got %s, want %s", got, data)
	}
}

func TestPromotionReportUnavailable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMOTION_DIR", dir)
	var logs bytes.Buffer
	oldOutput := logger.Log.Out
	logger.Log.SetOutput(&logs)
	t.Cleanup(func() { logger.Log.SetOutput(oldOutput) })
	r := promotionTestRouter()
	for _, data := range []string{"missing", "", `{"partial":`, `{} {}`, "not JSON"} {
		t.Run(data, func(t *testing.T) {
			if data != "missing" {
				writeFactorMiningTestFile(t, dir, "report.json", data)
			}
			assertFactorMiningJSON(t, promotionTestGet(t, r, "").Body.Bytes(), `{"pairs":[],"generated_at":null}`)
		})
	}
	if count := strings.Count(logs.String(), "Invalid JSON in"); count != 4 {
		t.Fatalf("want one warning per corrupt report, got %d: %s", count, logs.String())
	}
	// Opening a directory succeeds on Unix, but reading it must fall back too.
	if err := os.Remove(filepath.Join(dir, "report.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "report.json"), 0755); err != nil {
		t.Fatal(err)
	}
	assertFactorMiningJSON(t, promotionTestGet(t, r, "").Body.Bytes(), `{"pairs":[],"generated_at":null}`)
}

func TestPromotionReportSizeLimit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMOTION_DIR", dir)
	r := promotionTestRouter()
	oversized := `{"padding":"` + strings.Repeat("x", promotionMaxFileSize) + `"}`
	writeFactorMiningTestFile(t, dir, "report.json", oversized)
	assertFactorMiningJSON(t, promotionTestGet(t, r, "").Body.Bytes(), `{"pairs":[],"generated_at":null}`)
	atLimit := `{"padding":"` + strings.Repeat("x", promotionMaxFileSize-len(`{"padding":""}`)) + `"}`
	writeFactorMiningTestFile(t, dir, "report.json", atLimit)
	if got := promotionTestGet(t, r, "").Body.String(); got != atLimit {
		t.Fatalf("8 MiB report was not preserved: got %d bytes", len(got))
	}
}

func TestPromotionDirectoryAndFixedPath(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PROMOTION_DIR", "")
	writeFactorMiningTestFile(t, "data/promotion", "report.json", `{"directory":"default"}`)
	r := promotionTestRouter()
	assertFactorMiningJSON(t, promotionTestGet(t, r, "").Body.Bytes(), `{"directory":"default"}`)
	dir := t.TempDir()
	t.Setenv("PROMOTION_DIR", dir)
	writeFactorMiningTestFile(t, dir, "report.json", `{"directory":"override"}`)
	query := "?path=../../report.json&dir=../../&file=../../report.json"
	assertFactorMiningJSON(t, promotionTestGet(t, r, query).Body.Bytes(), `{"directory":"override"}`)
}

func TestPromotionRouteRequiresLoginAndHasSchema(t *testing.T) {
	t.Setenv("PROMOTION_DIR", t.TempDir())
	srv, _, token, _, _ := idorFixture(t)
	oldRegistry := append([]RouteDoc(nil), routeRegistry...)
	t.Cleanup(func() { routeRegistry = oldRegistry })
	srv.router = gin.New()
	srv.cryptoHandler = NewCryptoHandler(nil)
	srv.setupRoutes()
	for _, header := range []string{"", "invalid", "Bearer invalid", "Bearer " + token} {
		req := httptest.NewRequest(http.MethodGet, "/api/promotion/report", nil)
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
			t.Fatalf("GET report: got %d, want %d: %s", rec.Code, want, rec.Body.String())
		}
	}
	for _, route := range routeRegistry {
		if route.Method == http.MethodGet && route.Path == "/api/promotion/report" && route.Schema != "" {
			return
		}
	}
	t.Fatal("promotion report route is missing schema registration")
}
