package agent

import "testing"

func TestCleanUserFacingReplyInstruction(t *testing.T) {
	if cleanUserFacingReplyInstruction == "" {
		t.Fatal("expected clean user-facing reply instruction to be defined")
	}
	if got, want := cleanUserFacingReplyInstruction, "Your final reply must be clean and easy to understand, with no fluff, no internal jargon, and no unnecessary explanation."; got != want {
		t.Fatalf("unexpected instruction\nwant: %q\ngot:  %q", want, got)
	}
}
