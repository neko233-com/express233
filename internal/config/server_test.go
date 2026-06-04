package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServerFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yaml")
	content := `servers:
  a:
    replacements:
      f.properties:
        k: v
    post_hook: h.sh
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sf, err := LoadServerFile(path)
	if err != nil {
		t.Fatal(err)
	}
	e := sf.Entry("a")
	if e == nil || e.PostHook != "h.sh" {
		t.Fatalf("entry: %+v", e)
	}
	if e.Replacements["f.properties"] == nil || e.Replacements["f.properties"]["k"] != "v" {
		t.Fatalf("replacements missing: %+v", e.Replacements["f.properties"])
	}
}
