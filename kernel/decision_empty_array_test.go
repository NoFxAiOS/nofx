package kernel

import "testing"

func TestValidateJSONFormatAllowsEmptyDecisionArray(t *testing.T) {
	if err := validateJSONFormat("[]"); err != nil {
		t.Fatalf("expected [] to be allowed as no-trade decision array, got %v", err)
	}
}

func TestValidateDecisionFormatAllowsEmptyDecisionList(t *testing.T) {
	if err := ValidateDecisionFormat([]Decision{}); err != nil {
		t.Fatalf("expected empty decision list to be valid no-trade output, got %v", err)
	}
}
