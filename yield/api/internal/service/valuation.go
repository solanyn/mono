package service

import (
	"math"
	"sort"
	"time"

	"github.com/solanyn/mono/yield/api/internal/domain"
)

type ValuationResult struct {
	EstimateLow  int64   `json:"estimate_low"`
	EstimateHigh int64   `json:"estimate_high"`
	EstimateMid  int64   `json:"estimate_mid"`
	Confidence   float64 `json:"confidence"`
	CompCount    int     `json:"comparable_count"`
	IsStrata     bool    `json:"is_strata"`
}

type CapitalGrowth struct {
	PurchasePrice   int64   `json:"purchase_price"`
	CurrentEstimate int64   `json:"current_estimate"`
	AbsoluteGain    int64   `json:"absolute_gain"`
	TotalReturnPct  float64 `json:"total_return_pct"`
	AnnualizedPct   float64 `json:"annualized_return_pct"`
	YearsHeld       float64 `json:"years_held"`
}

func EstimateValue(comps []domain.Property, isStrata bool) ValuationResult {
	if len(comps) == 0 {
		return ValuationResult{}
	}

	prices := make([]int64, 0, len(comps))
	for _, c := range comps {
		if c.SalePrice > 0 {
			prices = append(prices, c.SalePrice)
		}
	}
	if len(prices) == 0 {
		return ValuationResult{}
	}

	sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })

	p25 := percentile(prices, 25)
	p50 := percentile(prices, 50)
	p75 := percentile(prices, 75)

	confidence := math.Min(float64(len(prices))/10.0, 1.0)

	return ValuationResult{
		EstimateLow:  p25,
		EstimateHigh: p75,
		EstimateMid:  p50,
		Confidence:   confidence,
		CompCount:    len(prices),
		IsStrata:     isStrata,
	}
}

func CalculateCapitalGrowth(purchasePrice int64, purchaseDate time.Time, currentEstimate int64) CapitalGrowth {
	if purchasePrice == 0 {
		return CapitalGrowth{}
	}

	years := time.Since(purchaseDate).Hours() / (24 * 365.25)
	if years <= 0 {
		years = 1
	}

	gain := currentEstimate - purchasePrice
	totalReturn := float64(gain) / float64(purchasePrice) * 100
	annualized := (math.Pow(float64(currentEstimate)/float64(purchasePrice), 1.0/years) - 1) * 100

	return CapitalGrowth{
		PurchasePrice:   purchasePrice,
		CurrentEstimate: currentEstimate,
		AbsoluteGain:    gain,
		TotalReturnPct:  totalReturn,
		AnnualizedPct:   annualized,
		YearsHeld:       years,
	}
}

func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	k := float64(p) / 100.0 * float64(len(sorted)-1)
	f := int(k)
	if f >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	frac := k - float64(f)
	return int64(float64(sorted[f])*(1-frac) + float64(sorted[f+1])*frac)
}
