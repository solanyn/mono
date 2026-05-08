package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	parquet "github.com/parquet-go/parquet-go"
)

type TelemetryRow struct {
	SessionID   string  `parquet:"session_id"`
	Lap         int32   `parquet:"lap"`
	FrameIndex  int32   `parquet:"frame_index"`
	TimestampNs int64   `parquet:"timestamp_ns"`
	X           float32 `parquet:"x"`
	Y           float32 `parquet:"y"`
	Z           float32 `parquet:"z"`
	Speed       float32 `parquet:"speed"`
	Throttle    float32 `parquet:"throttle"`
	Brake       float32 `parquet:"brake"`
	Steering    float32 `parquet:"steering"`
	RPM         int32   `parquet:"rpm"`
	Gear        int32   `parquet:"gear"`
	TireTempFL  float32 `parquet:"tire_temp_fl"`
	TireTempFR  float32 `parquet:"tire_temp_fr"`
	TireTempRL  float32 `parquet:"tire_temp_rl"`
	TireTempRR  float32 `parquet:"tire_temp_rr"`
	FuelLevel   float32 `parquet:"fuel_level"`
}

func WriteParquet(rows []TelemetryRow) ([]byte, error) {
	var buf bytes.Buffer
	w := parquet.NewGenericWriter[TelemetryRow](&buf, parquet.Compression(&parquet.Snappy))
	if _, err := w.Write(rows); err != nil {
		return nil, fmt.Errorf("parquet write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("parquet close: %w", err)
	}
	return buf.Bytes(), nil
}

func UploadToS3(ctx context.Context, client *s3.Client, bucket, sessionID string, lap int32, data []byte) (string, error) {
	now := time.Now().UTC()
	key := fmt.Sprintf("bronze/telemetry/%s/%04d/%02d/%02d/lap-%03d-%s.parquet",
		sessionID, now.Year(), now.Month(), now.Day(), lap, uuid.NewString()[:8])

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put %s/%s: %w", bucket, key, err)
	}
	return key, nil
}

func NewS3Client(endpoint, accessKey, secretKey, region string) *s3.Client {
	return s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		UsePathStyle: true,
	})
}

func GenerateSampleLap(sessionID string, lap int32) []TelemetryRow {
	numFrames := 3600
	rows := make([]TelemetryRow, numFrames)
	for i := 0; i < numFrames; i++ {
		t := float64(i) / float64(numFrames) * math.Pi * 2
		r := 200 + 60*math.Sin(t*2)
		rows[i] = TelemetryRow{
			SessionID:   sessionID,
			Lap:         lap,
			FrameIndex:  int32(i),
			TimestampNs: int64(i) * 16666667,
			X:           float32(math.Cos(t) * r),
			Y:           float32(5 * math.Sin(t*3)),
			Z:           float32(math.Sin(t) * r),
			Speed:       float32(180 + 80*math.Cos(t*3)),
			Throttle:    float32(math.Max(0, math.Cos(t*3))),
			Brake:       float32(math.Max(0, -math.Cos(t*3))),
			Steering:    float32(math.Sin(t*2) * 0.5),
			RPM:         int32(4000 + 3000*math.Cos(t*3)),
			Gear:        int32(3 + 2*math.Cos(t*2)),
			TireTempFL:  float32(85 + 10*math.Sin(t)),
			TireTempFR:  float32(86 + 10*math.Sin(t+0.1)),
			TireTempRL:  float32(82 + 8*math.Sin(t)),
			TireTempRR:  float32(83 + 8*math.Sin(t+0.1)),
			FuelLevel:   float32(100 - float64(i)/float64(numFrames)*5),
		}
	}
	return rows
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	endpoint := envOr("S3_ENDPOINT", "http://s3.goyangi.io")
	accessKey := envOr("S3_ACCESS_KEY", "")
	secretKey := envOr("S3_SECRET_KEY", "")
	region := envOr("S3_REGION", "us-east-1")
	bucket := envOr("S3_BUCKET", "line-bronze")

	if accessKey == "" || secretKey == "" {
		log.Fatal("S3_ACCESS_KEY and S3_SECRET_KEY required")
	}

	client := NewS3Client(endpoint, accessKey, secretKey, region)
	sessionID := "sess-" + uuid.NewString()[:8]

	for lap := int32(1); lap <= 3; lap++ {
		rows := GenerateSampleLap(sessionID, lap)
		data, err := WriteParquet(rows)
		if err != nil {
			log.Fatalf("parquet: %v", err)
		}
		log.Printf("lap %d: %d rows, %d bytes parquet", lap, len(rows), len(data))

		key, err := UploadToS3(context.Background(), client, bucket, sessionID, lap, data)
		if err != nil {
			log.Fatalf("upload: %v", err)
		}
		log.Printf("uploaded: s3://%s/%s", bucket, key)
	}
}
