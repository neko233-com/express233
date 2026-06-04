package compare_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/express233/internal/compare"
	"github.com/neko233-com/express233/internal/config"
)

func TestDiffVersions_modifiedKey(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from")
	to := filepath.Join(dir, "to")
	for _, root := range []string{from, to} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(from, "game.properties"), []byte("port=8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(to, "game.properties"), []byte("port=8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := &config.ServerEntry{
		Replacements: map[string]config.FileOverrides{
			"game.properties": {"port": "9000"},
		},
	}
	report, err := compare.DiffVersions(from, to, "p", "1.0", "2.0", "node-a", entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 0 {
		t.Fatalf("same effective config should have no diff, got %+v", report.Files)
	}
	entry.Replacements["game.properties"]["port"] = "9001"
	report, err = compare.DiffVersions(from, to, "p", "1.0", "2.0", "node-a", entry)
	if err != nil {
		t.Fatal(err)
	}
	// same replacement on both roots -> still no diff between versions
	if len(report.Files) != 0 {
		t.Fatalf("expected no diff when both versions identical, got %+v", report.Files)
	}
}
