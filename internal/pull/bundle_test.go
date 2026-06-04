package pull

import (
	"bytes"
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
