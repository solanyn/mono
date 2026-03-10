package service

import "github.com/solanyn/mono/yield/api/internal/domain"

type CatchmentResult struct {
	Primary   []domain.SchoolCatchment `json:"primary"`
	Secondary []domain.SchoolCatchment `json:"secondary"`
}

func GroupCatchments(catchments []domain.SchoolCatchment) CatchmentResult {
	var result CatchmentResult
	for _, c := range catchments {
		switch c.CatchType {
		case "PRIMARY", "INFANTS":
			result.Primary = append(result.Primary, c)
		default:
			result.Secondary = append(result.Secondary, c)
		}
	}
	return result
}
