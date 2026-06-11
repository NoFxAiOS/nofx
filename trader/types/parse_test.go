package types

import (
	"strings"
	"testing"
)

func TestParseFloatField(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "normal value", field: "totalEq", input: "1234.56", want: 1234.56},
		{name: "zero", field: "totalEq", input: "0", want: 0},
		{name: "negative", field: "upl", input: "-12.5", want: -12.5},
		{name: "empty string treated as zero", field: "upl", input: "", want: 0},
		{name: "garbage returns error", field: "totalEq", input: "abc", wantErr: true},
		{name: "null literal returns error", field: "totalEq", input: "null", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFloatField(tt.field, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseFloatField(%q, %q) expected error, got nil", tt.field, tt.input)
				}
				if !strings.Contains(err.Error(), tt.field) {
					t.Errorf("error %q should mention field name %q", err.Error(), tt.field)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFloatField(%q, %q) unexpected error: %v", tt.field, tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseFloatField(%q, %q) = %v, want %v", tt.field, tt.input, got, tt.want)
			}
		})
	}
}
