package store

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// BlobStorage persists immutable artifact contents by lowercase SHA-256.
// Put must be atomic and idempotent for the same digest. Delete must succeed
// when the object is already absent. The store owns reference counts and the
// local version projection used to build and validate deployment bundles.
type BlobStorage interface {
	Name() string
	Put(ctx context.Context, digest string, size int64, data io.Reader) error
	Open(ctx context.Context, digest string) (io.ReadCloser, error)
	Delete(ctx context.Context, digest string) error
}

type localBlobStorage struct {
	root string
}

func (b localBlobStorage) Name() string { return "local" }

func (b localBlobStorage) path(digest string) string {
	return filepath.Join(b.root, digest[:2], digest)
}

func (b localBlobStorage) Put(_ context.Context, digest string, _ int64, data io.Reader) error {
	path := b.path(digest)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeArtifactAtomically(path, data)
}

func (b localBlobStorage) Open(_ context.Context, digest string) (io.ReadCloser, error) {
	f, err := os.Open(b.path(digest))
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (b localBlobStorage) Delete(_ context.Context, digest string) error {
	err := os.Remove(b.path(digest))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func writeArtifactAtomically(path string, data io.Reader) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".artifact-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, copyErr := io.Copy(f, data)
	if err := errors.Join(copyErr, f.Close()); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
