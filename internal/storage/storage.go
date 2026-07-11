package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrNotFound = errors.New("not found")

// ImageStore persists processed image bytes and returns a public URL path.
type ImageStore interface {
	Save(ctx context.Context, username, name string, data []byte, contentType string) (publicURL string, err error)
}

// S3Config configures MinIO or any S3-compatible object store.
type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	PublicURL string // optional CDN/base URL prefix, e.g. https://cdn.waldi.blog
}

// S3Store uploads images and optionally serves them through /media/.
type S3Store struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

func NewS3Store(cfg S3Config) (*S3Store, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("S3 endpoint is required")
	}
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating S3 client: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("checking bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("creating bucket: %w", err)
		}
	}

	return &S3Store{
		client:    client,
		bucket:    bucket,
		publicURL: strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/"),
	}, nil
}

func (s *S3Store) Save(ctx context.Context, username, name string, data []byte, contentType string) (string, error) {
	key := objectKey(username, name)
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("uploading object: %w", err)
	}
	return s.publicPath(key), nil
}

func (s *S3Store) publicPath(key string) string {
	if s.publicURL != "" {
		parts := strings.Split(key, "/")
		for i, part := range parts {
			parts[i] = url.PathEscape(part)
		}
		return s.publicURL + "/" + strings.Join(parts, "/")
	}
	return "/media/" + key
}

func (s *S3Store) NeedsProxy() bool {
	return s.publicURL == ""
}

func (s *S3Store) Stream(ctx context.Context, w http.ResponseWriter, key string) error {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("getting object: %w", err)
	}
	defer func() { _ = obj.Close() }()

	info, err := obj.Stat()
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return ErrNotFound
		}
		return fmt.Errorf("stat object: %w", err)
	}

	if info.ContentType != "" {
		w.Header().Set("Content-Type", info.ContentType)
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, obj)
	return err
}

func objectKey(username, name string) string {
	return username + "/" + name
}

// LocalStore saves uploads to the local filesystem (development fallback).
type LocalStore struct {
	RootDir string
}

func (l LocalStore) Save(_ context.Context, username, name string, data []byte, _ string) (string, error) {
	dir := filepath.Join(l.RootDir, username)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating upload dir: %w", err)
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("creating upload file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("writing upload file: %w", err)
	}
	return "/static/uploads/" + username + "/" + name, nil
}
