package service

import (
	"testing"

	"github.com/solanyn/mono/yield/api/internal/domain"
)

func TestAnalyzeRent(t *testing.T) {
	p := func(v int64) *int64 { return &v }

	comps := []domain.Listing{
		{PriceNumeric: p(500)},
		{PriceNumeric: p(550)},
		{PriceNumeric: p(600)},
		{PriceNumeric: p(650)},
		{PriceNumeric: p(700)},
	}

	result := AnalyzeRent(600, comps)
	if result.ComparableCount != 5 {
		t.Errorf("expected 5 comps, got %d", result.ComparableCount)
	}
	if result.MedianRent != 600 {
		t.Errorf("expected median 600, got %d", result.MedianRent)
	}
	if result.Fairness != "fair" {
		t.Errorf("expected fair, got %s", result.Fairness)
	}
}

func TestAnalyzeRentExpensive(t *testing.T) {
	p := func(v int64) *int64 { return &v }

	comps := []domain.Listing{
		{PriceNumeric: p(400)},
		{PriceNumeric: p(450)},
		{PriceNumeric: p(500)},
		{PriceNumeric: p(550)},
	}

	result := AnalyzeRent(600, comps)
	if result.Fairness != "expensive" {
		t.Errorf("expected expensive, got %s", result.Fairness)
	}
}

func TestAnalyzeRentEmpty(t *testing.T) {
	result := AnalyzeRent(500, nil)
	if result.Fairness != "insufficient_data" {
		t.Errorf("expected insufficient_data, got %s", result.Fairness)
	}
}

func TestAnalyzeRentBelowMarket(t *testing.T) {
	p := func(v int64) *int64 { return &v }

	comps := []domain.Listing{
		{PriceNumeric: p(600)},
		{PriceNumeric: p(650)},
		{PriceNumeric: p(700)},
		{PriceNumeric: p(750)},
	}

	result := AnalyzeRent(500, comps)
	if result.Fairness != "below_market" {
		t.Errorf("expected below_market, got %s", result.Fairness)
	}
}
