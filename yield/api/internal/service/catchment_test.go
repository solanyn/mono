package service

import (
	"testing"

	"github.com/solanyn/mono/yield/api/internal/domain"
)

func TestGroupCatchments(t *testing.T) {
	catchments := []domain.SchoolCatchment{
		{UseID: "1", CatchType: "PRIMARY", School: "Marrickville Public"},
		{UseID: "2", CatchType: "HIGH_COED", School: "Marrickville High"},
		{UseID: "3", CatchType: "INFANTS", School: "Marrickville Infants"},
	}

	result := GroupCatchments(catchments)
	if len(result.Primary) != 2 {
		t.Errorf("expected 2 primary, got %d", len(result.Primary))
	}
	if len(result.Secondary) != 1 {
		t.Errorf("expected 1 secondary, got %d", len(result.Secondary))
	}
}

func TestGroupCatchmentsEmpty(t *testing.T) {
	result := GroupCatchments(nil)
	if len(result.Primary) != 0 || len(result.Secondary) != 0 {
		t.Error("expected empty result")
	}
}
