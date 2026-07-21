package telegram

import (
	"strings"
	"testing"
)

func TestParseJapaneseLanguageChoice(t *testing.T) {
	for _, choice := range []string{"3", "ja", "JA", "jp", "Japanese", "日本語"} {
		if got := parseLangChoice(choice); got != "ja" {
			t.Fatalf("parseLangChoice(%q) = %q, want ja", choice, got)
		}
	}
}

func TestJapaneseHelp(t *testing.T) {
	help := helpMsg("ja")
	if !strings.Contains(help, "NOFX ヘルプ") || !strings.Contains(help, "ポジションを表示して") {
		t.Fatalf("Japanese help is incomplete: %s", help)
	}
}
