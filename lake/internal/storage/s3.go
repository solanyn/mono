package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
	Bucket    string
}

type Client struct {
	s3     *s3.Client
	bucket string
}

func NewClient(cfg S3Config) *Client {
	resolver := s3.NewDefaultEndpointResolverV2()
	_ = resolver

	client := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true,
	})

	return &Client{s3: client, bucket: cfg.Bucket}
}

func (c *Client) PutParquet(ctx context.Context, layer, dataset, filename string, data []byte) (string, error) {
	now := time.Now().UTC()
	key := fmt.Sprintf("%s/%s/%04d/%02d/%02d/%s", layer, dataset, now.Year(), now.Month(), now.Day(), filename)

	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
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

func (c *Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	resp, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get %s: %w", key, err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.Bytes(), nil
}

func (c *Client) GetLatest(ctx context.Context, layer, dataset, filename string) ([]byte, error) {
	now := time.Now().UTC()
	for i := 0; i < 7; i++ {
		d := now.AddDate(0, 0, -i)
		key := fmt.Sprintf("%s/%s/%04d/%02d/%02d/%s", layer, dataset, d.Year(), d.Month(), d.Day(), filename)
		data, err := c.GetObject(ctx, key)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("no %s/%s data found in last 7 days", layer, dataset)
}

func (c *Client) Healthy(ctx context.Context) bool {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	})
	return err == nil
}
