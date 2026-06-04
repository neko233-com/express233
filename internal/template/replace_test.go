package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplacePropertiesByBasename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "any", "deep", "game.properties")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("server.id=old\nserver.port=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyByBasename(dir, OverridesFromLegacy(map[string]map[string]string{
		"game.properties": {"server.id": "srv-1", "server.port": "9001"},
	})); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	got := string(b)
	if !strings.Contains(got, "server.id=srv-1") || !strings.Contains(got, "server.port=9001") {
		t.Fatalf("got %q", got)
	}
}

func TestReplaceYAMLByBasename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conf", "app.yaml")
	content := "game:\n  serverId: old\n  listenPort: 1\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyByBasename(dir, OverridesFromLegacy(map[string]map[string]string{
		"app.yaml": {"game.serverId": "logic-1", "game.listenPort": "9001"},
	})); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	got := string(b)
	if !strings.Contains(got, "logic-1") || !strings.Contains(got, "9001") {
		t.Fatalf("got %q", got)
	}
}
