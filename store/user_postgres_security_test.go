package store

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Opt-in integration test. Use a disposable local database, never an operator's
// application database. Each run owns and cleans up its unique schema only.
func TestSecurityPostgresMigrationAndRegistration(t *testing.T) {
	raw := os.Getenv("NOFX_SECURITY_TEST_POSTGRES_DSN")
	if raw == "" {
		t.Skip("disposable PostgreSQL DSN not provided")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() != "127.0.0.1" || u.Path != "/nofx_security_test" {
		t.Fatal("requires the isolated localhost nofx_security_test database")
	}
	open := func(dsn string) *gorm.DB {
		t.Helper()
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
		return db
	}
	admin := open(raw)
	schema := "security_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	first, second := NewUserStore(open(u.String())), NewUserStore(open(u.String()))
	if err := first.db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, created_at TIMESTAMP, updated_at TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := first.db.Exec(`INSERT INTO users (id,email,password_hash) VALUES ('legacy','legacy@example.test','old-hash')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := first.initTables(); err != nil {
		t.Fatal(err)
	}
	legacy, err := second.GetByID("legacy")
	if err != nil || legacy.AuthVersion != 0 || legacy.PasswordHash != "old-hash" {
		t.Fatal(legacy, err)
	}
	if err := first.UpdatePassword("legacy", "new-hash"); err != nil {
		t.Fatal(err)
	}
	legacy, err = second.GetByID("legacy")
	if err != nil || legacy.AuthVersion != 1 {
		t.Fatal(legacy, err)
	}
	if err := first.RevokeSession("legacy", "test-session", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if revoked, err := second.IsSessionRevoked("legacy", "test-session"); err != nil || !revoked {
		t.Fatal(revoked, err)
	}
	if err := first.DeleteAll(); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i, st := range []*UserStore{first, second} {
		go func(i int, st *UserStore) {
			<-start
			ids := []string{"one", "two"}
			results <- st.CreateFirst(&User{ID: ids[i], Email: ids[i] + "@example.test", PasswordHash: "test"})
		}(i, st)
	}
	close(start)
	successes, conflicts := 0, 0
	for range 2 {
		select {
		case err := <-results:
			if err == nil {
				successes++
			} else if errors.Is(err, ErrAlreadyInitialized) {
				conflicts++
			} else {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("registration timed out")
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", successes, conflicts)
	}
}
