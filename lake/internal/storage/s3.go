package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type Client struct {
	s3 *s3.Client
}

func NewClient(endpoint, accessKey, secretKey, region string) *Client {
	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		UsePathStyle: true,
	})
	return &Client{s3: client}
}

func (c *Client) PutParquet(ctx context.Context, bucket, dataset, filename string, data []byte) (string, error) {
	now := time.Now().UTC()
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	unique := fmt.Sprintf("%s-%d-%s%s", base, now.UnixNano(), uuid.NewString()[:8], ext)
	key := fmt.Sprintf("%s/%04d/%02d/%02d/%s", dataset, now.Year(), now.Month(), now.Day(), unique)

	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
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

func (c *Client) PutRaw(ctx context.Context, bucket, key, contentType string, data []byte) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3 put %s/%s: %w", bucket, key, err)
	}
	return nil
}

func (c *Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	resp, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get %s/%s: %w", bucket, key, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("s3 read %s/%s: %w", bucket, key, err)
	}
	return data, nil
}

func (c *Client) Healthy(ctx context.Context, bucket string) bool {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	return err == nil
}
