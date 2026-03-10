package domain

import "testing"

func TestGrossYield(t *testing.T) {
	tests := []struct {
		name       string
		annualRent int
		price      int64
		want       float64
	}{
		{"typical investment", 26000, 500000, 5.2},
		{"high yield", 36400, 400000, 9.1},
		{"low yield", 15600, 800000, 1.95},
		{"zero price", 26000, 0, 0},
		{"zero rent", 0, 500000, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GrossYield(tt.annualRent, tt.price)
			if got != tt.want {
				t.Errorf("GrossYield(%d, %d) = %v, want %v", tt.annualRent, tt.price, got, tt.want)
			}
		})
	}
}

func TestRentFairness(t *testing.T) {
	tests := []struct {
		yield float64
		want  string
	}{
		{7.0, "excellent"},
		{6.0, "excellent"},
		{5.0, "good"},
		{4.5, "good"},
		{3.5, "fair"},
		{3.0, "fair"},
		{2.0, "poor"},
		{0, "poor"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RentFairness(tt.yield)
			if got != tt.want {
				t.Errorf("RentFairness(%v) = %q, want %q", tt.yield, got, tt.want)
			}
		})
	}
}

func TestAnalyse(t *testing.T) {
	stats := &SuburbStats{
		Suburb:      "Surry Hills",
		MedianPrice: 1000000,
		MeanPrice:   1100000,
		SaleCount:   50,
	}

	req := AnalyseRequest{
		Address:     "123 Crown St",
		Price:       900000,
		RentPerWeek: 750,
	}

	result := Analyse(req, stats)

	expectedYield := float64(750*52) / float64(900000) * 100
	if result.GrossYield != expectedYield {
		t.Errorf("GrossYield = %v, want %v", result.GrossYield, expectedYield)
	}

	if result.RentFairness != "fair" {
		t.Errorf("RentFairness = %q, want %q", result.RentFairness, "fair")
	}

	expectedPriceVsMedian := (float64(900000) - float64(1000000)) / float64(1000000) * 100
	if result.PriceVsMedian != expectedPriceVsMedian {
		t.Errorf("PriceVsMedian = %v, want %v", result.PriceVsMedian, expectedPriceVsMedian)
	}

	if result.SuburbMedian != 1000000 {
		t.Errorf("SuburbMedian = %v, want %v", result.SuburbMedian, 1000000)
	}
}

func TestAnalyseNoStats(t *testing.T) {
	req := AnalyseRequest{
		Address:     "123 Crown St",
		Price:       500000,
		RentPerWeek: 500,
	}

	result := Analyse(req, nil)

	if result.PriceVsMedian != 0 {
		t.Errorf("PriceVsMedian = %v, want 0", result.PriceVsMedian)
	}
	if result.SuburbMedian != 0 {
		t.Errorf("SuburbMedian = %v, want 0", result.SuburbMedian)
	}
}
