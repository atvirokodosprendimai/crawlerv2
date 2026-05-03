package storage

import (
	"context"
	"fmt"
	"io"
	"log"
)

// MigrateOptions controls migration behaviour.
type MigrateOptions struct {
	// DeleteAfter removes blobs from the source after successful copy.
	DeleteAfter bool
	// SkipExisting skips keys that already exist in destination.
	SkipExisting bool
	// DryRun logs actions without writing or deleting anything.
	DryRun bool
	// OnProgress is called after each blob with (key, bytesWritten, err).
	OnProgress func(key string, size int64, err error)
}

// MigrateResult summarises a completed migration.
type MigrateResult struct {
	Copied  int
	Skipped int
	Failed  int
	Bytes   int64
}

// MigrateStore copies blobs from src to dst for the given list of keys.
// keys comes from the database (crawl_results.file_path where store_id = src.ID()).
func MigrateStore(ctx context.Context, src, dst BlobStore, keys []string, opts MigrateOptions) (MigrateResult, error) {
	var res MigrateResult

	for _, key := range keys {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}

		if opts.SkipExisting {
			exists, err := dst.Exists(ctx, key)
			if err != nil {
				log.Printf("migrate: exists check failed for %q: %v", key, err)
			} else if exists {
				res.Skipped++
				if opts.OnProgress != nil {
					opts.OnProgress(key, 0, nil)
				}
				continue
			}
		}

		if opts.DryRun {
			log.Printf("migrate [dry-run]: would copy %q from %s → %s", key, src.ID(), dst.ID())
			res.Copied++
			continue
		}

		r, size, err := src.Get(ctx, key)
		if err != nil {
			res.Failed++
			if opts.OnProgress != nil {
				opts.OnProgress(key, 0, fmt.Errorf("get: %w", err))
			}
			continue
		}

		err = dst.Put(ctx, key, r, size, nil)
		r.Close()
		if err != nil {
			res.Failed++
			if opts.OnProgress != nil {
				opts.OnProgress(key, 0, fmt.Errorf("put: %w", err))
			}
			continue
		}

		res.Copied++
		res.Bytes += size
		if opts.OnProgress != nil {
			opts.OnProgress(key, size, nil)
		}

		if opts.DeleteAfter {
			if delErr := src.Delete(ctx, key); delErr != nil {
				log.Printf("migrate: delete from source failed for %q: %v", key, delErr)
			}
		}
	}

	return res, nil
}

// CopyBlob copies a single blob from src to dst. Used for one-off moves.
func CopyBlob(ctx context.Context, src, dst BlobStore, key string) (int64, error) {
	r, size, err := src.Get(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("get %q from %s: %w", key, src.ID(), err)
	}
	defer r.Close()

	// Stream without buffering the full blob in memory
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(pw, r)
		pw.CloseWithError(err)
		errCh <- err
	}()

	if err := dst.Put(ctx, key, pr, size, nil); err != nil {
		return 0, fmt.Errorf("put %q to %s: %w", key, dst.ID(), err)
	}
	if err := <-errCh; err != nil {
		return 0, err
	}
	return size, nil
}
