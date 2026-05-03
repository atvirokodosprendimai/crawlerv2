package storage

import (
	"context"
	"io"
)

// BlobStore is the unified interface for storing and retrieving crawled files.
// Both local filesystem and S3-compatible stores implement this.
type BlobStore interface {
	// Put writes content to the store under key. meta is stored alongside the blob.
	Put(ctx context.Context, key string, r io.Reader, size int64, meta map[string]string) error

	// Get retrieves a blob by key. Caller must close the returned reader.
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)

	// Delete removes a blob.
	Delete(ctx context.Context, key string) error

	// Exists reports whether a key is present.
	Exists(ctx context.Context, key string) (bool, error)

	// ID returns the store's configured identifier.
	ID() string

	// Type returns "local" or "s3".
	Type() string
}

// KeyFor returns the sharded key for a rawURL with the given extension.
// Pattern: {md5[0]}/{md5[1]}/{md5[2]}/{md5}.{ext}
// This mirrors the FileStore path logic so keys are consistent across store types.
func KeyFor(rawURL, ext string) string {
	return keyFromURL(rawURL, ext)
}
