package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/parquet-go/parquet-go"

	"github.com/solanyn/mono/line/data"
)

type carEntry struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Maker   string `json:"maker"`
	Country string `json:"country"`
	Group   string `json:"group,omitempty"`
}

func (s *server) loadCars(ctx context.Context) {
	if s.s3 == nil {
		slog.Warn("no S3 client configured, cars unavailable")
		return
	}
	carsData, err := s.s3.GetLatestByPrefix(ctx, "reference/cars/")
	if err != nil {
		slog.Warn("failed to load cars from S3", "err", err)
		return
	}
	rows, err := readCarsParquet(carsData)
	if err != nil {
		slog.Warn("failed to parse cars parquet", "err", err)
		return
	}
	s.cars = rows
	slog.Info("loaded cars from S3", "count", len(rows))
}

func readCarsParquet(buf []byte) ([]carEntry, error) {
	type parquetCar struct {
		ID      int32  `parquet:"id"`
		Name    string `parquet:"name"`
		Maker   string `parquet:"maker"`
		Country string `parquet:"country"`
		Group   string `parquet:"group"`
	}
	reader := parquet.NewGenericReader[parquetCar](bytes.NewReader(buf))
	defer reader.Close()

	rows := make([]parquetCar, reader.NumRows())
	n, err := reader.Read(rows)
	if err != nil && err != io.EOF {
		return nil, err
	}
	cars := make([]carEntry, n)
	for i := 0; i < n; i++ {
		cars[i] = carEntry{
			ID:      int(rows[i].ID),
			Name:    rows[i].Name,
			Maker:   rows[i].Maker,
			Country: rows[i].Country,
			Group:   rows[i].Group,
		}
	}
	return cars, nil
}

func (s *server) handleListCars(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.cars)
}

func (s *server) handleGetCar(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid car id", http.StatusBadRequest)
		return
	}
	for _, c := range s.cars {
		if c.ID == id {
			writeJSON(w, c)
			return
		}
	}
	http.Error(w, "car not found", http.StatusNotFound)
}

func (s *server) handleListTracks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(data.TracksJSON)
}
