package client

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SaleRecord struct {
	District       string
	PropertyID     string
	UnitNumber     string
	HouseNumber    string
	Street         string
	Suburb         string
	Postcode       string
	Area           float64
	AreaType       string
	ContractDate   time.Time
	SettlementDate time.Time
	Price          int64
	Zone           string
	Nature         string
	Purpose        string
	StrataLot      string
	DealingNumber  string
}

func DownloadNSWVGWeekly(ctx context.Context, date time.Time) ([]byte, error) {
	url := fmt.Sprintf("https://www.valuergeneral.nsw.gov.au/__psi/weekly/%s.zip", date.Format("20060102"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nsw vg download %s: %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func DownloadNSWVGYearly(ctx context.Context, year int) ([]byte, error) {
	url := fmt.Sprintf("https://www.valuergeneral.nsw.gov.au/__psi/yearly/%d.zip", year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nsw vg download %s: %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func ParseNSWVGZip(data []byte) ([]SaleRecord, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	var records []SaleRecord
	for _, f := range r.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".dat") {
			if strings.HasSuffix(strings.ToLower(f.Name), ".zip") {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				inner, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					continue
				}
				nested, err := ParseNSWVGZip(inner)
				if err != nil {
					continue
				}
				records = append(records, nested...)
			}
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		parsed := ParseDATFile(string(content))
		records = append(records, parsed...)
	}
	return records, nil
}

func ParseDATFile(content string) []SaleRecord {
	var records []SaleRecord
	var currentDistrict string

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ";")
		if len(fields) < 2 {
			continue
		}

		recType := strings.TrimSpace(fields[0])
		switch recType {
		case "A":
			if len(fields) > 1 {
				currentDistrict = strings.TrimSpace(fields[1])
			}
		case "B":
			rec := parseBRecord(fields, currentDistrict)
			if rec != nil {
				records = append(records, *rec)
			}
		}
	}
	return records
}

func parseBRecord(fields []string, district string) *SaleRecord {
	if len(fields) < 24 {
		return nil
	}

	get := func(i int) string {
		if i < len(fields) {
			return strings.TrimSpace(fields[i])
		}
		return ""
	}

	price, _ := strconv.ParseInt(get(15), 10, 64)
	if price == 0 {
		return nil
	}

	area, _ := strconv.ParseFloat(get(11), 64)
	contractDate := parseDate(get(13))
	settlementDate := parseDate(get(14))

	return &SaleRecord{
		District:       district,
		PropertyID:     get(2),
		UnitNumber:     get(5),
		HouseNumber:    get(6),
		Street:         get(8),
		Suburb:         get(9),
		Postcode:       get(10),
		Area:           area,
		AreaType:       get(12),
		ContractDate:   contractDate,
		SettlementDate: settlementDate,
		Price:          price,
		Zone:           get(16),
		Nature:         get(17),
		Purpose:        get(18),
		StrataLot:      get(19),
		DealingNumber:  get(23),
	}
}

func parseDate(s string) time.Time {
	for _, layout := range []string{"20060102", "2006-01-02", "02/01/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
