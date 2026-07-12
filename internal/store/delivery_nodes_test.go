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
