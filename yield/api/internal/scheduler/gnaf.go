package scheduler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/solanyn/mono/yield/api/internal/metrics"
	"github.com/solanyn/mono/yield/api/internal/store"
)

const gnafURL = "https://data.gov.au/data/dataset/19432f89-dc3a-4ef3-b943-5326ef1dbecc/resource/4b084096-65e4-4c8e-abbe-5e54ff85f42f/download/feb25_gnaf_pipeseparatedvalue.zip"

func (s *Scheduler) syncGNAF() {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	log.Println("gnaf: downloading GNAF archive")
	data, err := downloadURL(ctx, gnafURL)
	if err != nil {
		log.Printf("gnaf: download: %v", err)
		return
	}
	log.Printf("gnaf: downloaded %d MB", len(data)/1024/1024)

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		log.Printf("gnaf: zip open: %v", err)
		return
	}

	var addressFile, localityFile, stateFile *zip.File
	for _, f := range r.File {
		lower := strings.ToLower(f.Name)
		if strings.Contains(lower, "address_default_geocode_psv") && strings.HasSuffix(lower, ".psv") {
			addressFile = f
		}
		if strings.Contains(lower, "address_detail_psv") && strings.HasSuffix(lower, ".psv") && !strings.Contains(lower, "mesh") {
			if addressFile == nil {
				addressFile = f
			}
		}
		if strings.Contains(lower, "locality_psv") && strings.HasSuffix(lower, ".psv") && !strings.Contains(lower, "alias") && !strings.Contains(lower, "neighbour") && !strings.Contains(lower, "point") {
			localityFile = f
		}
		if strings.Contains(lower, "state_psv") && strings.HasSuffix(lower, ".psv") {
			stateFile = f
		}
	}

	states := make(map[string]string)
	if stateFile != nil {
		states = parseStatePSV(stateFile)
	}

	localities := make(map[string]localityInfo)
	if localityFile != nil {
		localities = parseLocalityPSV(localityFile, states)
	}

	var total int64
	for _, f := range r.File {
		lower := strings.ToLower(f.Name)
		if !strings.HasSuffix(lower, ".psv") {
			continue
		}
		if !strings.Contains(lower, "address_detail_psv") {
			continue
		}
		if strings.Contains(lower, "mesh") {
			continue
		}

		log.Printf("gnaf: processing %s", f.Name)
		count, err := s.processAddressDetailPSV(ctx, f, localities)
		if err != nil {
			log.Printf("gnaf: process %s: %v", f.Name, err)
			continue
		}
		total += count
		log.Printf("gnaf: %s — %d addresses", f.Name, count)
	}

	if total == 0 {
		for _, f := range r.File {
			lower := strings.ToLower(f.Name)
			if strings.Contains(lower, ".zip") {
				log.Printf("gnaf: found nested zip %s, processing", f.Name)
				nested, err := processNestedGNAFZip(ctx, f, s, localities)
				if err != nil {
					log.Printf("gnaf: nested %s: %v", f.Name, err)
					continue
				}
				total += nested
			}
		}
	}

	metrics.Global.SalesIngested.Add(total)
	log.Printf("gnaf: sync complete — %d total addresses", total)
}

type localityInfo struct {
	Name     string
	State    string
	Postcode string
}

func parseStatePSV(f *zip.File) map[string]string {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()

	reader := csv.NewReader(rc)
	reader.Comma = '|'
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return nil
	}

	pidIdx := colIndex(header, "STATE_PID")
	abbrIdx := colIndex(header, "STATE_ABBREVIATION")

	states := make(map[string]string)
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		if pidIdx >= 0 && abbrIdx >= 0 && pidIdx < len(row) && abbrIdx < len(row) {
			states[strings.TrimSpace(row[pidIdx])] = strings.TrimSpace(row[abbrIdx])
		}
	}
	return states
}

