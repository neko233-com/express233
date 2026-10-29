package store

import (
	"errors"
	"strings"
	"testing"
)

func TestQueuedFleetPinsArtifactAgainstManualAndRetentionDeletion(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.CreateProject(1, 1, "pinned")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.0.0", "1.0.1"} {
		if _, err := st.CreateVersion(1, p.ID, p.Name, version); err != nil {
			t.Fatal(err)
		}
		if err := st.WriteVersionFile(1, p.Name, version, "binary", strings.NewReader("fixture")); err != nil {
			t.Fatal(err)
		}
		if err := st.PublishVersion(1, p.ID, version); err != nil {
			t.Fatal(err)
		}
	}
	d, err := st.CreatePushDeployment(PushDeployment{TenantID: 1, ProjectID: p.ID, Version: "1.0.0", RequestedBy: "root", Selector: "{}"}, []PushServerBinding{{ServerID: "fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteVersion(1, p.ID, p.Name, "1.0.0"); !errors.Is(err, ErrVersionInDeployment) {
		t.Fatalf("queued version deleted: %v", err)
	}
	removed, err := st.PrunePublishedVersions(1, p.ID, p.Name, 1)
	if err != nil || len(removed) != 0 {
		t.Fatal("retention removed active version")
	}
	if err := st.InterruptPushDeployments(); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkPushDeploymentRunning(d.ID); err == nil {
		t.Fatal("interrupted deployment restarted")
	}
	if err := st.DeleteVersion(1, p.ID, p.Name, "1.0.0"); err != nil {
		t.Fatal(err)
	}
}
