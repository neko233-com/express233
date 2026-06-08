package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageIndexAndSearch(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tid := int64(1)
	p, err := st.CreateProject(tid, 1, "stor-proj")
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	_ = v
	root, _ := st.VersionDir(tid, p.Name, "1.0.0")
	if err := os.WriteFile(filepath.Join(root, "game.properties"), []byte("port=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := st.RebuildStorageIndex(tid)
	if err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Fatalf("expected index entries, got %d", n)
	}

	hits, err := st.SearchStorageIndex(tid, "game.properties", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected search hit")
	}
	if hits[0].ProjectName != "stor-proj" {
		t.Fatalf("project: %s", hits[0].ProjectName)
	}
}

func TestStorageOverviewAndDeletePlan(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tid := int64(1)
	p, err := st.CreateProject(tid, 1, "del-proj")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVersion(tid, p.ID, p.Name, "1.0.0"); err != nil {
		t.Fatal(err)
	}

	ov, err := st.StorageOverviewForTenant(tid)
	if err != nil {
		t.Fatal(err)
	}
	if ov.ProjectCount != 1 || ov.VersionCount != 1 {
		t.Fatalf("counts: projects=%d versions=%d", ov.ProjectCount, ov.VersionCount)
	}
	if ov.TotalBytes <= 0 {
		t.Fatal("expected positive total bytes")
	}

	tn, _ := st.TenantByID(tid)
	relVer := filepath.ToSlash(filepath.Join("userdata", tn.Slug, "projects", p.Name, "1.0.0"))
	plan, err := st.PlanStorageDelete(tid, relVer, RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Allowed {
		t.Fatalf("expected allowed delete plan: %s", plan.DenyReason)
	}
	if plan.Kind != "version" {
		t.Fatalf("kind: %s", plan.Kind)
	}
}

func TestStorageTreeAt(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tid := int64(1)
	p, _ := st.CreateProject(tid, 1, "tree-proj")
	_, _ = st.CreateVersion(tid, p.ID, p.Name, "2.0.0")

	node, err := st.StorageTreeAt(tid, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(node.Path, "userdata") {
		t.Fatalf("root path: %s", node.Path)
	}
	found := false
	for _, c := range node.Children {
		if c.Name == "projects" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected projects folder in tree")
	}
}
