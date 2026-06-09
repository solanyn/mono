package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/snappy"
)

const (
	tarballURL = "https://github.com/ddm999/gt7info/archive/refs/heads/master.tar.gz"
)

type CarRow struct {
	ID      int32  `parquet:"id"`
	Name    string `parquet:"name"`
	Maker   string `parquet:"maker"`
	Country string `parquet:"country"`
	Group   string `parquet:"group"`
}

type TrackRow struct {
	ID       int32  `parquet:"id"`
	Name     string `parquet:"name"`
	Category string `parquet:"category"`
}

func main() {
	slog.Info("refdata: starting reference data sync")

	endpoint := envOr("S3_ENDPOINT", "http://localhost:3900")
	accessKey := envOr("S3_ACCESS_KEY", "minioadmin")
	secretKey := envOr("S3_SECRET_KEY", "minioadmin")
	region := envOr("S3_REGION", "us-east-1")
	bucket := envOr("S3_BUCKET", "line-bronze")

	tmpDir, err := os.MkdirTemp("", "refdata-*")
	if err != nil {
		slog.Error("create temp dir", "err", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	slog.Info("downloading tarball", "url", tarballURL)
	repoDir, err := downloadAndExtract(tmpDir, tarballURL)
	if err != nil {
		slog.Error("download tarball failed", "err", err)
		os.Exit(1)
	}

	dbDir := filepath.Join(repoDir, "_data", "db")

	makers, err := parseMakers(filepath.Join(dbDir, "maker.csv"))
	if err != nil {
		slog.Error("parse makers", "err", err)
		os.Exit(1)
	}
	slog.Info("parsed makers", "count", len(makers))

	countries, err := parseCountries(filepath.Join(dbDir, "country.csv"))
	if err != nil {
		slog.Error("parse countries", "err", err)
		os.Exit(1)
	}
	slog.Info("parsed countries", "count", len(countries))

	carGroups, err := parseCarGroups(filepath.Join(dbDir, "cargrp.csv"))
	if err != nil {
		slog.Error("parse car groups", "err", err)
		os.Exit(1)
	}
	slog.Info("parsed car groups", "count", len(carGroups))

	cars, err := parseCars(filepath.Join(dbDir, "cars.csv"), makers, countries, carGroups)
	if err != nil {
		slog.Error("parse cars", "err", err)
		os.Exit(1)
	}
	slog.Info("parsed cars", "count", len(cars))

	tracks, err := parseTracks(filepath.Join(dbDir, "course.csv"))
	if err != nil {
		slog.Error("parse tracks", "err", err)
		os.Exit(1)
	}
	slog.Info("parsed tracks", "count", len(tracks))

	ctx := context.Background()
	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		UsePathStyle: true,
	})

	date := time.Now().UTC().Format("2006-01-02")

	carsData, err := writeParquet(cars)
	if err != nil {
		slog.Error("write cars parquet", "err", err)
		os.Exit(1)
	}
	carsKey := fmt.Sprintf("reference/cars/%s.parquet", date)
	if err := putObject(ctx, client, bucket, carsKey, carsData); err != nil {
		slog.Error("upload cars", "err", err)
		os.Exit(1)
	}
	slog.Info("uploaded cars", "key", carsKey, "size", len(carsData), "rows", len(cars))

	tracksData, err := writeParquet(tracks)
	if err != nil {
		slog.Error("write tracks parquet", "err", err)
		os.Exit(1)
	}
	tracksKey := fmt.Sprintf("reference/tracks/%s.parquet", date)
	if err := putObject(ctx, client, bucket, tracksKey, tracksData); err != nil {
		slog.Error("upload tracks", "err", err)
		os.Exit(1)
	}
	slog.Info("uploaded tracks", "key", tracksKey, "size", len(tracksData), "rows", len(tracks))

	slog.Info("refdata: sync complete")
}

type makerInfo struct {
	name      string
	countryID int
}

