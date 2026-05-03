package storage

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
)

// WriteURL writes content from r to store keyed by rawURL+ext.
// contentLength=-1 is allowed for unknown-length streams (S3 uses chunked upload).
// Returns (key, contentHashHex, bytesWritten, error).
func WriteURL(ctx context.Context, store BlobStore, rawURL, ext string, r io.Reader, contentLength int64) (key, hexHash string, written int64, err error) {
	key = keyFromURL(rawURL, ext)

	// Count bytes and hash content while streaming
	hasher := md5.New()
	counter := &countWriter{}
	pr, pw := io.Pipe()

	go func() {
		_, werr := io.Copy(io.MultiWriter(pw, hasher, counter), r)
		pw.CloseWithError(werr)
	}()

	if err = store.Put(ctx, key, pr, contentLength, nil); err != nil {
		return
	}
	written = counter.n
	hexHash = fmt.Sprintf("%x", hasher.Sum(nil))
	return
}

type countWriter struct{ n int64 }

func (c *countWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
