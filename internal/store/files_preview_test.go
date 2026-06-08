package store

import (
	"strings"
	"testing"
)

func TestReadVersionTextFile(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, err := st.CreateProject(tid, st.TestRootUserID(), "preview-game")
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(tid, p.Name, v.Version, "config/app.yaml", strings.NewReader("server:\n  id: s1\n")); err != nil {
		t.Fatal(err)
	}

	data, size, err := st.ReadVersionTextFile(tid, p.Name, v.Version, "config/app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(data)) || !strings.Contains(string(data), "id: s1") {
		t.Fatalf("unexpected preview: size=%d data=%q", size, data)
	}
}

func TestReadVersionTextFileRejectsUnsafeAndBinary(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, _ := st.CreateProject(tid, st.TestRootUserID(), "preview-guard")
	v, _ := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	_ = st.WriteVersionFile(tid, p.Name, v.Version, "bin.dat", strings.NewReader("a\x00b"))

	if _, _, err := st.ReadVersionTextFile(tid, p.Name, v.Version, "../server.yaml"); err == nil {
		t.Fatal("want unsafe path error")
	}
	if _, _, err := st.ReadVersionTextFile(tid, p.Name, v.Version, "bin.dat"); err == nil {
		t.Fatal("want binary preview error")
	}
}

func TestReadVersionTextFileRejectsLargeFile(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, _ := st.CreateProject(tid, st.TestRootUserID(), "preview-large")
	v, _ := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	large := strings.Repeat("x", MaxPreviewFileBytes+1)
	_ = st.WriteVersionFile(tid, p.Name, v.Version, "large.txt", strings.NewReader(large))

	_, size, err := st.ReadVersionTextFile(tid, p.Name, v.Version, "large.txt")
	if err == nil {
		t.Fatal("want large file error")
	}
	if size != int64(len(large)) {
		t.Fatalf("size = %d, want %d", size, len(large))
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}
