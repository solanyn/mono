package storage

import (
	"context"
	"io"
	"log"
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
