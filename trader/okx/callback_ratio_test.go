package okx

import "testing"

func TestNormalizeOKXCallbackRatio(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "okx percent fraction", in: 0.5531, want: 0.005531},
		{name: "already decimal", in: 0.005531, want: 0.005531},
		{name: "whole percent", in: 2.5, want: 0.025},
		{name: "zero", in: 0, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeOKXCallbackRatio(tc.in); got != tc.want {
				t.Fatalf("normalizeOKXCallbackRatio(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
