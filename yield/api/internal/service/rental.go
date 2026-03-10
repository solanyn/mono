package service

import (
	"math"
	"sort"

	"github.com/solanyn/mono/yield/api/internal/domain"
)

type RentAnalysis struct {
	MedianRent      int     `json:"median_rent_pw"`
	Percentile      float64 `json:"percentile"`
	Fairness        string  `json:"fairness"`
	ComparableCount int     `json:"comparable_count"`
	VacancyProxy    float64 `json:"vacancy_proxy_pct"`
}

func AnalyzeRent(currentRent int, comparables []domain.Listing) RentAnalysis {
	if len(comparables) == 0 {
		return RentAnalysis{Fairness: "insufficient_data"}
	}

	rents := make([]int, 0, len(comparables))
	for _, l := range comparables {
		if l.PriceNumeric != nil && *l.PriceNumeric > 0 {
			rents = append(rents, int(*l.PriceNumeric))
		}
	}
	if len(rents) == 0 {
		return RentAnalysis{Fairness: "insufficient_data"}
	}

	sort.Ints(rents)
	median := rents[len(rents)/2]

	pctile := calculatePercentile(rents, currentRent)

	var fairness string
	switch {
	case pctile <= 25:
		fairness = "below_market"
	case pctile <= 50:
		fairness = "fair"
	case pctile <= 75:
		fairness = "above_average"
	default:
		fairness = "expensive"
	}

	return RentAnalysis{
		MedianRent:      median,
		Percentile:      pctile,
		Fairness:        fairness,
		ComparableCount: len(rents),
	}
}

func calculatePercentile(sorted []int, value int) float64 {
	below := 0
	for _, v := range sorted {
		if v < value {
			below++
		}
	}
	return math.Round(float64(below)/float64(len(sorted))*100*10) / 10
}
