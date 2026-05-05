package main

import (
	"bytes"
	"testing"

	parquet "github.com/parquet-go/parquet-go"
)

func TestWriteParquet(t *testing.T) {
	rows := GenerateSampleLap("test-session", 1)

	data, err := WriteParquet(rows)
	if err != nil {
		t.Fatalf("WriteParquet: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("empty parquet output")
	}

	t.Logf("3600 frames → %d bytes parquet (%.1f bytes/frame)", len(data), float64(len(data))/3600)

	reader := parquet.NewGenericReader[TelemetryRow](bytes.NewReader(data))
	defer reader.Close()

	if reader.NumRows() != 3600 {
		t.Fatalf("expected 3600 rows, got %d", reader.NumRows())
	}

	readBack := make([]TelemetryRow, 10)
	n, err := reader.Read(readBack)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n != 10 {
		t.Fatalf("expected 10 rows, got %d", n)
	}

	if readBack[0].SessionID != "test-session" {
		t.Fatalf("expected test-session, got %s", readBack[0].SessionID)
	}
	if readBack[0].Lap != 1 {
		t.Fatalf("expected lap 1, got %d", readBack[0].Lap)
	}
	if readBack[0].Speed == 0 {
		t.Fatal("expected non-zero speed")
	}
}

func TestParquetSize(t *testing.T) {
	rows := GenerateSampleLap("compress-test", 1)
	data, err := WriteParquet(rows)
	if err != nil {
		t.Fatalf("WriteParquet: %v", err)
	}

	bytesPerFrame := float64(len(data)) / 3600.0
	t.Logf("parquet=%d bytes, %.1f bytes/frame (3600 frames, 18 columns)", len(data), bytesPerFrame)

	if bytesPerFrame > 150 {
		t.Fatalf("parquet too large: %.1f bytes/frame (expected <150)", bytesPerFrame)
	}
}

func TestGenerateSampleLap(t *testing.T) {
	rows := GenerateSampleLap("gen-test", 5)

	if len(rows) != 3600 {
		t.Fatalf("expected 3600 rows, got %d", len(rows))
	}

	first := rows[0]
	last := rows[3599]

	if first.SessionID != "gen-test" {
		t.Fatalf("expected gen-test, got %s", first.SessionID)
	}
	if first.Lap != 5 {
		t.Fatalf("expected lap 5, got %d", first.Lap)
	}
	if first.FrameIndex != 0 {
		t.Fatalf("expected frame 0, got %d", first.FrameIndex)
	}
	if last.FrameIndex != 3599 {
		t.Fatalf("expected frame 3599, got %d", last.FrameIndex)
	}
	if first.FuelLevel <= last.FuelLevel {
		t.Fatal("fuel should decrease over lap")
	}
}
