package store

import (
	"testing"
	"time"
)

func TestDeliveryNodeDesiredStateAndAutoFollow(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, 1, "fictional-control-plane")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2031, 2, 3, 4, 5, 6, 0, time.Local)
	node, err := st.HeartbeatDeliveryNode(DeliveryNodeHeartbeat{TenantID: 1, ProjectID: project.ID, ServerID: "logic-fictional-21", Role: "logic", Environment: "test", Labels: []string{"region:test", "role:logic"}, OS: "linux", Arch: "amd64", CurrentVersion: "1.0.0", Status: "ok", HeartbeatIntervalSeconds: 30}, now)
	if err != nil {
		t.Fatal(err)
	}
	if node.DeliveryMode != "pull" || node.CurrentVersion != "1.0.0" || len(node.Labels) != 2 {
		t.Fatalf("registered node: %+v", node)
	}
	autoFollow := true
	node, err = st.SetDeliveryNodeDesired(1, project.ID, node.ServerID, "1.1.0", &autoFollow, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if node.DesiredGeneration != 1 || node.DesiredVersion != "1.1.0" || !node.AutoFollow || node.Status != "pending" {
		t.Fatalf("desired state: %+v", node)
	}
	node, err = st.HeartbeatDeliveryNode(DeliveryNodeHeartbeat{TenantID: 1, ProjectID: project.ID, ServerID: node.ServerID, Role: "logic", Environment: "test", CurrentVersion: "1.1.0", AppliedGeneration: 1, Status: "ok", HeartbeatIntervalSeconds: 30}, now.Add(2*time.Second))
	if err != nil || node.AppliedGeneration != 1 || node.CurrentVersion != node.DesiredVersion {
		t.Fatalf("converged node: %+v err=%v", node, err)
	}
	advanced, err := st.AdvanceAutoFollowDeliveryNodes(1, project.ID, "1.2.0", now.Add(3*time.Second))
	if err != nil || advanced != 1 {
		t.Fatalf("advance auto-follow count=%d err=%v", advanced, err)
	}
	replayed, err := st.AdvanceAutoFollowDeliveryNodes(1, project.ID, "1.2.0", now.Add(4*time.Second))
	if err != nil || replayed != 0 {
		t.Fatalf("idempotent publish replay count=%d err=%v", replayed, err)
	}
	node, _ = st.GetDeliveryNode(1, project.ID, node.ServerID)
	if node.DesiredVersion != "1.2.0" || node.DesiredGeneration != 2 {
		t.Fatalf("auto-follow state: %+v", node)
	}
}

func TestDeliveryNodeRejectsUnsafeMetadata(t *testing.T) {
	st, _ := Open(t.TempDir())
	defer st.Close()
	project, _ := st.CreateProject(1, 1, "fictional-node-validation")
	if _, err := st.HeartbeatDeliveryNode(DeliveryNodeHeartbeat{TenantID: 1, ProjectID: project.ID, ServerID: "../../secret", Status: "ok"}, time.Now()); err == nil {
		t.Fatal("unsafe server_id accepted")
	}
}

func TestLatestSuccessfulPushVersionIgnoresDryRunsAndPreservesHistory(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, 1, "dry-run-history")
	if err != nil {
		t.Fatal(err)
	}
	host, err := st.CreatePushHost(PushHost{TenantID: 1, Name: "fixture-node", Address: "192.0.2.10", Username: "deploy", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := st.CreatePushServerBinding(PushServerBinding{TenantID: 1, HostID: host.ID, ProjectName: project.Name, ServerID: "logic-01", RemoteRoot: "/srv/game"})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		name, version, selector, status, want string
	}{
		{"preview only", "1.0.0", `{"dry_run":true}`, "success", ""},
		{"legacy deployment without flag", "1.0.0", `{"server_ids":["logic-01"]}`, "success", "1.0.0"},
		{"newer preview preserves installed version", "2.0.0", `{"dry_run":true}`, "success", "1.0.0"},
		{"actual deployment advances version", "2.0.0", `{"dry_run":false}`, "success", "2.0.0"},
		{"failed deployment preserves installed version", "3.0.0", `{"dry_run":false}`, "failed", "2.0.0"},
		{"legacy empty selector remains readable", "1.5.0", "", "success", "1.5.0"},
		{"preview after rollback preserves installed version", "4.0.0", `{"dry_run":true}`, "success", "1.5.0"},
	} {
		t.Run(step.name, func(t *testing.T) {
			deployment, err := st.CreatePushDeployment(PushDeployment{TenantID: 1, ProjectID: project.ID, Version: step.version, RequestedBy: "fixture", Selector: step.selector}, []PushServerBinding{*binding})
			if err != nil {
				t.Fatal(err)
			}
			detail, err := st.GetPushDeploymentByID(deployment.ID)
			if err != nil || len(detail.Targets) != 1 {
				t.Fatalf("load target: err=%v", err)
			}
			if err := st.MarkPushTarget(detail.Targets[0].ID, step.status, "fixture result"); err != nil {
				t.Fatal(err)
			}
			if err := st.CompletePushDeployment(deployment.ID); err != nil {
				t.Fatal(err)
			}
			got, err := st.LatestSuccessfulPushVersion(project.ID, binding.ID)
			if err != nil || got != step.want {
				t.Fatalf("installed version=%q, want %q, err=%v", got, step.want, err)
			}
		})
	}
}
