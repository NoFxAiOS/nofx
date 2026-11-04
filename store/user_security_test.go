package store

import (
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func securityUserStore(t *testing.T, path string) *UserStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path+"?_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	s := NewUserStore(db)
	if err := s.initTables(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSecurityFirstRegistrationAcrossConnections(t *testing.T) {
	path := t.TempDir() + "/registration.db"
	first, second := securityUserStore(t, path), securityUserStore(t, path)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i, s := range []*UserStore{first, second} {
		wg.Add(1)
		go func(i int, s *UserStore) {
			defer wg.Done()
			<-start
			ids := []string{"one", "two"}
			results <- s.CreateFirst(&User{ID: ids[i], Email: ids[i] + "@example.test", PasswordHash: "test"})
		}(i, s)
	}
	close(start)
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrAlreadyInitialized) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", successes, conflicts)
	}
	if n, err := first.Count(); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err := first.DeleteAll(); err != nil {
		t.Fatal(err)
	}
	if err := second.CreateFirst(&User{ID: "reset", Email: "reset@example.test", PasswordHash: "test"}); err != nil {
		t.Fatal("explicit local reset must allow initialization", err)
	}
}

func TestSecurityUserMigrationAndSessionStorage(t *testing.T) {
	path := t.TempDir() + "/migration.db"
	s := securityUserStore(t, path)
	// Simulate the pre-upgrade schema; no live database is touched.
	if err := s.db.Migrator().DropColumn(&User{}, "auth_version"); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Exec("INSERT INTO users (id,email,password_hash) VALUES (?,?,?)", "legacy", "legacy@example.test", "old-hash").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.initTables(); err != nil {
		t.Fatal(err)
	}
	u, err := s.GetByID("legacy")
	if err != nil || u.AuthVersion != 0 || u.PasswordHash != "old-hash" {
		t.Fatal(u, err)
	}
	if err := s.UpdatePassword(u.ID, "new-hash"); err != nil {
		t.Fatal(err)
	}
	u, _ = s.GetByID(u.ID)
	if u.AuthVersion != 1 || u.PasswordHash != "new-hash" {
		t.Fatal(u)
	}
	if err := s.UpdatePassword("missing", "hash"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal(err)
	}
	if err := s.RevokeSession("legacy", "test-session", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeSession("legacy", "expired-session", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	other := securityUserStore(t, path)
	if revoked, err := other.IsSessionRevoked("legacy", "test-session"); err != nil || !revoked {
		t.Fatal(revoked, err)
	}
	if revoked, err := other.IsSessionRevoked("legacy", "expired-session"); err != nil || revoked {
		t.Fatal(revoked, err)
	}
	var rows []revokedSession
	if err := s.db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if len(row.SessionHash) != 64 || row.SessionHash == "test-session" {
			t.Fatal("raw token persisted")
		}
	}
	if revoked, err := other.IsSessionRevoked("another-user", "test-session"); err != nil || revoked {
		t.Fatal("revocation crossed user boundary", revoked, err)
	}
}
