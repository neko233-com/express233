package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

type fixtureBlobStorage struct {
	data        map[string][]byte
	putError    error
	openError   error
	deleteError error
}

func (b *fixtureBlobStorage) Name() string { return "fixture" }
func (b *fixtureBlobStorage) Put(_ context.Context, hash string, size int64, data io.Reader) error {
	if b.putError != nil {
		return b.putError
	}
	if _, ok := b.data[hash]; ok {
		return nil
	}
	body, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	if int64(len(body)) != size {
		return fmt.Errorf("invalid artifact size")
	}
	b.data[hash] = body
	return nil
}
func (b *fixtureBlobStorage) Open(_ context.Context, hash string) (io.ReadCloser, error) {
	if b.openError != nil {
		return nil, b.openError
	}
	data, ok := b.data[hash]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (b *fixtureBlobStorage) Delete(_ context.Context, hash string) error {
	if b.deleteError != nil {
		return b.deleteError
	}
	delete(b.data, hash)
	return nil
}

func TestArtifactStorageStrategy(t *testing.T) {
	dir := t.TempDir()
	backend := &fixtureBlobStorage{data: make(map[string][]byte)}
	s, err := OpenWithBlobStorage(dir, backend)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	const tenant int64 = 1
	project, err := s.CreateProject(tenant, s.TestRootUserID(), "strategy")
	if err != nil {
		t.Fatal(err)
	}
	version, err := s.CreateVersion(tenant, project.ID, project.Name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"first.txt", "second.txt"} {
		if err := s.WriteVersionFile(tenant, project.Name, version.Version, path, strings.NewReader("shared")); err != nil {
			t.Fatal(err)
		}
	}
	if len(backend.data) != 1 {
		t.Fatalf("expected one backend object, got %d", len(backend.data))
	}
	stats, err := s.BlobStats()
	if err != nil || stats.TotalRefs != 2 {
		t.Fatalf("unexpected references: %+v %v", stats, err)
	}

	backend.openError = errors.New("read unavailable")
	if err := s.WriteVersionFile(tenant, project.Name, version.Version, "first.txt", strings.NewReader("replacement")); err == nil {
		t.Fatal("materialization should fail")
	}
	backend.openError = nil
	current, _, err := s.ReadVersionTextFile(tenant, project.Name, version.Version, "first.txt")
	if err != nil || string(current) != "shared" || len(backend.data) != 1 {
		t.Fatalf("failed replacement changed committed data: %q %v", current, err)
	}

	if err := s.DeleteVersionFile(tenant, project.Name, version.Version, "first.txt"); err != nil {
		t.Fatal(err)
	}
	backend.deleteError = errors.New("delete unavailable")
	if err := s.DeleteVersionFile(tenant, project.Name, version.Version, "second.txt"); err != nil {
		t.Fatal(err)
	}
	backend.deleteError = nil
	tree, err := s.StorageTreeAt(tenant, "blobs")
	if err != nil || len(tree.Children) != 1 || len(tree.Children[0].Children) != 1 {
		t.Fatalf("missing retryable orphan: %+v %v", tree, err)
	}
	orphan := tree.Children[0].Children[0].Path
	plan, err := s.PlanStorageDelete(tenant, orphan, RoleAdmin)
	if err != nil || !plan.Allowed {
		t.Fatalf("cannot retry orphan cleanup: %+v %v", plan, err)
	}
	if err := s.ExecuteStorageDelete(tenant, orphan); err != nil || len(backend.data) != 0 {
		t.Fatalf("orphan cleanup failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if local, err := Open(dir); err == nil {
		_ = local.Close()
		t.Fatal("existing metadata must reject implicit storage changes")
	}
}
