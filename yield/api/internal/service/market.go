package service

import "github.com/solanyn/mono/yield/api/internal/domain"

type MarketOverview struct {
	Suburb           string  `json:"suburb"`
	State            string  `json:"state"`
	MedianPrice      int64   `json:"median_price"`
	MeanPrice        int64   `json:"mean_price"`
	SaleCount        int     `json:"sale_count"`
	AuctionClearance float64 `json:"auction_clearance_pct"`
	DaysOnMarket     int     `json:"days_on_market"`
	MedianYield      float64 `json:"median_yield_pct"`
}

func BuildMarketOverview(stats domain.SuburbStats) MarketOverview {
	return MarketOverview{
		Suburb:           stats.Suburb,
		State:            stats.State,
		MedianPrice:      stats.MedianPrice,
		MeanPrice:        stats.MeanPrice,
		SaleCount:        stats.SaleCount,
		AuctionClearance: stats.AuctionClearance,
		DaysOnMarket:     stats.DaysOnMarket,
		MedianYield:      stats.MedianYield,
	}
}
