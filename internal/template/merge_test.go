package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeYAMLNestedPartial(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "application.yaml")
	content := `mysql:
  host: db.internal
  port: 3306
  password: old
redis:
  host: redis.internal
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	override := map[string]any{
		"mysql": map[string]any{
			"password": "secret",
			"url":      "jdbc:mysql://db.internal:3306/game",
		},
	}
	if err := ApplyByBasename(dir, map[string]map[string]any{
		"application.yaml": override,
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	s := string(got)
	if !strings.Contains(s, "password: secret") || !strings.Contains(s, "host: db.internal") {
		t.Fatalf("partial merge failed:\n%s", s)
	}
	if strings.Contains(s, "password: old") {
		t.Fatalf("old password should be replaced:\n%s", s)
	}
}