func parseMakers(path string) (map[int]makerInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	nameIdx, countryIdx := colIndex(header, "Name", "name"), colIndex(header, "Country", "country_id", "country")
	idIdx := colIndex(header, "ID", "id")

	makers := make(map[int]makerInfo)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		id, _ := strconv.Atoi(record[idIdx])
		cid, _ := strconv.Atoi(record[countryIdx])
		makers[id] = makerInfo{name: record[nameIdx], countryID: cid}
	}
	return makers, nil
}

func parseCountries(path string) (map[int]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	idIdx := colIndex(header, "ID", "id")
	codeIdx := colIndex(header, "Code", "code")

	countries := make(map[int]string)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		id, _ := strconv.Atoi(record[idIdx])
		countries[id] = record[codeIdx]
	}
	return countries, nil
}

func parseCarGroups(path string) (map[int]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	idIdx := colIndex(header, "ID", "id")
	groupIdx := colIndex(header, "Group", "group")

	groups := make(map[int]string)
	groupNames := map[string]string{
		"1": "Gr.1", "2": "Gr.2", "3": "Gr.3", "4": "Gr.4",
		"B": "Gr.B", "N": "N", "X": "X",
	}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		id, _ := strconv.Atoi(record[idIdx])
		raw := record[groupIdx]
		if name, ok := groupNames[raw]; ok {
			groups[id] = name
		} else {
			groups[id] = raw
		}
	}
	return groups, nil
}

func parseCars(path string, makers map[int]makerInfo, countries map[int]string, groups map[int]string) ([]CarRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	idIdx := colIndex(header, "ID", "id")
	nameIdx := colIndex(header, "ShortName", "name")
	makerIdx := colIndex(header, "Maker", "maker")

	var cars []CarRow
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		id, _ := strconv.Atoi(record[idIdx])
		makerID, _ := strconv.Atoi(record[makerIdx])
		maker := makers[makerID]
		country := countries[maker.countryID]
		if country == "" {
			country = "xx"
		}
		cars = append(cars, CarRow{
			ID:      int32(id),
			Name:    record[nameIdx],
			Maker:   maker.name,
			Country: country,
			Group:   groups[id],
		})
	}
	return cars, nil
}

func parseTracks(path string) ([]TrackRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	idIdx := colIndex(header, "ID", "id")
	nameIdx := colIndex(header, "Name", "name")
	catIdx := colIndex(header, "Category", "category")

	var tracks []TrackRow
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		id, _ := strconv.Atoi(record[idIdx])
		cat := ""
		if catIdx >= 0 && catIdx < len(record) {
			cat = record[catIdx]
		}
		tracks = append(tracks, TrackRow{
			ID:       int32(id),
			Name:     record[nameIdx],
			Category: cat,
		})
	}
	return tracks, nil
}

func writeParquet[T any](rows []T) ([]byte, error) {
	var buf bytes.Buffer
	w := parquet.NewGenericWriter[T](&buf, parquet.Compression(&snappy.Codec{}))
	if _, err := w.Write(rows); err != nil {
		return nil, fmt.Errorf("parquet write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("parquet close: %w", err)
	}
	return buf.Bytes(), nil
}

func putObject(ctx context.Context, client *s3.Client, bucket, key string, data []byte) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/octet-stream"),
	})
	return err
}

func colIndex(header []string, names ...string) int {
	for i, h := range header {
		for _, n := range names {
			if h == n {
				return i
			}
		}
	}
	return -1
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func downloadAndExtract(destDir, url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	var topDir string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar next: %w", err)
		}

		if topDir == "" {
			topDir = strings.SplitN(hdr.Name, "/", 2)[0]
		}

		target := filepath.Join(destDir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			f, err := os.Create(target)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(f, io.LimitReader(tr, 50<<20)); err != nil {
				f.Close()
				return "", err
			}
			f.Close()
		}
	}

	if topDir == "" {
		return "", fmt.Errorf("empty archive")
	}
	return filepath.Join(destDir, topDir), nil
}
