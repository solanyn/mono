package service

import (
	"testing"

	"github.com/solanyn/mono/yield/api/internal/domain"
)

func TestBuildMarketOverview(t *testing.T) {
	stats := domain.SuburbStats{
		Suburb:           "MARRICKVILLE",
		State:            "NSW",
		MedianPrice:      1200000,
		MeanPrice:        1350000,
		SaleCount:        45,
		AuctionClearance: 72.5,
		DaysOnMarket:     28,
		MedianYield:      3.2,
	}

	result := BuildMarketOverview(stats)
	if result.Suburb != "MARRICKVILLE" {
		t.Errorf("expected MARRICKVILLE, got %s", result.Suburb)
	}
	if result.MedianPrice != 1200000 {
		t.Errorf("expected 1200000, got %d", result.MedianPrice)
	}
	if result.SaleCount != 45 {
		t.Errorf("expected 45, got %d", result.SaleCount)
	}
	if result.AuctionClearance != 72.5 {
		t.Errorf("expected 72.5, got %f", result.AuctionClearance)
	}
}
