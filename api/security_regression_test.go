package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nofx/auth"
	"nofx/manager"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

func TestSecurityPublicCompetitionRechecksCachedVisibility(t *testing.T) {
	st := securityStore(t)
	for _, id := range []string{"private", "public"} {
		if err := st.Trader().Create(&store.Trader{ID: id, UserID: "owner", Name: id, AIModelID: "fixture", ExchangeID: "fixture"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Trader().UpdateShowInCompetition("owner", "private", false); err != nil {
		t.Fatal(err)
	}
	s := NewServer(nil, st, nil, 0)
	cached := map[string]interface{}{
		"traders": []map[string]interface{}{{"trader_id": "private"}, {"trader_id": "public"}, {"trader_id": "deleted"}},
		"count":   3, "total_count": 3,
	}
	data, err := s.filterPublicCompetition(cached)
	if err != nil {
		t.Fatal(err)
	}
	rows := data["traders"].([]map[string]interface{})
	if len(rows) != 1 || rows[0]["trader_id"] != "public" || data["count"] != 1 || data["total_count"] != 1 {
		t.Fatal(data)
	}
	if err := st.Trader().UpdateShowInCompetition("owner", "public", false); err != nil {
		t.Fatal(err)
	}
	data, err = s.filterPublicCompetition(cached)
	if err != nil || data["count"] != 0 || data["total_count"] != 0 {
		t.Fatal("cached data survived opt-out", data, err)
	}
	if cached["count"] != 3 || len(cached["traders"].([]map[string]interface{})) != 3 {
		t.Fatal("shared cache was mutated")
	}
	// Private and absent traders must be indistinguishable; neither should
	// reach the nil runtime manager or query account/exchange data.
	private := securityRequest(s.router, "GET", "/api/traders/private/public-config", "", "")
	absent := securityRequest(s.router, "GET", "/api/traders/missing/public-config", "", "")
	if private.Code != 404 || absent.Code != 404 || private.Body.String() != absent.Body.String() {
		t.Fatal(private.Code, private.Body.String(), absent.Code, absent.Body.String())
	}
	sqlDB, _ := st.GormDB().DB()
	_ = sqlDB.Close()
	if _, err := s.filterPublicCompetition(cached); err == nil {
		t.Fatal("visibility lookup failure must fail closed")
	}
	if w := securityRequest(s.router, "GET", "/api/traders/private/public-config", "", ""); w.Code != 500 {
		t.Fatal("database failure exposed config", w.Code)
	}
}

func TestSecurityTraderMutationsRequireOwnership(t *testing.T) {
	securityJWTSecret(t)
	st := securityStore(t)
	for _, id := range []string{"owner", "other"} {
		if err := st.User().Create(&store.User{ID: id, Email: id + "@example.test", PasswordHash: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	tr := &store.Trader{ID: "owned", UserID: "owner", Name: "owned", AIModelID: "fixture", ExchangeID: "fixture", CustomPrompt: "original"}
	if err := st.Trader().Create(tr); err != nil {
		t.Fatal(err)
	}
	if err := st.Equity().Save(&store.EquitySnapshot{TraderID: tr.ID, TotalEquity: 1234, Timestamp: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateJWT("other", "other@example.test", 0)
	if err != nil {
		t.Fatal(err)
	}
	// A nil manager verifies that rejected requests cannot touch runtime bots.
	s := NewServer(nil, st, nil, 0)
	for _, tc := range []struct{ method, suffix, body string }{
		{"PUT", "/prompt", `{"custom_prompt":"changed","override_base_prompt":true}`},
		{"PUT", "/competition", `{"show_in_competition":false}`},
		{"DELETE", "", ""},
	} {
		for _, id := range []string{"owned", "missing"} {
			w := securityRequest(s.router, tc.method, "/api/traders/"+id+tc.suffix, tc.body, token)
			if w.Code != 404 {
				t.Fatalf("%s %s: %d %s", tc.method, id+tc.suffix, w.Code, w.Body.String())
			}
		}
	}
	unchanged, err := st.Trader().GetByID("owned")
	if err != nil || unchanged.CustomPrompt != "original" || !unchanged.ShowInCompetition {
		t.Fatal("unauthorized mutation", unchanged, err)
	}
	var count int64
	if err := st.GormDB().Model(&store.EquitySnapshot{}).Where("trader_id = ?", "owned").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("unauthorized history deletion", count, err)
	}
	// Legitimate owner operations still work, including idempotent updates.
	for range 2 {
		if err := st.Trader().UpdateCustomPrompt("owner", "owned", "new", true); err != nil {
			t.Fatal(err)
		}
		if err := st.Trader().UpdateShowInCompetition("owner", "owned", false); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Trader().Delete("owner", "owned"); err != nil {
		t.Fatal(err)
	}
	if err := st.GormDB().Model(&store.EquitySnapshot{}).Where("trader_id = ?", "owned").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("owner history cleanup failed", count, err)
	}
}

func securityStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir() + "/security.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func securityRequest(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "198.51.100.10:45678"
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func securityJWTSecret(t *testing.T) {
	t.Helper()
	previous := auth.JWTSecret
	auth.SetJWTSecret("isolated-security-regression-not-a-live-secret")
	t.Cleanup(func() { auth.JWTSecret = previous })
}

func TestSecurityLoginThrottleRejectsSpoofedHeaders(t *testing.T) {
	t.Setenv("NOFX_TRUSTED_PROXIES", "")
	s := NewServer(nil, nil, nil, 0)
	for i := 0; i < 12; i++ {
		r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{}`))
		r.RemoteAddr = "198.51.100.10:45678"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		r.Header.Set("X-Real-IP", fmt.Sprintf("203.0.113.%d", i+1))
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, r)
		want := http.StatusBadRequest
		if i >= 8 {
			want = http.StatusTooManyRequests
		}
		if w.Code != want {
			t.Fatalf("attempt %d: got %d want %d", i+1, w.Code, want)
		}
	}
}

func TestRemovedTelegramRoutes(t *testing.T) {
	s := NewServer(nil, nil, nil, 0)
	for _, route := range s.router.Routes() {
		if strings.Contains(route.Path, "telegram") {
			t.Fatalf("removed endpoint still registered: %s", route.Path)
		}
	}
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/telegram"}, {"POST", "/api/telegram"},
		{"POST", "/api/telegram/model"}, {"DELETE", "/api/telegram/binding"},
	} {
		if w := securityRequest(s.router, tc.method, tc.path, `{}`, ""); w.Code != 404 {
			t.Fatalf("%s %s: %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestSecurityTrustedProxyBoundary(t *testing.T) {
	for _, tc := range []struct{ trusted, peer, forwarded, want string }{
		{"", "198.51.100.10:1234", "203.0.113.9", "198.51.100.10"},
		{"192.0.2.1", "198.51.100.10:1234", "203.0.113.9", "198.51.100.10"},
		{"192.0.2.1", "192.0.2.1:1234", "203.0.113.9, 198.51.100.10", "198.51.100.10"},
		{"192.0.2.0/24", "192.0.2.1:1234", "198.51.100.10, 192.0.2.2", "198.51.100.10"},
	} {
		router := gin.New()
		if err := configureTrustedProxies(router, tc.trusted); err != nil {
			t.Fatal(err)
		}
		router.GET("/", func(c *gin.Context) { c.String(200, c.ClientIP()) })
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.forwarded)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Body.String() != tc.want {
			t.Fatalf("%+v: got %s", tc, w.Body.String())
		}
	}
	for _, bad := range []string{"*", "0.0.0.0/0", "::/0", "0.0.0.0", "::", "proxy.example"} {
		if configureTrustedProxies(gin.New(), bad) == nil {
			t.Errorf("accepted unsafe proxy %s", bad)
		}
	}
}

func TestSecurityAPIListenAddress(t *testing.T) {
	for _, tc := range []struct{ setting, want string }{
		{"", "127.0.0.1:8080"}, {"127.0.0.1", "127.0.0.1:8080"},
		{"0.0.0.0", "0.0.0.0:8080"}, {"::1", "[::1]:8080"},
	} {
		t.Setenv("NOFX_API_BIND_ADDRESS", tc.setting)
		got, err := apiListenAddress(8080)
		if err != nil || got != tc.want {
			t.Fatalf("%s: %s %v", tc.setting, got, err)
		}
	}
	t.Setenv("NOFX_API_BIND_ADDRESS", "untrusted.example")
	if _, err := apiListenAddress(8080); err == nil {
		t.Fatal("non-IP bind address accepted")
	}
}

type securityGatedBody struct {
	io.Reader
	ready   chan<- struct{}
	release <-chan struct{}
	once    sync.Once
}

func (b *securityGatedBody) Read(p []byte) (int, error) {
	b.once.Do(func() { b.ready <- struct{}{}; <-b.release })
	return b.Reader.Read(p)
}
func (b *securityGatedBody) Close() error { return nil }

func TestSecurityConcurrentFirstRegistration(t *testing.T) {
	securityJWTSecret(t)
	st := securityStore(t)
	s := NewServer(manager.NewTraderManager(), st, nil, 0)
	ready, release := make(chan struct{}, 2), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	results := make(chan int, 2)
	for _, email := range []string{"owner@example.test", "racer@example.test"} {
		body := `{"email":"` + email + `","password":"test-only-password"}`
		r := httptest.NewRequest("POST", "/api/register", strings.NewReader(body))
		r.RemoteAddr = "198.51.100.10:45678"
		r.Header.Set("Content-Type", "application/json")
		r.Body = &securityGatedBody{Reader: strings.NewReader(body), ready: ready, release: release}
		go func() { w := httptest.NewRecorder(); s.router.ServeHTTP(w, r); results <- w.Code }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			t.Fatal("registration did not reach gate")
		}
	}
	unblock()
	counts := map[int]int{}
	for i := 0; i < 2; i++ {
		select {
		case code := <-results:
			counts[code]++
		case <-time.After(10 * time.Second):
			t.Fatal("registration timed out")
		}
	}
	if counts[200] != 1 || counts[403] != 1 {
		t.Fatalf("statuses: %v", counts)
	}
	if count, err := st.User().Count(); err != nil || count != 1 {
		t.Fatalf("users=%d error=%v", count, err)
	}
}

func securityLogin(t *testing.T, s *Server, password string) string {
	t.Helper()
	w := securityRequest(s.router, "POST", "/api/login", `{"email":"owner@example.test","password":"`+password+`"}`, "")
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Token == "" {
		t.Fatal("missing login token", err)
	}
	return response.Token
}

func securityProtectedServer(st *store.Store) *Server {
	s := NewServer(manager.NewTraderManager(), st, nil, 0)
	s.router.GET("/security-protected", s.authMiddleware(), func(c *gin.Context) { c.Status(204) })
	return s
}

func TestSecuritySessionLifecycle(t *testing.T) {
	securityJWTSecret(t)
	path := t.TempDir() + "/sessions.db"
	st, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	hash, err := auth.HashPassword("old-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.User().Create(&store.User{ID: "owner", Email: "owner@example.test", PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}
	s := securityProtectedServer(st)
	first, second := securityLogin(t, s, "old-password"), securityLogin(t, s, "old-password")
	if first == second {
		t.Fatal("independent logins must not share a token")
	}
	check := func(token string, want int) {
		t.Helper()
		if w := securityRequest(s.router, "GET", "/security-protected", "", token); w.Code != want {
			t.Fatalf("session status %d want %d: %s", w.Code, want, w.Body.String())
		}
	}
	check(first, 204)
	if w := securityRequest(s.router, "POST", "/api/logout", "", first); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	check(first, 401)
	check(second, 204)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	s = securityProtectedServer(st)
	check(first, 401) // durable across process/store recreation
	check(second, 204)
	if w := securityRequest(s.router, "PUT", "/api/user/password", `{"new_password":"new-password"}`, second); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	check(second, 401)
	third := securityLogin(t, s, "new-password")
	check(third, 204)
	if err := st.User().UpdatePassword("owner", hash); err != nil {
		t.Fatal(err)
	} // CLI reset uses this same method
	check(third, 401)
	fourth := securityLogin(t, s, "old-password")
	check(fourth, 204)
	if err := st.User().DeleteAll(); err != nil {
		t.Fatal(err)
	}
	check(fourth, 401)
}

func TestSecuritySessionDBFailureClosed(t *testing.T) {
	securityJWTSecret(t)
	st := securityStore(t)
	if err := st.User().Create(&store.User{ID: "owner", Email: "owner@example.test"}); err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateJWT("owner", "owner@example.test", 0)
	if err != nil {
		t.Fatal(err)
	}
	s := securityProtectedServer(st)
	if err := st.GormDB().Migrator().DropTable("revoked_sessions"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/security-protected", "/api/logout"} {
		method := "GET"
		if path == "/api/logout" {
			method = "POST"
		}
		if w := securityRequest(s.router, method, path, "", token); w.Code != 503 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}

func TestSecurityLogoutRejectsEquivalentTokenEncodings(t *testing.T) {
	securityJWTSecret(t)
	st := securityStore(t)
	if err := st.User().Create(&store.User{ID: "owner", Email: "owner@example.test"}); err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateJWT("owner", "owner@example.test", 0)
	if err != nil {
		t.Fatal(err)
	}
	// A SHA-256 signature's final base64url character has two unused bits.
	// These variants decode to identical signed bytes, not forged signatures.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	index := strings.IndexByte(alphabet, token[len(token)-1])
	alias := token[:len(token)-1] + string(alphabet[index|1])
	s := securityProtectedServer(st)
	if w := securityRequest(s.router, "GET", "/security-protected", "", alias); w.Code != 204 {
		t.Fatalf("fixture must be an equivalent valid JWT: %d", w.Code)
	}
	if w := securityRequest(s.router, "POST", "/api/logout", "", token); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for variant := 0; variant < 4; variant++ {
		raw := token[:len(token)-1] + string(alphabet[index|variant])
		if w := securityRequest(s.router, "GET", "/security-protected", "", raw); w.Code != 401 {
			t.Fatalf("logged-out session revived via signature encoding %d: status %d", variant, w.Code)
		}
	}
}

func TestSecurityBatchEquityRespectsOptOut(t *testing.T) {
	st := securityStore(t)
	for _, id := range []string{"private", "public"} {
		tr := &store.Trader{ID: id, UserID: "owner", Name: id, AIModelID: "fixture", ExchangeID: "fixture", InitialBalance: 1000}
		if err := st.Trader().Create(tr); err != nil {
			t.Fatal(err)
		}
		if err := st.GormDB().Model(tr).Update("show_in_competition", id == "public").Error; err != nil {
			t.Fatal(err)
		}
		if err := st.Equity().Save(&store.EquitySnapshot{TraderID: id, TotalEquity: 1234.56, Timestamp: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	s := NewServer(manager.NewTraderManager(), st, nil, 0)
	for _, body := range []string{`{"trader_ids":["private","public","missing"]}`, `{"trader_ids":["private","public","missing"],"hours":24}`} {
		w := securityRequest(s.router, "POST", "/api/equity-history-batch", body, "")
		var data struct {
			Histories map[string]json.RawMessage `json:"histories"`
			Errors    map[string]string          `json:"errors"`
			Count     int                        `json:"count"`
		}
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if data.Count != 1 || data.Histories["private"] != nil || data.Histories["missing"] != nil || !strings.Contains(string(data.Histories["public"]), "1234.56") {
			t.Fatal(w.Body.String())
		}
		if data.Errors["private"] != data.Errors["missing"] {
			t.Fatal("private trader existence leaked")
		}
	}
	// Opt-out takes effect even after a successful public request.
	if err := st.GormDB().Model(&store.Trader{}).Where("id = ?", "public").Update("show_in_competition", false).Error; err != nil {
		t.Fatal(err)
	}
	w := securityRequest(s.router, "POST", "/api/equity-history-batch", `{"trader_ids":["public"]}`, "")
	if strings.Contains(w.Body.String(), "1234.56") {
		t.Fatal("opt-out retained a public balance")
	}
}
