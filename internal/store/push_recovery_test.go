package store

import "testing"

func TestInterruptedFleetRetainsUnknownOutcomeAndCannotBeClaimedAgain(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.CreateProject(1, 1, "interrupted-fleet")
	if err != nil {
		t.Fatal(err)
	}
	d, err := st.CreatePushDeployment(PushDeployment{TenantID: 1, ProjectID: p.ID, Version: "fixture", RequestedBy: "root", Selector: "{}"}, []PushServerBinding{{HostID: 1, ServerID: "one"}, {HostID: 1, ServerID: "two"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MarkPushDeploymentRunning(d.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.GetPushDeploymentByID(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MarkPushTarget(loaded.Targets[0].ID, "running", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.InterruptPushDeployments(); err != nil {
		t.Fatal(err)
	}
	loaded, err = st.GetPushDeploymentByID(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != "failed" || loaded.Targets[0].Status != "unknown" || loaded.Targets[1].Status != "skipped" {
		t.Fatalf("interrupted result lost: %+v", loaded)
	}
	if err := st.MarkPushDeploymentRunning(d.ID); err == nil {
		t.Fatal("interrupted deployment replayed")
	}
}
