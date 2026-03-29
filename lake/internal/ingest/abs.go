package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const absURL = "https://data.api.abs.gov.au/data/CPI/1+2.10001+10002+10003+20001+20002+20003+20004+30001+30002+30003+40001+40002+40003+50001+50002+50003+60001+60002+60003+70001+70002+70003+80001+80002+80003+90001+90002+90003+100001+100002+100003+110001+110002+110003+120001+120002+120003+131179.10+20.M?format=jsondata&detail=dataonly&startPeriod=2020-01"

func IngestABS(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "abs"

	rows, err := fetchABS(ctx)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("fetch abs: %w", err)
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "abs.cpi", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, bucket, "abs", "cpi_monthly.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("abs: wrote %d rows to %s", len(rows), key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

func fetchABS(ctx context.Context) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, absURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("abs http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseABSJSON(body)
}

func parseABSJSON(data []byte) ([]map[string]interface{}, error) {
	var envelope map[string]interface{}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	dataSets, ok := envelope["dataSets"].([]interface{})
	if !ok || len(dataSets) == 0 {
		return nil, fmt.Errorf("no dataSets")
	}

	ds := dataSets[0].(map[string]interface{})
	series, ok := ds["series"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no series")
	}

	structure, _ := envelope["structure"].(map[string]interface{})
	dimensions, _ := structure["dimensions"].(map[string]interface{})
	obsDim := extractDimValues(dimensions, "observation")
	seriesDim := extractDimValues(dimensions, "series")

	var rows []map[string]interface{}
	for seriesKey, seriesVal := range series {
		sv := seriesVal.(map[string]interface{})
		observations, _ := sv["observations"].(map[string]interface{})

		seriesLabels := resolveSeriesLabels(seriesKey, seriesDim)

		for obsKey, obsVal := range observations {
			ov := obsVal.([]interface{})
			if len(ov) == 0 {
				continue
			}

			row := make(map[string]interface{})
			for k, v := range seriesLabels {
				row[k] = v
			}

			timePeriod := resolveObsDim(obsKey, obsDim)
			row["time_period"] = timePeriod
			row["value"] = ov[0]

			rows = append(rows, row)
		}
	}

	return rows, nil
}

func extractDimValues(dims map[string]interface{}, key string) [][]map[string]interface{} {
	dimList, ok := dims[key].([]interface{})
	if !ok {
		return nil
	}
	var result [][]map[string]interface{}
	for _, d := range dimList {
		dm := d.(map[string]interface{})
		vals, _ := dm["values"].([]interface{})
		var dimVals []map[string]interface{}
		for _, v := range vals {
			dimVals = append(dimVals, v.(map[string]interface{}))
		}
		result = append(result, dimVals)
	}
	return result
}

func resolveSeriesLabels(key string, dims [][]map[string]interface{}) map[string]interface{} {
	parts := splitKey(key)
	labels := make(map[string]interface{})
	for i, idx := range parts {
		if i < len(dims) && idx < len(dims[i]) {
			v := dims[i][idx]
			id, _ := v["id"].(string)
			name, _ := v["name"].(string)
			labels[fmt.Sprintf("dim_%d_id", i)] = id
			labels[fmt.Sprintf("dim_%d_name", i)] = name
			if i == 0 {
				labels["indicator_id"] = id
				labels["indicator_name"] = name
			}
		}
	}
	return labels
}

func resolveObsDim(key string, dims [][]map[string]interface{}) string {
	parts := splitKey(key)
	if len(parts) > 0 && len(dims) > 0 && parts[0] < len(dims[0]) {
		if id, ok := dims[0][parts[0]]["id"].(string); ok {
			return id
		}
	}
	return key
}

func splitKey(key string) []int {
	var result []int
	for _, p := range strings.Split(key, ":") {
		var n int
		fmt.Sscanf(p, "%d", &n)
		result = append(result, n)
	}
	return result
}
