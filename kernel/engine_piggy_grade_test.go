package kernel

import (
	"reflect"
	"testing"

	"nofx/market/breakout"
	"nofx/store"
)

// piggyGradeRank is the ordering behind the piggy_dash quality floor:
// noise 0 < weak 1 < medium 2 < strong 3, and anything unknown ranks 0.
func TestPiggyGradeRank(t *testing.T) {
	cases := []struct {
		grade string
		want  int
	}{
		{"noise", 0},
		{"weak", 1},
		{"medium", 2},
		{"strong", 3},
		{"", 0},      // unknown → 0
		{"bogus", 0}, // unknown → 0
	}
	for _, c := range cases {
		if got := piggyGradeRank(c.grade); got != c.want {
			t.Errorf("piggyGradeRank(%q) = %d, want %d", c.grade, got, c.want)
		}
	}
	ordered := []string{"noise", "weak", "medium", "strong"}
	for i := 1; i < len(ordered); i++ {
		if piggyGradeRank(ordered[i]) <= piggyGradeRank(ordered[i-1]) {
			t.Fatalf("%s must rank above %s", ordered[i], ordered[i-1])
		}
	}
}

// The configured floor admits a row only when its grade ranks at or above the
// minimum, and only on a real cross pattern. The default (piggy_dash_min_grade
// unset) resolves to medium on the live source path.
func TestPiggyMinGradeFloor(t *testing.T) {
	breakout.DefaultScheduler().SetSnapshotForTest([]breakout.ScanResult{
		{Symbol: "STRONGUSDT", Direction: breakout.DirUp, Score: 85, Grade: "strong", Pattern: breakout.PatternBreakout},
		{Symbol: "MEDUSDT", Direction: breakout.DirUp, Score: 65, Grade: "medium", Pattern: breakout.PatternBreakout},
		{Symbol: "WEAKUSDT", Direction: breakout.DirUp, Score: 45, Grade: "weak", Pattern: breakout.PatternBreakout},
		{Symbol: "NOISEUSDT", Direction: breakout.DirUp, Score: 30, Grade: "noise", Pattern: breakout.PatternBreakout},
	})
	e := NewStrategyEngine(&store.StrategyConfig{CoinSource: store.CoinSourceConfig{
		SourceType:     "piggy_dash",
		PiggyDashLimit: 10,
	}})

	cases := []struct {
		minGrade string
		want     []string
	}{
		{"weak", []string{"STRONGUSDT", "MEDUSDT", "WEAKUSDT"}},
		{"medium", []string{"STRONGUSDT", "MEDUSDT"}},
		{"strong", []string{"STRONGUSDT"}},
	}
	for _, c := range cases {
		coins, err := e.getPiggyDashCoins(10, "", c.minGrade)
		if err != nil {
			t.Fatalf("minGrade %s: %v", c.minGrade, err)
		}
		if got := piggyTestSymbols(coins); !reflect.DeepEqual(got, c.want) {
			t.Errorf("minGrade %s: pool = %v, want %v", c.minGrade, got, c.want)
		}
	}

	// Unset config → EffectivePiggyDashMinGrade (medium) drives the real
	// source path, so the weak row is excluded without any explicit setting.
	coins, err := e.GetCandidateCoins()
	if err != nil {
		t.Fatalf("GetCandidateCoins: %v", err)
	}
	if got, want := piggyTestSymbols(coins), []string{"STRONGUSDT", "MEDUSDT"}; !reflect.DeepEqual(got, want) {
		t.Errorf("default floor: pool = %v, want %v", got, want)
	}

	// A board with nothing at the floor is an error (the source halts for the
	// cycle), never a quiet backfill of weaker rows.
	breakout.DefaultScheduler().SetSnapshotForTest([]breakout.ScanResult{
		{Symbol: "WEAKUSDT", Direction: breakout.DirUp, Score: 45, Grade: "weak", Pattern: breakout.PatternBreakout},
	})
	if _, err := e.getPiggyDashCoins(10, "", "medium"); err == nil {
		t.Fatal("a board with nothing at the medium floor must error, not backfill weak rows")
	}
}

func piggyTestSymbols(coins []CandidateCoin) []string {
	var out []string
	for _, c := range coins {
		out = append(out, c.Symbol)
	}
	return out
}
