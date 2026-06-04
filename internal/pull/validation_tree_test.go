package pull_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/hookspec"
	"github.com/neko233-com/express233/internal/pull"
	"github.com/neko233-com/express233/internal/template"
	"gopkg.in/yaml.v3"
)

func TestValidationTree_PullReplacementsPerServer(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "validation-tree")
	versionRoot := filepath.Join(root, "version")

	sf, err := config.LoadServerFile(filepath.Join(root, "server.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	for _, sid := range []string{"game-srv-01", "game-srv-02"} {
		var buf bytes.Buffer
		if err := pull.BuildBundleFromDir(versionRoot, sf, "tree-demo", "1.0.0", sid, &buf); err != nil {
			t.Fatalf("%s build: %v", sid, err)
		}
		dest := t.TempDir()
		m, err := pull.ExtractBundle(&buf, dest)
		if err != nil {
			t.Fatalf("%s extract: %v", sid, err)
		}
		if m.ServerID != sid {
			t.Fatalf("manifest server_id: %s", m.ServerID)
		}
		if m.PostHookSpec != hookspec.DefaultPath {
			t.Fatalf("expected post_hook_spec, got %q", m.PostHookSpec)
		}
		expectRoot := filepath.Join(root, "expected", sid)
		assertFileEqual(t, filepath.Join(expectRoot, "conf", "app", "application.yaml"),
			filepath.Join(dest, "conf", "app", "application.yaml"))
		assertFileEqual(t, filepath.Join(expectRoot, "deploy", "game.properties"),
			filepath.Join(dest, "deploy", "game.properties"))
	}
}

func TestValidationTree_PostHookPlan(t *testing.T) {
	versionRoot := filepath.Join("..", "..", "testdata", "validation-tree", "version")
	plan, err := hookspec.PlanLines(versionRoot, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) == 0 || !strings.Contains(plan[0], "restart.sh") {
		t.Fatalf("linux plan: %v", plan)
	}
	planWin, err := hookspec.PlanLines(versionRoot, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if len(planWin) == 0 || !strings.Contains(planWin[0], "restart.ps1") {
		t.Fatalf("windows plan: %v", planWin)
	}
}

func TestValidationTree_PostHookExecuteLinux(t *testing.T) {
	if hookspec.CurrentOS() == "windows" {
		t.Skip("skip hook execute on windows CI")
	}
	versionRoot := filepath.Join("..", "..", "testdata", "validation-tree", "version")
	dest := t.TempDir()
	copyTree(versionRoot, dest)
	err := hookspec.Execute(dest, hookspec.RunContext{
		DestDir: dest,
		Env:     map[string]string{"EXPRESS233_SERVER_ID": "game-srv-01"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, ".express233", "hook-ran.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "restart-linux\n" && string(b) != "restart-linux" {
		if !strings.Contains(string(b), "restart-linux") {
			t.Fatalf("hook marker: %q", string(b))
		}
	}
}

func assertFileEqual(t *testing.T, wantPath, gotPath string) {
	t.Helper()
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("missing %s: %v", gotPath, err)
	}
	base := filepath.Base(wantPath)
	switch strings.ToLower(filepath.Ext(base)) {
	case ".yaml", ".yml":
		if !yamlMapsEqual(want, got) {
			t.Fatalf("%s\nyaml semantic mismatch\n--- want ---\n%s\n--- got ---\n%s", gotPath, want, got)
		}
	default:
		if strings.ReplaceAll(string(want), "\r\n", "\n") != strings.ReplaceAll(string(got), "\r\n", "\n") {
			t.Fatalf("%s\ncontent mismatch\n--- want ---\n%s\n--- got ---\n%s", gotPath, want, got)
		}
	}
}

func yamlMapsEqual(a, b []byte) bool {
	var ma, mb map[string]any
	if yaml.Unmarshal(a, &ma) != nil || yaml.Unmarshal(b, &mb) != nil {
		return false
	}
	return reflect.DeepEqual(template.FlattenScalars("", ma), template.FlattenScalars("", mb))
}

func copyTree(src, dst string) {
	_ = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		data, _ := os.ReadFile(path)
		_ = os.WriteFile(target, data, 0o644)
		return nil
	})
}
