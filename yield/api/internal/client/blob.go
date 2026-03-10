package client

import (
	"context"
)

type BlobStore interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}
