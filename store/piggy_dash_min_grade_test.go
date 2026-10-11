package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEffectivePiggyDashMinGrade(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "medium"},       // unset → default
		{"weak", "weak"},     // explicit lower floor
		{"WEAK", "weak"},     // case-insensitive
		{"medium", "medium"}, // the default, spelled out
		{"Strong", "strong"}, // case-insensitive
		{"bogus", "medium"},  // unknown → default
		{"noise", "medium"},  // not an admissible floor → default
	}
	for _, c := range cases {
		cs := CoinSourceConfig{PiggyDashMinGrade: c.in}
		if got := cs.EffectivePiggyDashMinGrade(); got != c.want {
			t.Errorf("PiggyDashMinGrade=%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

// The field must round-trip under the JSON name the UI writes. A config
// without it resolves to the medium default (the intended change from the
// old weak floor), and an unset value must not be serialized at all.
func TestPiggyDashMinGradeJSON(t *testing.T) {
	var cs CoinSourceConfig
	if err := json.Unmarshal([]byte(`{"use_piggy_dash":true,"piggy_dash_min_grade":"strong"}`), &cs); err != nil {
		t.Fatal(err)
	}
	if got := cs.EffectivePiggyDashMinGrade(); got != "strong" {
		t.Fatalf("got %q, want strong", got)
	}

	var legacy CoinSourceConfig
	if err := json.Unmarshal([]byte(`{"use_piggy_dash":true,"piggy_dash_limit":5}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if got := legacy.EffectivePiggyDashMinGrade(); got != DefaultPiggyDashMinGrade {
		t.Fatalf("legacy config: got %q, want default %q", got, DefaultPiggyDashMinGrade)
	}

	out, err := json.Marshal(CoinSourceConfig{UsePiggyDash: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "piggy_dash_min_grade") {
		t.Fatalf("unset piggy_dash_min_grade must be omitted, got %s", out)
	}
}
