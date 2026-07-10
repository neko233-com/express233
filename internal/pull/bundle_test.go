package pull

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/store"
)

func TestBuildAndExtractBundle(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	sf := &config.ServerFile{
		Servers: map[string]config.ServerEntry{
			"s1": {
				Replacements: map[string]config.FileOverrides{
					"cfg.properties": {"k": "v2"},
				},
				PostHook:    "hook.sh",
				PostHookEnv: map[string]string{"X": "1"},
			},
		},
	}
	yamlPath, _ := st.ServerYAMLPath(tid)
	_ = os.WriteFile(yamlPath, []byte("servers:\n  s1:\n    replacements:\n      cfg.properties:\n        k: v2\n    post_hook: hook.sh\n"), 0o644)

	p, _ := st.CreateProject(tid, st.TestRootUserID(), "p1")
	v, _ := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	_ = st.WriteVersionFile(tid, p.Name, v.Version, "nested/cfg.properties", bytes.NewBufferString("k=old\n"))
	_ = st.PublishVersion(tid, p.ID, v.Version)

	var buf bytes.Buffer
	if err := BuildBundle(st, tid, sf, p.Name, v.Version, "s1", &buf); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	m, err := ExtractBundle(&buf, dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.ServerID != "s1" || m.PostHook != "hook.sh" {
		t.Fatalf("manifest: %+v", m)
	}
	b, err := os.ReadFile(filepath.Join(dest, "nested", "cfg.properties"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "k=v2\n" {
		t.Fatalf("replacement failed: %q", string(b))
	}
}

func TestBuildBundlePreservesUploadedExecutableModes(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, _ := st.CreateProject(tid, st.TestRootUserID(), "mode-preservation")
	v, _ := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	var uploaded bytes.Buffer
	gz := gzip.NewWriter(&uploaded)
	tw := tar.NewWriter(gz)
	for _, file := range []struct {
		name string
		mode int64
		body string
	}{
		{name: "scripts/restart.sh", mode: 0o755, body: "#!/bin/sh\nexit 0\n"},
		{name: "scripts/restart.ps1", mode: 0o644, body: "exit 0\n"},
		{name: "docs/restart-copy.txt", mode: 0o644, body: "#!/bin/sh\nexit 0\n"},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: file.name, Mode: file.mode, Size: int64(len(file.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, file.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := st.ExtractTarToVersion(tid, p.Name, v.Version, bytes.NewReader(uploaded.Bytes()), true); err != nil {
		t.Fatal(err)
	}

	sf := &config.ServerFile{Servers: map[string]config.ServerEntry{"111": {}}}
	var bundle bytes.Buffer
	if err := BuildBundle(st, tid, sf, p.Name, v.Version, "111", &bundle); err != nil {
		t.Fatal(err)
	}
	gr, err := gzip.NewReader(bytes.NewReader(bundle.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	modes := map[string]int64{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		modes[hdr.Name] = hdr.Mode
	}
	if got := modes["scripts/restart.sh"]; got != 0o755 {
		t.Fatalf("restart.sh mode=%#o, want 0755", got)
	}
	if got := modes["scripts/restart.ps1"]; got != 0o644 {
		t.Fatalf("restart.ps1 mode=%#o, want 0644", got)
	}
	if got := modes["docs/restart-copy.txt"]; got != 0o644 {
		t.Fatalf("same-content non-executable mode=%#o, want 0644", got)
	}
}
