package parser

import "testing"

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123 Crown Street", "123 CROWN ST"},
		{"45 George Road", "45 GEORGE RD"},
		{"1/23 Victoria Avenue", "1/23 VICTORIA AVE"},
		{"  10  Smith   Place  ", "10 SMITH PL"},
		{"7 Harbour Terrace", "7 HARBOUR TCE"},
		{"99 Pacific Highway", "99 PACIFIC HWY"},
		{"12 Oak Close", "12 OAK CL"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeAddress(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeAddress(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
