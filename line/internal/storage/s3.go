package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type S3Client struct {
	client *s3.Client
	bucket string
}

func NewS3Client(endpoint, accessKey, secretKey, region, bucket string) *S3Client {
	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		UsePathStyle: true,
	})
	return &S3Client{client: client, bucket: bucket}
}

func (c *S3Client) PutLap(ctx context.Context, sessionID string, lapNum int, data []byte) (string, error) {
	now := time.Now().UTC()
	key := fmt.Sprintf("bronze/telemetry/%s/%04d/%02d/%02d/lap-%03d-%s.parquet",
		sessionID, now.Year(), now.Month(), now.Day(), lapNum, uuid.NewString()[:8])

	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put %s: %w", key, err)
	}
	return key, nil
}

func (c *S3Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get %s: %w", key, err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("s3 read body: %w", err)
	}
	return buf.Bytes(), nil
}

func (c *S3Client) GetLatestByPrefix(ctx context.Context, prefix string) ([]byte, error) {
	list, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 list %s: %w", prefix, err)
	}
	if len(list.Contents) == 0 {
		return nil, fmt.Errorf("no objects found with prefix %s", prefix)
	}
	var latest string
	for _, obj := range list.Contents {
		if *obj.Key > latest {
			latest = *obj.Key
		}
	}
	return c.GetObject(ctx, latest)
}

func (c *S3Client) GetLap(ctx context.Context, sessionID string, lapNum int) ([]byte, error) {
	prefix := fmt.Sprintf("bronze/telemetry/%s/", sessionID)
	target := fmt.Sprintf("lap-%03d-", lapNum)

	list, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 list %s: %w", prefix, err)
	}

	var key string
	for _, obj := range list.Contents {
		if strings.Contains(*obj.Key, target) {
			key = *obj.Key
			break
		}
	}
	if key == "" {
		return nil, fmt.Errorf("lap %d not found in session %s", lapNum, sessionID)
	}

	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get %s: %w", key, err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("s3 read body: %w", err)
	}
	return buf.Bytes(), nil
}
