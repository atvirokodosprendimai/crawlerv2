package storage

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStore stores blobs on the local filesystem under a base directory.
// Key structure mirrors keyFromURL: base/{hex[0]}/{hex[1]}/{hex[2]}/{hex}.ext
type LocalStore struct {
	id   string
	base string
}

func NewLocalStore(id, base string) *LocalStore {
	return &LocalStore{id: id, base: base}
}

func (s *LocalStore) ID() string   { return s.id }
func (s *LocalStore) Type() string { return "local" }

func (s *LocalStore) Put(_ context.Context, key string, r io.Reader, _ int64, _ map[string]string) error {
	full := filepath.Join(s.base, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *LocalStore) Get(_ context.Context, key string) (io.ReadCloser, int64, error) {
	full := filepath.Join(s.base, filepath.FromSlash(key))
	f, err := os.Open(full)
	if err != nil {
		return nil, 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, fi.Size(), nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	return os.Remove(filepath.Join(s.base, filepath.FromSlash(key)))
}

func (s *LocalStore) Exists(_ context.Context, key string) (bool, error) {
	_, err := os.Stat(filepath.Join(s.base, filepath.FromSlash(key)))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// WriteURL is a convenience helper used by the worker: writes content keyed by URL,
// returns (key, size, contentHashHex, error).
func (s *LocalStore) WriteURL(ctx context.Context, rawURL, ext string, r io.Reader) (key string, size int64, hexHash string, err error) {
	key = keyFromURL(rawURL, ext)
	hasher := md5.New()
	pr, pw := io.Pipe()

	// tee into hasher while streaming to store
	go func() {
		_, werr := io.Copy(io.MultiWriter(pw, hasher), r)
		pw.CloseWithError(werr)
	}()

	full := filepath.Join(s.base, filepath.FromSlash(key))
	if err = os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		pr.CloseWithError(err)
		return
	}
	f, err := os.Create(full)
	if err != nil {
		pr.CloseWithError(err)
		return
	}
	defer f.Close()

	size, err = io.Copy(f, pr)
	if err != nil {
		return
	}
	hexHash = fmt.Sprintf("%x", hasher.Sum(nil))
	return
}
