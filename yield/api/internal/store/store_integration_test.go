package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/solanyn/mono/yield/api/internal/store"
	"github.com/solanyn/mono/yield/api/internal/testutil"
)

func TestBulkInsertSalesIntegration(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(db.Cleanup)

	q := store.New(db.Pool)
	ctx := context.Background()

	propID := "TEST-001"
	street := "CROWN ST"
	houseNum := "123"
	postcode := "2010"
	area := 150.0
	areaType := "M"
	nature := "R"
	purpose := "RESIDENCE"
	dealing1 := "AA111111"
	dealing2 := "AA222222"
	price1 := int64(950000)
	price2 := int64(1050000)
	contractDate1 := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	contractDate2 := time.Date(2025, 9, 20, 0, 0, 0, 0, time.UTC)

	sales := []store.Sale{
		{
			District:      "SYDNEY",
			PropertyID:    &propID,
			HouseNumber:   &houseNum,
			Street:        &street,
			Suburb:        "SURRY HILLS",
			Postcode:      &postcode,
			Area:          &area,
			AreaType:      &areaType,
			ContractDate:  &contractDate1,
			Price:         &price1,
			Nature:        &nature,
			Purpose:       &purpose,
			DealingNumber: &dealing1,
			Source:        "nsw_vg",
		},
		{
			District:      "SYDNEY",
			PropertyID:    &propID,
			HouseNumber:   &houseNum,
			Street:        &street,
			Suburb:        "SURRY HILLS",
			Postcode:      &postcode,
			Area:          &area,
			AreaType:      &areaType,
			ContractDate:  &contractDate2,
			Price:         &price2,
			Nature:        &nature,
			Purpose:       &purpose,
			DealingNumber: &dealing2,
			Source:        "nsw_vg",
		},
	}

	inserted, err := q.BulkInsertSales(ctx, sales)
	if err != nil {
		t.Fatalf("BulkInsertSales: %v", err)
	}
	if inserted != 2 {
		t.Errorf("BulkInsertSales inserted = %d, want 2", inserted)
	}

	inserted2, err := q.BulkInsertSales(ctx, sales)
	if err != nil {
		t.Fatalf("BulkInsertSales (idempotent): %v", err)
	}
	if inserted2 != 2 {
		t.Logf("BulkInsertSales idempotent: %d (ON CONFLICT DO NOTHING)", inserted2)
	}

	results, err := q.GetSalesBySuburb(ctx, "SURRY HILLS", 10)
	if err != nil {
		t.Fatalf("GetSalesBySuburb: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("GetSalesBySuburb count = %d, want 2", len(results))
	}

	for _, s := range results {
		if s.Suburb != "SURRY HILLS" {
			t.Errorf("sale suburb = %q, want SURRY HILLS", s.Suburb)
		}
		if s.Price == nil || (*s.Price != 950000 && *s.Price != 1050000) {
			t.Errorf("unexpected price: %v", s.Price)
		}
	}

	since := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	median, err := q.GetSuburbMedianPrice(ctx, "SURRY HILLS", since)
	if err != nil {
		t.Fatalf("GetSuburbMedianPrice: %v", err)
	}
	if median.SaleCount != 2 {
		t.Errorf("median sale_count = %d, want 2", median.SaleCount)
	}
	if median.MedianPrice == nil {
		t.Fatal("median price is nil")
	}
	expectedMedian := float64(950000+1050000) / 2
	if *median.MedianPrice < expectedMedian-1 || *median.MedianPrice > expectedMedian+1 {
		t.Errorf("median price = %v, want ~%v", *median.MedianPrice, expectedMedian)
	}

	comps, err := q.GetComparableSales(ctx, "SURRY HILLS", nil, since, 900000, 1100000, 10)
	if err != nil {
		t.Fatalf("GetComparableSales: %v", err)
	}
	if len(comps) != 2 {
		t.Errorf("GetComparableSales count = %d, want 2", len(comps))
	}

	compsNarrow, err := q.GetComparableSales(ctx, "SURRY HILLS", nil, since, 900000, 960000, 10)
	if err != nil {
		t.Fatalf("GetComparableSales narrow: %v", err)
	}
	if len(compsNarrow) != 1 {
		t.Errorf("GetComparableSales narrow count = %d, want 1", len(compsNarrow))
	}
}

func TestPortfolioIntegration(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(db.Cleanup)

	q := store.New(db.Pool)
	ctx := context.Background()

	postcode := "2010"
	propType := "unit"
	beds := int16(2)
	baths := int16(1)
	price := int64(750000)
	rent := int32(650)

	p, err := q.InsertPortfolioProperty(ctx, store.PortfolioProperty{
		Address:       "5/123 Crown St",
		Suburb:        "SURRY HILLS",
		Postcode:      &postcode,
		PropertyType:  &propType,
		Bedrooms:      &beds,
		Bathrooms:     &baths,
		PurchasePrice: &price,
		CurrentRentPW: &rent,
	})
	if err != nil {
		t.Fatalf("InsertPortfolioProperty: %v", err)
	}
	if p.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if p.Address != "5/123 Crown St" {
		t.Errorf("address = %q, want 5/123 Crown St", p.Address)
	}

	all, err := q.GetPortfolio(ctx)
	if err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("portfolio count = %d, want 1", len(all))
	}

	got, err := q.GetPortfolioProperty(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetPortfolioProperty: %v", err)
	}
	if got.Suburb != "SURRY HILLS" {
		t.Errorf("suburb = %q, want SURRY HILLS", got.Suburb)
	}

	newRent := int32(700)
	if err := q.UpdatePortfolioRent(ctx, p.ID, newRent); err != nil {
		t.Fatalf("UpdatePortfolioRent: %v", err)
	}

	updated, _ := q.GetPortfolioProperty(ctx, p.ID)
	if updated.CurrentRentPW == nil || *updated.CurrentRentPW != 700 {
		t.Errorf("rent after update = %v, want 700", updated.CurrentRentPW)
	}

	if err := q.DeletePortfolioProperty(ctx, p.ID); err != nil {
		t.Fatalf("DeletePortfolioProperty: %v", err)
	}

	afterDelete, _ := q.GetPortfolio(ctx)
	if len(afterDelete) != 0 {
		t.Errorf("portfolio after delete = %d, want 0", len(afterDelete))
	}
}
