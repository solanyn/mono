package service

import (
	"testing"
	"time"

	"github.com/solanyn/mono/yield/api/internal/domain"
)

func TestEstimateValue(t *testing.T) {
	comps := []domain.Property{
		{SalePrice: 800000},
		{SalePrice: 850000},
		{SalePrice: 900000},
		{SalePrice: 950000},
		{SalePrice: 1000000},
	}

	result := EstimateValue(comps, false)
	if result.CompCount != 5 {
		t.Errorf("expected 5 comps, got %d", result.CompCount)
	}
	if result.EstimateMid != 900000 {
		t.Errorf("expected mid 900000, got %d", result.EstimateMid)
	}
	if result.EstimateLow >= result.EstimateMid {
		t.Errorf("low %d should be < mid %d", result.EstimateLow, result.EstimateMid)
	}
	if result.EstimateHigh <= result.EstimateMid {
		t.Errorf("high %d should be > mid %d", result.EstimateHigh, result.EstimateMid)
	}
	if result.IsStrata {
		t.Error("expected non-strata")
	}
}

func TestEstimateValueEmpty(t *testing.T) {
	result := EstimateValue(nil, false)
	if result.CompCount != 0 {
		t.Errorf("expected 0 comps, got %d", result.CompCount)
	}
}

func TestEstimateValueStrata(t *testing.T) {
	comps := []domain.Property{
		{SalePrice: 500000},
		{SalePrice: 550000},
		{SalePrice: 600000},
	}
	result := EstimateValue(comps, true)
	if !result.IsStrata {
		t.Error("expected strata")
	}
}

func TestCalculateCapitalGrowth(t *testing.T) {
	purchase := time.Now().Add(-5 * 365 * 24 * time.Hour)
	result := CalculateCapitalGrowth(500000, purchase, 700000)

	if result.AbsoluteGain != 200000 {
		t.Errorf("expected gain 200000, got %d", result.AbsoluteGain)
	}
	if result.TotalReturnPct < 39 || result.TotalReturnPct > 41 {
		t.Errorf("expected ~40%% total return, got %.1f%%", result.TotalReturnPct)
	}
	if result.AnnualizedPct < 5 || result.AnnualizedPct > 9 {
		t.Errorf("expected ~7%% annualized, got %.1f%%", result.AnnualizedPct)
	}
}

func TestCalculateCapitalGrowthZero(t *testing.T) {
	result := CalculateCapitalGrowth(0, time.Now(), 700000)
	if result.AbsoluteGain != 0 {
		t.Errorf("expected 0 gain for zero purchase, got %d", result.AbsoluteGain)
	}
}
