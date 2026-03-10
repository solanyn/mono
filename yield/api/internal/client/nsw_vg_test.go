package client

import "testing"

func TestParseDATFile(t *testing.T) {
	content := `A;SYDNEY;20260302
B;;PROP001;;1;5;10;SMITH;ST;MARRICKVILLE;2204;300;M;20260215;20260301;950000;;R;RESIDENCE;LOT1;;;;DEAL001
B;;PROP002;;2;7;22;JONES;RD;NEWTOWN;2042;0;M;20260220;20260305;1200000;;R;RESIDENCE;;;DEAL002;;;
C;LOT1;DP12345
D;PURCHASER;REDACTED
B;;PROP003;;3;9;15;KING;AVE;ENMORE;2042;450;M;20260225;20260310;0;;R;RESIDENCE;;;DEAL003;;;
`

	records := ParseDATFile(content)
	if len(records) != 2 {
		t.Fatalf("expected 2 records (skip zero price), got %d", len(records))
	}

	r := records[0]
	if r.District != "SYDNEY" {
		t.Errorf("expected district SYDNEY, got %s", r.District)
	}
	if r.PropertyID != "PROP001" {
		t.Errorf("expected PROP001, got %s", r.PropertyID)
	}
	if r.Suburb != "MARRICKVILLE" {
		t.Errorf("expected MARRICKVILLE, got %s", r.Suburb)
	}
	if r.Price != 950000 {
		t.Errorf("expected 950000, got %d", r.Price)
	}
	if r.Nature != "R" {
		t.Errorf("expected R, got %s", r.Nature)
	}
	if r.DealingNumber != "DEAL001" {
		t.Errorf("expected DEAL001, got %s", r.DealingNumber)
	}

	r2 := records[1]
	if r2.Price != 1200000 {
		t.Errorf("expected 1200000, got %d", r2.Price)
	}
}

func TestParseDATFileEmpty(t *testing.T) {
	records := ParseDATFile("")
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestParseBRecordShortFields(t *testing.T) {
	fields := []string{"B", "PROP", "UNIT"}
	result := parseBRecord(fields, "TEST")
	if result != nil {
		t.Error("expected nil for short fields")
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input string
		year  int
	}{
		{"20260215", 2026},
		{"2026-02-15", 2026},
		{"15/02/2026", 2026},
		{"invalid", 1},
	}

	for _, tt := range tests {
		d := parseDate(tt.input)
		if d.Year() != tt.year {
			t.Errorf("parseDate(%q): expected year %d, got %d", tt.input, tt.year, d.Year())
		}
	}
}
