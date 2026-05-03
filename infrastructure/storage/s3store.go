package storage

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Store stores blobs in an S3-compatible object store (Garage, MinIO, AWS S3, etc.).
type S3Store struct {
	id     string
	client *minio.Client
	bucket string
}

// S3Config holds connection parameters for one S3 endpoint.
type S3Config struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	UseSSL    bool   `json:"use_ssl"`
	Region    string `json:"region"`
}

func NewS3Store(id string, cfg S3Config) (*S3Store, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 store %q: %w", id, err)
	}
	return &S3Store{id: id, client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Store) ID() string   { return s.id }
func (s *S3Store) Type() string { return "s3" }

func (s *S3Store) Put(ctx context.Context, key string, r io.Reader, size int64, meta map[string]string) error {
	opts := minio.PutObjectOptions{UserMetadata: meta}
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, opts)
	return err
}

func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, err
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, err
	}
	return obj, info.Size, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// EnsureBucket creates the bucket if it doesn't exist.
func (s *S3Store) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	}
	return nil
}

// WriteURL uploads content keyed by URL, returns (key, size, contentHashHex, error).
func (s *S3Store) WriteURL(ctx context.Context, rawURL, ext string, r io.Reader, size int64, meta map[string]string) (key string, writtenSize int64, hexHash string, err error) {
	key = keyFromURL(rawURL, ext)

	hasher := md5.New()
	pr, pw := io.Pipe()

	go func() {
		_, werr := io.Copy(io.MultiWriter(pw, hasher), r)
		pw.CloseWithError(werr)
	}()

	info, err := s.client.PutObject(ctx, s.bucket, key, pr, size, minio.PutObjectOptions{
		UserMetadata: meta,
	})
	if err != nil {
		return
	}
	writtenSize = info.Size
	hexHash = fmt.Sprintf("%x", hasher.Sum(nil))
	return
}
