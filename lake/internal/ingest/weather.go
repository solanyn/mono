package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

type cityCoord struct {
	Name string
	Lat  float64
	Lon  float64
}

var auCities = []cityCoord{
	{"Sydney", -33.87, 151.21},
	{"Melbourne", -37.81, 144.96},
	{"Brisbane", -27.47, 153.03},
	{"Perth", -31.95, 115.86},
	{"Adelaide", -34.93, 138.60},
	{"Hobart", -42.88, 147.33},
	{"Canberra", -35.28, 149.13},
	{"Darwin", -12.46, 130.84},
}

func IngestWeather(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "weather"
	var rows []map[string]interface{}

	for _, city := range auCities {
		url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.2f&longitude=%.2f&daily=temperature_2m_max,temperature_2m_min,precipitation_sum,wind_speed_10m_max&timezone=Australia%%2FSydney&past_days=7&forecast_days=1", city.Lat, city.Lon)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("weather: %s: %v", city.Name, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			continue
		}
		daily, _ := data["daily"].(map[string]interface{})
		if daily == nil {
			continue
		}
		dates, _ := daily["time"].([]interface{})
		maxTemps, _ := daily["temperature_2m_max"].([]interface{})
		minTemps, _ := daily["temperature_2m_min"].([]interface{})
		precip, _ := daily["precipitation_sum"].([]interface{})
		wind, _ := daily["wind_speed_10m_max"].([]interface{})
		for i, d := range dates {
			row := map[string]interface{}{"city": city.Name, "date": d, "latitude": city.Lat, "longitude": city.Lon}
			if i < len(maxTemps) {
				row["temp_max"] = maxTemps[i]
			}
			if i < len(minTemps) {
				row["temp_min"] = minTemps[i]
			}
			if i < len(precip) {
				row["precipitation"] = precip[i]
			}
			if i < len(wind) {
				row["wind_max"] = wind[i]
			}
			rows = append(rows, row)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if len(rows) == 0 {
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "weather.open_meteo", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, "weather", "daily.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("weather: wrote %d rows to %s", len(rows), key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}
