package parser

import (
	"regexp"
	"strings"
)

var reNormalize = regexp.MustCompile(`\s+`)

func NormalizeAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.ToUpper(addr)
	addr = reNormalize.ReplaceAllString(addr, " ")

	replacements := map[string]string{
		"STREET":    "ST",
		"ROAD":      "RD",
		"AVENUE":    "AVE",
		"DRIVE":     "DR",
		"PLACE":     "PL",
		"COURT":     "CT",
		"CRESCENT":  "CRES",
		"BOULEVARD": "BLVD",
		"LANE":      "LN",
		"TERRACE":   "TCE",
		"PARADE":    "PDE",
		"HIGHWAY":   "HWY",
		"CIRCUIT":   "CCT",
		"CLOSE":     "CL",
	}

	words := strings.Fields(addr)
	for i, w := range words {
		if r, ok := replacements[w]; ok {
			words[i] = r
		}
	}
	return strings.Join(words, " ")
}
