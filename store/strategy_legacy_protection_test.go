package store

import (
	"encoding/json"
	"testing"
)

func TestProtectionValueSourceUnmarshalJSONSupportsLegacyEnabledShape(t *testing.T) {
	var src ProtectionValueSource
	if err := json.Unmarshal([]byte(`{"enabled":false}`), &src); err != nil {
		t.Fatalf("unmarshal legacy disabled source: %v", err)
	}
	if src.Mode != ProtectionValueModeDisabled {
		t.Fatalf("expected disabled mode, got %q", src.Mode)
	}

	if err := json.Unmarshal([]byte(`{"enabled":true,"value":1.5}`), &src); err != nil {
		t.Fatalf("unmarshal legacy enabled source: %v", err)
	}
	if src.Mode != ProtectionValueModeManual {
		t.Fatalf("expected manual mode, got %q", src.Mode)
	}
	if src.Value != 1.5 {
		t.Fatalf("expected value 1.5, got %v", src.Value)
	}
}

func TestProtectionValueSourceUnmarshalJSONSupportsModernModeShape(t *testing.T) {
	var src ProtectionValueSource
	if err := json.Unmarshal([]byte(`{"mode":"ai","value":0}`), &src); err != nil {
		t.Fatalf("unmarshal modern source: %v", err)
	}
	if src.Mode != ProtectionValueModeAI {
		t.Fatalf("expected ai mode, got %q", src.Mode)
	}
}
