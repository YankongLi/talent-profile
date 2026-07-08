package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client interface {
	Put(ctx context.Context, input PutObjectInput) (ObjectInfo, error)
	PresignedGetURL(ctx context.Context, key string, expires time.Duration) (*url.URL, error)
	Delete(ctx context.Context, key string) error
}

type PutObjectInput struct {
	Key         string
	Reader      io.Reader
	Size        int64
	ContentType string
	Metadata    map[string]string
}

type ObjectInfo struct {
	Bucket      string
	Key         string
	Size        int64
	ETag        string
	ContentType string
}

type MinIOClient struct {
	client *minio.Client
	bucket string
}

func NewMinIOClient(cfg config.StorageConfig) (*MinIOClient, error) {
	endpoint, useSSL, err := normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	if cfg.Bucket == "" {
		return nil, errors.New("storage bucket is required")
	}
	if cfg.AccessKeyID == "" {
		return nil, errors.New("storage access key id is required")
	}
	if cfg.SecretAccessKey == "" {
		return nil, errors.New("storage secret access key is required")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}

	return &MinIOClient{client: client, bucket: cfg.Bucket}, nil
}

func (c *MinIOClient) Put(ctx context.Context, input PutObjectInput) (ObjectInfo, error) {
	if input.Key == "" {
		return ObjectInfo{}, errors.New("object key is required")
	}
	if input.Reader == nil {
		return ObjectInfo{}, errors.New("object reader is required")
	}
	if input.Size < 0 {
		return ObjectInfo{}, errors.New("object size must not be negative")
	}

	info, err := c.client.PutObject(ctx, c.bucket, input.Key, input.Reader, input.Size, minio.PutObjectOptions{
		ContentType:  input.ContentType,
		UserMetadata: input.Metadata,
	})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("put object %q: %w", input.Key, err)
	}

	return ObjectInfo{
		Bucket:      info.Bucket,
		Key:         info.Key,
		Size:        info.Size,
		ETag:        info.ETag,
		ContentType: input.ContentType,
	}, nil
}

func (c *MinIOClient) PresignedGetURL(ctx context.Context, key string, expires time.Duration) (*url.URL, error) {
	if key == "" {
		return nil, errors.New("object key is required")
	}
	if expires <= 0 {
		return nil, errors.New("signed url expiry must be positive")
	}

	u, err := c.client.PresignedGetObject(ctx, c.bucket, key, expires, nil)
	if err != nil {
		return nil, fmt.Errorf("presign object %q: %w", key, err)
	}
	return u, nil
}

func (c *MinIOClient) Delete(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("object key is required")
	}
	if err := c.client.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}
	return nil
}

func normalizeEndpoint(endpoint string, useSSL bool) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false, errors.New("storage endpoint is required")
	}

	if strings.HasPrefix(endpoint, "http://") {
		return strings.TrimPrefix(endpoint, "http://"), false, nil
	}
	if strings.HasPrefix(endpoint, "https://") {
		return strings.TrimPrefix(endpoint, "https://"), true, nil
	}
	return endpoint, useSSL, nil
}
