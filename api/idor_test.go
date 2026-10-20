package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"nofx/auth"
	"nofx/manager"
	"nofx/store"
)

// IDOR regression (P1, 2026-09-25): getTraderFromQuery must reject a
// trader_id the authenticated user does not own, and ai-costs must be
// scoped to the caller's own traders. Exercises the REAL auth + store
// stack against a two-user fixture.
func idorFixture(t *testing.T) (*Server, *gin.Engine, string, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st, err := store.NewWithConfig(store.DBConfig{
		Type: store.DBTypeSQLite,
		Path: filepath.Join(t.TempDir(), "idor.db"),
	})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	auth.JWTSecret = []byte("test-jwt-secret-at-least-32-bytes-long")
	// Two users, one trader each.
	userA, userB := "user-aaa", "user-bbb"
	for _, id := range []string{userA, userB} {
		if err := st.User().Create(&store.User{ID: id, Email: id + "@test.com"}); err != nil {
			t.Fatal(err)
		}
	}
	traderA := &store.Trader{ID: "trader-A", UserID: userA, Name: "A", IsRunning: false}
	if err := st.Trader().Create(traderA); err != nil {
		t.Fatalf("create trader A: %v", err)
	}
	if err := st.Trader().Create(&store.Trader{ID: "trader-B", UserID: userB, Name: "B"}); err != nil {
		t.Fatalf("create trader B: %v", err)
	}
	// AI spend booked to trader A.
	_ = st.AICharge().Record("trader-A", "glm-5.3-flash", "z.ai")

	srv := &Server{store: st, traderManager: manager.NewTraderManager()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("/", srv.authMiddleware())
	protected.GET("/probe", func(c *gin.Context) {
		_, traderID, err := srv.getTraderFromQuery(c)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"trader_id": traderID})
	})
	protected.GET("/ai-costs", srv.handleGetAICosts)

	tokenA, err := auth.GenerateJWT(userA, "a@test")
	if err != nil {
		t.Fatalf("jwt: %v", err)
	}
	return srv, r, tokenA, userA, traderA.ID
}

