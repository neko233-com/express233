package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPreviewByBasename(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "any", "deep", "path")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "game.properties"), []byte("server.port=1\nserver.id=template\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := BuildPreview(dir, "p", "1.0", "s1", OverridesFromLegacy(map[string]map[string]string{
		"game.properties": {"server.port": "9001", "server.id": "s1"},
	}), "hook.sh", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 1 {
		t.Fatalf("files: %+v", report.Files)
	}
	f := report.Files[0]
	if f.Basename != "game.properties" || len(f.Paths) != 1 {
		t.Fatalf("paths: %+v", f)
	}
	var portChange *KeyChange
	for i := range f.Changes {
		if f.Changes[i].Key == "server.port" {
			portChange = &f.Changes[i]
		}
	}
	if portChange == nil || portChange.Before != "1" || portChange.After != "9001" {
		t.Fatalf("changes: %+v", f.Changes)
	}
}

func TestApplyByBasenameIgnoresPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a", "b", "cfg.properties")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(p, []byte("k=old\n"), 0o644)
	if err := ApplyByBasename(dir, OverridesFromLegacy(map[string]map[string]string{
		"cfg.properties": {"k": "new"},
	})); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "k=new\n" {
		t.Fatalf("got %q", b)
	}
}
