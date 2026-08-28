package store

import (
	"os"
	"strings"
	"testing"
)

func TestPrunePublishedVersionsKeepsConfiguredNewestAndReclaimsFiles(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	project, err := st.CreateProject(1, st.TestRootUserID(), "sdk-server")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.0.0", "1.0.1", "1.0.2", "1.0.3"} {
		if _, err := st.CreateVersion(1, project.ID, project.Name, version); err != nil {
			t.Fatal(err)
		}
		if err := st.WriteVersionFile(1, project.Name, version, "bin/server", strings.NewReader(version)); err != nil {
			t.Fatal(err)
		}
		if err := st.PublishVersion(1, project.ID, version); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := st.PrunePublishedVersions(1, project.ID, project.Name, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(removed, ","), "1.0.2,1.0.1,1.0.0"; got != want {
		t.Fatalf("removed=%v want=[1.0.2 1.0.1 1.0.0]", removed)
	}
	published, err := st.ListPublishedVersions(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(published) != 1 || published[0].Version != "1.0.3" {
		t.Fatalf("published=%v want newest 1.0.3 only", published)
	}
	if _, err := st.VersionDir(1, project.Name, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	oldDir, _ := st.VersionDir(1, project.Name, "1.0.0")
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old version directory remains: %v", err)
	}
}

func TestPrunePublishedVersionsZeroKeepsEveryVersion(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	project, err := st.CreateProject(1, st.TestRootUserID(), "unlimited-server")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.0.0", "1.0.1"} {
		if _, err := st.CreateVersion(1, project.ID, project.Name, version); err != nil {
			t.Fatal(err)
		}
		if err := st.WriteVersionFile(1, project.Name, version, "bin/server", strings.NewReader(version)); err != nil {
			t.Fatal(err)
		}
		if err := st.PublishVersion(1, project.ID, version); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := st.PrunePublishedVersions(1, project.ID, project.Name, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed=%v want none", removed)
	}
	published, err := st.ListPublishedVersions(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(published) != 2 {
		t.Fatalf("published=%d want=2", len(published))
	}
}
