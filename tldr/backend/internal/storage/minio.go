package storage

import (
	"context"
	"io"
	"log"
	"strings"
	"tldr/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	minio  *minio.Client
	bucket string
}

func NewMinIOClient(cfg config.MinIOConfig) *Client {
	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	return &Client{minio: minioClient, bucket: cfg.Bucket}
}

func (c *Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := c.minio.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	return obj, err
}

func (c *Client) ListNewsSummaries(ctx context.Context) ([]string, error) {
	var summaries []string

	opts := minio.ListObjectsOptions{
		Prefix:    "news/",
		Recursive: true,
	}

	for object := range c.minio.ListObjects(ctx, c.bucket, opts) {
		if object.Err != nil {
			return nil, object.Err
		}

		// Extract the filename like "2025-05-07.md"
		parts := strings.Split(object.Key, "/")
		if len(parts) > 1 {
			summaries = append(summaries, parts[len(parts)-1])
		}
	}

	return summaries, nil
}