func TestIDORForeignTraderRejected(t *testing.T) {
	_, r, tokenA, _, _ := idorFixture(t)

	// User A explicitly requests user B's trader → forbidden.
	req := httptest.NewRequest(http.MethodGet, "/probe?trader_id=trader-B", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-user trader_id must be rejected, got %d: %s", rec.Code, rec.Body.String())
	}

	// User A requests their own trader → passes.
	req2 := httptest.NewRequest(http.MethodGet, "/probe?trader_id=trader-A", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenA)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("own trader_id must pass, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// No trader_id → defaults to the user's OWN first trader, never a
	// global-map head.
	req3 := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req3.Header.Set("Authorization", "Bearer "+tokenA)
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK || !containsStr(rec3.Body.String(), "trader-A") {
		t.Fatalf("default must resolve to the user's own trader, got %d: %s", rec3.Code, rec3.Body.String())
	}
}

// The ai-costs endpoint must not leak other users' spend.
func TestIDORAICostsScoped(t *testing.T) {
	_, r, tokenA, _, _ := idorFixture(t)

	req := httptest.NewRequest(http.MethodGet, "/ai-costs?trader_id=trader-B", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign ai-costs must be forbidden, got %d: %s", rec.Code, rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/ai-costs?trader_id=trader-A", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenA)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("own ai-costs must pass, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 ||
		indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// P1 #2 (2026-09-25): trader deletion must verify ownership BEFORE purging
// the associated equity snapshots, and purge+delete must be atomic. A
// non-owner delete call must leave the victim's equity history intact.
func TestIDORDelateScopedEquity(t *testing.T) {
	srv, _, _, userA, _ := idorFixture(t)
	st := srv.store

	// Seed equity snapshots for BOTH traders.
	if err := st.Equity().Save(&store.EquitySnapshot{TraderID: "trader-A", TotalEquity: 100, Timestamp: time.Now()}); err != nil {
		t.Fatalf("seed equity A: %v", err)
	}
	if err := st.Equity().Save(&store.EquitySnapshot{TraderID: "trader-B", TotalEquity: 200, Timestamp: time.Now()}); err != nil {
		t.Fatalf("seed equity B: %v", err)
	}

	// User A tries to delete user B's trader.
	err := st.Trader().Delete(userA, "trader-B")
	if err == nil {
		t.Fatal("deleting a foreign trader must be rejected")
	}

	// B's equity history must be intact; B's trader row intact.
	var countB int64
	st.GormDB().Model(&store.EquitySnapshot{}).Where("trader_id = ?", "trader-B").Count(&countB)
	if countB != 1 {
		t.Fatalf("foreign delete must NOT purge equity snapshots, got %d rows", countB)
	}
	if _, err := st.Trader().GetByID("trader-B"); err != nil {
		t.Fatal("foreign delete must not delete the trader row")
	}

	// Owner deletes their own trader: equity purge + row delete, atomic.
	if err := st.Trader().Delete(userA, "trader-A"); err != nil {
		t.Fatalf("own delete must succeed: %v", err)
	}
	var countA int64
	st.GormDB().Model(&store.EquitySnapshot{}).Where("trader_id = ?", "trader-A").Count(&countA)
	if countA != 0 {
		t.Fatalf("own delete must purge own equity snapshots, got %d rows", countA)
	}
	if _, err := st.Trader().GetByID("trader-A"); err == nil {
		t.Fatal("own delete must remove the trader row")
	}
}

// Public equity-history regression (2026-09-26): the IDOR fix made
// getTraderFromQuery require a user, silently 400ing every unauthenticated
// dashboard call. The public resolver must serve competition-visible
// traders and reject hidden/missing ones identically.
func TestPublicEquityHistoryVisibility(t *testing.T) {
	srv, _, _, userA, _ := idorFixture(t)
	st := srv.store

	// trader-A: visible (default true). A hidden twin for the contrast.
	// (Create+false field alone doesn't stick: GORM's default:true tag omits
	// zero-value bools from the INSERT, so the DB default wins — flip it via
	// the explicit update path instead.)
	if err := st.Trader().Create(&store.Trader{ID: "trader-hidden", UserID: userA, Name: "H"}); err != nil {
		t.Fatalf("seed hidden trader: %v", err)
	}
	if err := st.Trader().UpdateShowInCompetition(userA, "trader-hidden", false); err != nil {
		t.Fatalf("hide trader: %v", err)
	}

	r := gin.New()
	r.GET("/pub-equity", func(c *gin.Context) {
		id, err := srv.getTraderIDPublic(c)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"trader_id": id})
	})

	// Visible trader → 200.
	req := httptest.NewRequest(http.MethodGet, "/pub-equity?trader_id=trader-A", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("visible trader must resolve, got %d: %s", rec.Code, rec.Body.String())
	}

	// Hidden trader → same error as missing (no existence oracle).
	req2 := httptest.NewRequest(http.MethodGet, "/pub-equity?trader_id=trader-hidden", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("hidden trader must NOT resolve, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// Hidden trader + its own owner's token → resolves; another user's token
	// or a garbage token → still hidden.
	tokenOwner, _ := auth.GenerateJWT(userA, "a@test")
	tokenOther, _ := auth.GenerateJWT("user-bbb", "b@test")
	for _, tc := range []struct {
		name, token string
		want        int
	}{{"owner", tokenOwner, http.StatusOK}, {"other user", tokenOther, http.StatusNotFound}, {"garbage", "junk", http.StatusNotFound}} {
		rq := httptest.NewRequest(http.MethodGet, "/pub-equity?trader_id=trader-hidden", nil)
		rq.Header.Set("Authorization", "Bearer "+tc.token)
		rc := httptest.NewRecorder()
		r.ServeHTTP(rc, rq)
		if rc.Code != tc.want {
			t.Fatalf("hidden trader with %s token: got %d want %d", tc.name, rc.Code, tc.want)
		}
	}

	// Missing trader → same error.
	req3 := httptest.NewRequest(http.MethodGet, "/pub-equity?trader_id=trader-ghost", nil)
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("missing trader must 404, got %d", rec3.Code)
	}

	// Empty trader_id → error.
	req4 := httptest.NewRequest(http.MethodGet, "/pub-equity", nil)
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusNotFound {
		t.Fatalf("missing trader_id must 404, got %d", rec4.Code)
	}
}
