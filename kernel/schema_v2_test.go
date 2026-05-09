package kernel

import (
	"encoding/json"
	"testing"
)

func TestDecisionSchemaV2FieldsParseCompatibly(t *testing.T) {
	raw := `[
		{
			"symbol":"BTCUSDT",
			"action":"wait",
			"regime":"range",
			"setup_type":"none",
			"confidence":64,
			"quality_score":{
				"total":64,
				"trend_alignment":12,
				"structure_location":10,
				"sr_fib_quality":8,
				"derivatives_context":12,
				"trigger_quality":8,
				"net_rr":14
			},
			"reasoning":"range middle, wait"
		}
	]`

	var decisions []Decision
	if err := json.Unmarshal([]byte(raw), &decisions); err != nil {
		t.Fatalf("expected v2 fields to parse, got %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(decisions))
	}
	d := decisions[0]
	if d.Regime != "range" || d.SetupType != "none" || d.QualityScore == nil || d.QualityScore.Total != 64 || d.QualityScore.DerivativesContext != 12 {
		t.Fatalf("unexpected parsed v2 fields: %+v", d)
	}
	if err := ValidateDecisionFormat(decisions); err != nil {
		t.Fatalf("expected wait decision with v2 fields to remain valid, got %v", err)
	}
}
