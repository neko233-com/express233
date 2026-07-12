package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionVCSProvenanceAndPublishedManifestAreImmutable(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, st.TestRootUserID(), "provenance-game")
	if err != nil {
		t.Fatal(err)
	}
	version, err := st.CreateVersion(1, project.ID, project.Name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	vcs := VCSProvenance{Provider: "gitea", Repository: "https://git.example.test/game/server", Ref: "refs/tags/v1.0.0", Commit: "0123456789abcdef", RunURL: "https://git.example.test/game/server/actions/runs/7", RunID: "7"}
	if err := st.SetVersionVCSProvenance(project.ID, version.Version, vcs); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(1, project.Name, version.Version, "bin/server", strings.NewReader("binary-v1")); err != nil {
		t.Fatal(err)
	}
	if err := st.PublishVersion(1, project.ID, version.Version); err != nil {
		t.Fatal(err)
	}
	published, err := st.GetVersion(project.ID, version.Version)
	if err != nil {
		t.Fatal(err)
	}
	if published.VCS.Commit != vcs.Commit || published.Artifact.SHA256 == "" || published.Artifact.FileCount != 1 || published.Artifact.Bytes != int64(len("binary-v1")) {
		t.Fatalf("unexpected published provenance: %+v", published)
	}
	if err := st.SetVersionVCSProvenance(project.ID, version.Version, vcs); err == nil {
		t.Fatal("published provenance mutation should fail")
	}

	root, err := st.VersionDir(1, project.Name, version.Version)
	if err != nil {
		t.Fatal(err)
	}
	first, err := BuildArtifactManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "server"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := BuildArtifactManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 == second.SHA256 {
		t.Fatal("content change must alter manifest digest")
	}
}

func TestValidateVCSProvenanceRejectsCredentialURL(t *testing.T) {
	err := ValidateVCSProvenance(VCSProvenance{Provider: "github", Repository: "https://token@example.test/org/repo", Ref: "main", Commit: "0123456789abcdef"})
	if err == nil {
		t.Fatal("credential URL should be rejected")
	}
}
