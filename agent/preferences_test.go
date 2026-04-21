package agent

import (
	"strings"
	"testing"
)

func TestNewPersistentPreference(t *testing.T) {
	pref, err := NewPersistentPreference("  Always answer in Chinese.  ")
	if err != nil {
		t.Fatalf("expected preference to be created, got error: %v", err)
	}
	if pref.ID == "" {
		t.Fatal("expected non-empty preference id")
	}
	if pref.Text != "Always answer in Chinese." {
		t.Fatalf("expected trimmed text, got %q", pref.Text)
	}
	if pref.CreatedAt == "" {
		t.Fatal("expected created_at to be set")
	}
	if strings.Contains(pref.ID, "Always") {
		t.Fatalf("expected generated id, got %q", pref.ID)
	}
}

func TestNewPersistentPreferenceRejectsEmptyText(t *testing.T) {
	if _, err := NewPersistentPreference("   "); err == nil {
		t.Fatal("expected empty text to be rejected")
	}
}
