package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlobDedupAcrossVersions(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, err := st.CreateProject(tid, st.TestRootUserID(), "dedup-game")
	if err != nil {
		t.Fatal(err)
	}
	v1, err := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := st.CreateVersion(tid, p.ID, p.Name, "1.0.1")
	if err != nil {
		t.Fatal(err)
	}

	content := strings.NewReader("shared=payload\n")
	if err := st.WriteVersionFile(tid, p.Name, v1.Version, "cfg/game.properties", content); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(tid, p.Name, v2.Version, "cfg/game.properties", strings.NewReader("shared=payload\n")); err != nil {
		t.Fatal(err)
	}

	stats, err := st.BlobStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.BlobCount != 1 {
		t.Fatalf("expected 1 blob, got %d", stats.BlobCount)
	}
	if stats.TotalRefs != 2 {
		t.Fatalf("expected ref_count sum 2, got %d", stats.TotalRefs)
	}

	if err := st.DeleteVersion(tid, p.ID, p.Name, v1.Version); err != nil {
		t.Fatal(err)
	}
	stats, err = st.BlobStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.BlobCount != 1 || stats.TotalRefs != 1 {
		t.Fatalf("after delete v1: %+v", stats)
	}

	if err := st.DeleteVersion(tid, p.ID, p.Name, v2.Version); err != nil {
		t.Fatal(err)
	}
	stats, err = st.BlobStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.BlobCount != 0 {
		t.Fatalf("expected blobs gc'd, got %+v", stats)
	}
	if _, err := os.Stat(filepath.Join(dir, "blobs")); err == nil {
		entries, _ := os.ReadDir(filepath.Join(dir, "blobs"))
		for _, e := range entries {
			if e.Name() == "." || e.Name() == ".." {
				continue
			}
			sub, _ := os.ReadDir(filepath.Join(dir, "blobs", e.Name()))
			if len(sub) > 0 {
				t.Fatalf("blob files remain under blobs/: %v", sub)
			}
		}
	}
}

func TestBlobDeleteFileDecrementsRef(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, _ := st.CreateProject(tid, st.TestRootUserID(), "gc-game")
	v, _ := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	_ = st.WriteVersionFile(tid, p.Name, v.Version, "a.txt", strings.NewReader("one"))
	_ = st.WriteVersionFile(tid, p.Name, v.Version, "b.txt", strings.NewReader("one"))

	stats, _ := st.BlobStats()
	if stats.BlobCount != 1 || stats.TotalRefs != 2 {
		t.Fatalf("setup: %+v", stats)
	}

	if err := st.DeleteVersionFile(tid, p.Name, v.Version, "a.txt"); err != nil {
		t.Fatal(err)
	}
	stats, _ = st.BlobStats()
	if stats.TotalRefs != 1 {
		t.Fatalf("after delete file: %+v", stats)
	}
}

func TestBlobMigrationAdoptsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	userdata := filepath.Join(dir, "userdata", "default", "projects", "legacy", "1.0.0")
	if err := os.MkdirAll(filepath.Join(userdata, "cfg"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("legacy=1\n")
	if err := os.WriteFile(filepath.Join(userdata, "cfg", "game.properties"), payload, 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	stats, err := st.BlobStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.BlobCount != 1 || stats.TotalRefs != 1 {
		t.Fatalf("migration stats: %+v", stats)
	}

	linked, err := st.isLinkedToBlob(filepath.Join(userdata, "cfg", "game.properties"))
	if err != nil || !linked {
		t.Fatalf("expected migrated file linked to blob: linked=%v err=%v", linked, err)
	}
}
