package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func winrateTestContext(query string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	return c
}

func TestParseWinrateQueryStrictValidation(t *testing.T) {
	base := "symbol=NVDA&marketType=hip3_perp"
	for _, query := range []string{
		"winMin=abc&winMax=100", "winMin=12garbage&winMax=100",
		"winMin=%ZZ&winMax=%ZZ",
		"winMin=-1&winMax=100", "winMin=&winMax=100", "winMax=100",
		"winMin=0", "winMin=0&winMax=0", "winMin=50&winMax=20",
		"winMin=0&winMin=20&winMax=100", "winMin=0&winMax=101",
		"costMin=NaN&costMax=108", "costMin=92.5&costMax=108",
		"costMin=92&costMax=1e2", "costMin=92", "costMin=108&costMax=92",
		"minRoundTrips=0", "minRoundTrips=-1", "minRoundTrips=10001",
		"minRoundTrips=99999999999999999999999999", "chain=testnet",
	} {
		t.Run(query, func(t *testing.T) {
			if _, err := parseWinrateQuery(winrateTestContext(base + "&" + query)); err == nil {
				t.Fatal("invalid input was silently accepted")
			}
		})
	}
	for _, query := range []string{"symbol=NVDA&marketType=typo", "symbol="} {
		if _, err := parseWinrateQuery(winrateTestContext(query)); err == nil {
			t.Fatalf("accepted %s", query)
		}
	}
	q, err := parseWinrateQuery(winrateTestContext(base))
	if err != nil || q.MinRoundTrips != 1 {
		t.Fatalf("defaults: %+v, %v", q, err)
	}
	q, err = parseWinrateQuery(winrateTestContext(base + "&winMin=0&winMax=100&costMin=92&costMax=108&minRoundTrips=3"))
	if err != nil || q.WinMax != 100 || q.CostMin != 92 || q.CostMax != 108 || q.MinRoundTrips != 3 {
		t.Fatalf("valid values changed: %+v, %v", q, err)
	}
}

func TestParseWinrateHoldersQueryStrictValidation(t *testing.T) {
	base, _ := url.ParseQuery("symbol=BTC&marketType=core_perp&snapshotId=XGSZMBVFLOZRFYMWGC44OBBQNX&row=0&rowEnd=19&column=0&columnEnd=17")
	q, err := parseWinrateHoldersQuery(winrateTestContext(base.Encode()))
	if err != nil || q.Offset != 0 || q.Limit != 50 {
		t.Fatalf("valid query: %+v, %v", q, err)
	}
	for _, tc := range [][2]string{
		{"row", ""}, {"row", "1junk"}, {"rowEnd", "20"}, {"columnEnd", "18"},
		{"offset", "-1"}, {"offset", "1000001"}, {"offset", "2.5"},
		{"limit", "0"}, {"limit", "101"}, {"snapshotId", "bad"}, {"side", "both"},
	} {
		t.Run(tc[0]+tc[1], func(t *testing.T) {
			values, _ := url.ParseQuery(base.Encode())
			values.Set(tc[0], tc[1])
			if _, err := parseWinrateHoldersQuery(winrateTestContext(values.Encode())); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	for _, field := range []string{"row", "rowEnd", "column", "columnEnd", "snapshotId"} {
		values, _ := url.ParseQuery(base.Encode())
		values.Del(field)
		if _, err := parseWinrateHoldersQuery(winrateTestContext(values.Encode())); err == nil {
			t.Fatalf("missing %s accepted", field)
		}
	}
}

func TestWinrateBadQueriesRejectedBeforeWalletResolution(t *testing.T) {
	// No store or wallet: touching wallet resolution here would fail/panic.
	s := &Server{}
	for _, handler := range []gin.HandlerFunc{s.handleVergexHolderWinrateMap, s.handleVergexHolderWinrateHolders} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", "test-user")
		c.Request = httptest.NewRequest(http.MethodGet, "/?symbol=NVDA&costMin=NaN&costMax=108", nil)
		handler(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d, want 400", w.Code)
		}
	}
}
