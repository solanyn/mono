package storage

import (
	"testing"
)

func TestWriteParquetRoundtrip(t *testing.T) {
	rows := make([]TelemetryRow, 100)
	for i := range rows {
		rows[i] = TelemetryRow{
			PacketID:    int32(i),
			PosX:        float32(i) * 1.5,
			PosY:        float32(i) * 0.5,
			PosZ:        float32(i) * 2.0,
			Speed:       float32(i) * 3.6,
			RPM:         float32(3000 + i*50),
			Throttle:    int32(i % 256),
			Brake:       int32((100 - i) % 256),
			Gear:        int32(i%6 + 1),
			CurrentLap:  int32(i/20 + 1),
			CarID:       1234,
			TireTempFL:  80.0,
			TireTempFR:  82.0,
			TireTempRL:  78.0,
			TireTempRR:  79.0,
		}
	}

	data, err := WriteParquet(rows)
	if err != nil {
		t.Fatalf("WriteParquet: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("empty parquet output")
	}

	bytesPerRow := float64(len(data)) / float64(len(rows))
	if bytesPerRow > 200 {
		t.Errorf("bytes per row = %.1f, want < 200", bytesPerRow)
	}

	t.Logf("wrote %d rows, %d bytes (%.1f bytes/row)", len(rows), len(data), bytesPerRow)
}
