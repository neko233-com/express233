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

func TestMergeYAMLAcceptsNamedStringMapTrees(t *testing.T) {
	type namedMap map[string]any
	data := []byte(`database:
  mysql:
    host: db.internal
    password: old
    pool:
      max_open: 8
      timeout: 5s
  redis:
    host: redis.internal
`)
	override := map[string]any{
		"database": namedMap{
			"mysql": namedMap{
				"password": "secret",
				"pool": namedMap{
					"max_open": 16,
				},
			},
		},
	}
	got, err := MergeBytes("application.yaml", data, override)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "password: secret") || !strings.Contains(s, "host: db.internal") || !strings.Contains(s, "timeout: 5s") || !strings.Contains(s, "host: redis.internal") {
		t.Fatalf("named map merge lost unrelated fields:\n%s", s)
	}
}
