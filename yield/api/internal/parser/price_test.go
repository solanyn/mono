package parser

import "testing"

func ptr(v int64) *int64 { return &v }

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input    string
		wantVal  *int64
		wantLow  *int64
		wantHigh *int64
		weekly   bool
	}{
		{"$1,200,000", ptr(1200000), nil, nil, false},
		{"$1.2M", ptr(1200000), nil, nil, false},
		{"$1.2m", ptr(1200000), nil, nil, false},
		{"$3M", ptr(3000000), nil, nil, false},
		{"$950 per week", ptr(950), nil, nil, true},
		{"$950 pw", ptr(950), nil, nil, true},
		{"$850 p/w", ptr(850), nil, nil, true},
		{"$1,200,000 - $1,300,000", ptr(1250000), ptr(1200000), ptr(1300000), false},
		{"$800,000 – $900,000", ptr(850000), ptr(800000), ptr(900000), false},
		{"Auction Guide $3M", ptr(3000000), nil, nil, false},
		{"Contact Agent", nil, nil, nil, false},
		{"Expressions of Interest", nil, nil, nil, false},
		{"POA", nil, nil, nil, false},
		{"", nil, nil, nil, false},
		{"$500k", ptr(500000), nil, nil, false},
		{"$2.5k", ptr(2500), nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParsePrice(tt.input)

			if tt.wantVal == nil && got.Value != nil {
				t.Errorf("ParsePrice(%q).Value = %d, want nil", tt.input, *got.Value)
			}
			if tt.wantVal != nil && got.Value == nil {
				t.Errorf("ParsePrice(%q).Value = nil, want %d", tt.input, *tt.wantVal)
			}
			if tt.wantVal != nil && got.Value != nil && *got.Value != *tt.wantVal {
				t.Errorf("ParsePrice(%q).Value = %d, want %d", tt.input, *got.Value, *tt.wantVal)
			}

			if tt.wantLow != nil && got.Low != nil && *got.Low != *tt.wantLow {
				t.Errorf("ParsePrice(%q).Low = %d, want %d", tt.input, *got.Low, *tt.wantLow)
			}
			if tt.wantHigh != nil && got.High != nil && *got.High != *tt.wantHigh {
				t.Errorf("ParsePrice(%q).High = %d, want %d", tt.input, *got.High, *tt.wantHigh)
			}

			if got.IsWeekly != tt.weekly {
				t.Errorf("ParsePrice(%q).IsWeekly = %v, want %v", tt.input, got.IsWeekly, tt.weekly)
			}
		})
	}
}
