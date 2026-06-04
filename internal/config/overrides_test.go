package config

import "testing"

func TestFileOverridesNormalizeNestedYAML(t *testing.T) {
	raw := FileOverrides{
		"mysql": map[string]any{
			"url":      "jdbc://x",
			"password": "p",
		},
	}
	out, err := raw.Normalize("application.yaml")
	if err != nil {
		t.Fatal(err)
	}
	mysql, ok := out["mysql"].(map[string]any)
	if !ok || mysql["password"] != "p" {
		t.Fatalf("got %+v", out)
	}
}

func TestFileOverridesDottedAndNestedMergeSameTree(t *testing.T) {
	raw := FileOverrides{
		"mysql.url": "jdbc://x",
		"mysql":     map[string]any{"password": "p"},
	}
	out, err := raw.Normalize("application.yaml")
	if err != nil {
		t.Fatal(err)
	}
	mysql, ok := out["mysql"].(map[string]any)
	if !ok || mysql["url"] != "jdbc://x" || mysql["password"] != "p" {
		t.Fatalf("merge tree: %+v", out)
	}
}
