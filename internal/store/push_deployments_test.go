package store

import "testing"

func TestPushBindingsSupportRepeatedServerIDByLabel(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const tid int64 = 1
	host, err := st.CreatePushHost(PushHost{TenantID: tid, Name: "logic-a", Address: "10.0.0.10", Username: "deploy", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"test", "canary"} {
		if _, err := st.CreatePushServerBinding(PushServerBinding{TenantID: tid, HostID: host.ID, ServerID: "logic-01", Labels: label, RemoteRoot: "/opt/game"}); err != nil {
			t.Fatalf("create %s binding: %v", label, err)
		}
	}
	targets, err := st.ListPushBindingsForSelector(tid, "", []string{"logic-01"}, []string{"test"}, true)
	if err != nil || len(targets) != 1 || targets[0].Labels != "test" {
		t.Fatalf("test selector: targets=%+v err=%v", targets, err)
	}
	all, err := st.ListPushBindingsForSelector(tid, "", []string{"logic-01"}, nil, true)
	if err != nil || len(all) != 2 {
		t.Fatalf("all selector: targets=%+v err=%v", all, err)
	}
}

func TestPushBindingsAreScopedByProjectTargetTag(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const tid int64 = 1
	host, err := st.CreatePushHost(PushHost{TenantID: tid, Name: "logic-projects", Address: "10.0.0.12", Username: "deploy", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{"game-server-sf", "game-server-hz"} {
		binding, err := st.CreatePushServerBinding(PushServerBinding{TenantID: tid, HostID: host.ID, ProjectName: project, ServerID: "111", Labels: "prod", RemoteRoot: "/opt/game"})
		if err != nil {
			t.Fatalf("create %s binding: %v", project, err)
		}
		if binding.TargetTag != project+"|111" {
			t.Fatalf("target tag=%q", binding.TargetTag)
		}
	}
	targets, err := st.ListPushBindingsForSelector(tid, "game-server-sf", []string{"111"}, []string{"prod"}, true)
	if err != nil || len(targets) != 1 || targets[0].ProjectName != "game-server-sf" {
		t.Fatalf("project selector: targets=%+v err=%v", targets, err)
	}
}

func TestPushDeploymentIdempotencyKeyReturnsOriginalExecution(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, 1, "idempotent-release")
	if err != nil {
		t.Fatal(err)
	}
	host, err := st.CreatePushHost(PushHost{TenantID: 1, Name: "idempotent-node", Address: "10.0.0.13", Username: "deploy", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := st.CreatePushServerBinding(PushServerBinding{TenantID: 1, HostID: host.ID, ProjectName: project.Name, ServerID: "111", Labels: "prod", RemoteRoot: "/opt/game"})
	if err != nil {
		t.Fatal(err)
	}
	request := PushDeployment{TenantID: 1, ProjectID: project.ID, Version: "0.0.1", RequestedBy: "root", Selector: `{}`, IdempotencyKey: "release-request-001"}
	first, err := st.CreatePushDeployment(request, []PushServerBinding{*binding})
	if err != nil || first.Replayed {
		t.Fatalf("first execution: %+v err=%v", first, err)
	}
	second, err := st.CreatePushDeployment(request, []PushServerBinding{*binding})
	if err != nil || !second.Replayed || second.ID != first.ID {
		t.Fatalf("replayed execution: first=%+v second=%+v err=%v", first, second, err)
	}
	var count int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM push_deployments WHERE tenant_id=? AND idempotency_key=?`, 1, request.IdempotencyKey).Scan(&count); err != nil || count != 1 {
		t.Fatalf("deployment count=%d err=%v", count, err)
	}
	otherProject, err := st.CreateProject(1, 1, "other-idempotent-release")
	if err != nil {
		t.Fatal(err)
	}
	request.ProjectID = otherProject.ID
	third, err := st.CreatePushDeployment(request, []PushServerBinding{*binding})
	if err != nil || third.Replayed || third.ID == first.ID {
		t.Fatalf("project-scoped key: first=%+v third=%+v err=%v", first, third, err)
	}
}

func TestPushHostTOFUDoesNotOverwriteKnownKey(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const tid int64 = 1
	host, err := st.CreatePushHost(PushHost{TenantID: tid, Name: "logic-b", Address: "10.0.0.11", Username: "deploy", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordPushHostKey(tid, host.ID, "ssh-ed25519 AAAAfirst", "SHA256:first"); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordPushHostKey(tid, host.ID, "ssh-ed25519 AAAAsecond", "SHA256:second"); err != nil {
		t.Fatal(err)
	}
	got, _, err := st.GetPushHost(tid, host.ID)
	if err != nil || got.HostKeyFingerprint != "SHA256:first" {
		t.Fatalf("TOFU key=%+v err=%v", got, err)
	}
}
