package parser

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	reCleanPrice   = regexp.MustCompile(`[,$\s]`)
	reNumeric      = regexp.MustCompile(`(\d+(?:\.\d+)?)`)
	reMillions     = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*m\b`)
	reThousands    = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*k\b`)
	reRange        = regexp.MustCompile(`(?i)\$?([\d,.]+)\s*[-–]\s*\$?([\d,.]+)`)
	rePerWeek      = regexp.MustCompile(`(?i)([\d,.]+)\s*(?:per\s*week|pw|p/w)`)
	reContactAgent = regexp.MustCompile(`(?i)contact\s*agent|expression|POA|price\s*on\s*application`)
)

type PriceResult struct {
	Value    *int64
	Low      *int64
	High     *int64
	IsWeekly bool
}

func ParsePrice(s string) PriceResult {
	s = strings.TrimSpace(s)
	if s == "" || reContactAgent.MatchString(s) {
		return PriceResult{}
	}

	if m := rePerWeek.FindStringSubmatch(s); m != nil {
		v := parseCleanNumber(m[1])
		return PriceResult{Value: &v, IsWeekly: true}
	}

	if m := reRange.FindStringSubmatch(s); m != nil {
		low := parseCleanNumber(m[1])
		high := parseCleanNumber(m[2])
		mid := int64(math.Round(float64(low+high) / 2))
		return PriceResult{Value: &mid, Low: &low, High: &high}
	}

	if m := reMillions.FindStringSubmatch(s); m != nil {
		f, _ := strconv.ParseFloat(m[1], 64)
		v := int64(f * 1_000_000)
		return PriceResult{Value: &v}
	}

	if m := reThousands.FindStringSubmatch(s); m != nil {
		f, _ := strconv.ParseFloat(m[1], 64)
		v := int64(f * 1_000)
		return PriceResult{Value: &v}
	}

	cleaned := reCleanPrice.ReplaceAllString(s, "")
	if m := reNumeric.FindString(cleaned); m != "" {
		f, err := strconv.ParseFloat(m, 64)
		if err == nil && f > 0 {
			v := int64(f)
			return PriceResult{Value: &v}
		}
	}

	return PriceResult{}
}

func parseCleanNumber(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.TrimSpace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return int64(f)
}