func parseLocalityPSV(f *zip.File, states map[string]string) map[string]localityInfo {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()

	reader := csv.NewReader(rc)
	reader.Comma = '|'
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return nil
	}

	pidIdx := colIndex(header, "LOCALITY_PID")
	nameIdx := colIndex(header, "LOCALITY_NAME")
	stateIdx := colIndex(header, "STATE_PID")

	localities := make(map[string]localityInfo)
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		if pidIdx >= 0 && nameIdx >= 0 && pidIdx < len(row) && nameIdx < len(row) {
			info := localityInfo{Name: strings.TrimSpace(row[nameIdx])}
			if stateIdx >= 0 && stateIdx < len(row) {
				info.State = states[strings.TrimSpace(row[stateIdx])]
			}
			localities[strings.TrimSpace(row[pidIdx])] = info
		}
	}
	return localities
}

func (s *Scheduler) processAddressDetailPSV(ctx context.Context, f *zip.File, localities map[string]localityInfo) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	reader := csv.NewReader(rc)
	reader.Comma = '|'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return 0, err
	}

	pidIdx := colIndex(header, "ADDRESS_DETAIL_PID")
	numIdx := colIndex(header, "NUMBER_FIRST")
	streetNameIdx := colIndex(header, "STREET_NAME")
	streetTypeIdx := colIndex(header, "STREET_TYPE_CODE")
	localityIdx := colIndex(header, "LOCALITY_PID")
	postcodeIdx := colIndex(header, "POSTCODE")
	latIdx := colIndex(header, "LATITUDE")
	lonIdx := colIndex(header, "LONGITUDE")

	if pidIdx < 0 {
		return 0, fmt.Errorf("ADDRESS_DETAIL_PID column not found in %v", header)
	}

	var batch []store.GNAFAddress
	var total int64

	for {
		row, err := reader.Read()
		if err != nil {
			break
		}

		pid := getField(row, pidIdx)
		if pid == "" {
			continue
		}

		addr := store.GNAFAddress{GNAFPID: pid}

		if v := getField(row, numIdx); v != "" {
			addr.StreetNumber = &v
		}
		if v := getField(row, streetNameIdx); v != "" {
			addr.StreetName = &v
		}
		if v := getField(row, streetTypeIdx); v != "" {
			addr.StreetType = &v
		}
		if v := getField(row, postcodeIdx); v != "" {
			addr.Postcode = &v
		}

		if loc, ok := localities[getField(row, localityIdx)]; ok {
			addr.Suburb = &loc.Name
			addr.State = &loc.State
		}

		if v := getField(row, latIdx); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				addr.Lat = &f
			}
		}
		if v := getField(row, lonIdx); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				addr.Lon = &f
			}
		}

		batch = append(batch, addr)
		if len(batch) >= 1000 {
			if err := s.bulkUpsertGNAF(ctx, batch); err != nil {
				return total, err
			}
			total += int64(len(batch))
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := s.bulkUpsertGNAF(ctx, batch); err != nil {
			return total, err
		}
		total += int64(len(batch))
	}

	return total, nil
}

func (s *Scheduler) bulkUpsertGNAF(ctx context.Context, addrs []store.GNAFAddress) error {
	for _, a := range addrs {
		if err := s.queries.UpsertGNAFAddress(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func processNestedGNAFZip(ctx context.Context, f *zip.File, s *Scheduler, localities map[string]localityInfo) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	inner, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return 0, err
	}

	r, err := zip.NewReader(bytes.NewReader(inner), int64(len(inner)))
	if err != nil {
		return 0, err
	}

	var total int64
	for _, nested := range r.File {
		lower := strings.ToLower(nested.Name)
		if strings.Contains(lower, "address_detail_psv") && strings.HasSuffix(lower, ".psv") && !strings.Contains(lower, "mesh") {
			count, err := s.processAddressDetailPSV(ctx, nested, localities)
			if err != nil {
				log.Printf("gnaf: nested process %s: %v", nested.Name, err)
				continue
			}
			total += count
		}
	}
	return total, nil
}

func downloadURL(ctx context.Context, url string) ([]byte, error) {
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
		return nil, fmt.Errorf("download %s: %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func colIndex(header []string, name string) int {
	for i, h := range header {
		if strings.TrimSpace(strings.ToUpper(h)) == name {
			return i
		}
	}
	return -1
}

func getField(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}
