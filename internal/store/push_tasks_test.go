package store

import (
	"database/sql"
	"testing"
	"time"
)

func TestPushTaskCRUDAndImmutableDeploymentSnapshot(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, 1, "task-game")
	if err != nil {
		t.Fatal(err)
	}
	host, err := st.CreatePushHost(PushHost{TenantID: 1, Name: "logic", Address: "127.0.0.1", Username: "root", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := st.CreatePushServerBinding(PushServerBinding{TenantID: 1, HostID: host.ID, ServerID: "21", Labels: "test", RemoteRoot: "/srv/game"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := st.CreatePushDeploymentTask(PushDeploymentTask{TenantID: 1, ProjectID: project.ID, Name: "逻辑服发布", ServerIDs: []string{"21", "21"}, Tags: []string{"test"}, CreatedBy: "root"})
	if err != nil {
		t.Fatal(err)
	}
	if len(task.ServerIDs) != 1 || task.TagMatch != "all" {
		t.Fatalf("normalized task: %+v", task)
	}
	deployment, err := st.CreatePushDeployment(PushDeployment{TenantID: 1, ProjectID: project.ID, TaskID: task.ID, TaskName: task.Name, Version: "1.0.0", RequestedBy: "root", Selector: `{"server_ids":["21"],"tags":["test"],"tag_match":"all"}`}, []PushServerBinding{*binding})
	if err != nil {
		t.Fatal(err)
	}
	gotTask, err := st.GetPushDeploymentTask(1, project.ID, task.ID)
	if err != nil || gotTask.RunCount != 1 || gotTask.LastRunAt == "" {
		t.Fatalf("task after run: %+v err=%v", gotTask, err)
	}
	gotTask.Name = "新名字"
	gotTask.Version = "2.0.0"
	if err := st.UpdatePushDeploymentTask(*gotTask); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePushDeploymentTask(1, project.ID, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetPushDeploymentTask(1, project.ID, task.ID); err != sql.ErrNoRows {
		t.Fatalf("deleted task err=%v", err)
	}
	log, err := st.GetPushDeployment(1, project.ID, deployment.ID)
	if err != nil || log.TaskName != "逻辑服发布" || log.Version != "1.0.0" || len(log.Targets) != 1 {
		t.Fatalf("immutable deployment snapshot: %+v err=%v", log, err)
	}
}

func TestPruneLogsUsesThirtyDayRollingWindow(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, 1, "retention-game")
	if err != nil {
		t.Fatal(err)
	}
	host, err := st.CreatePushHost(PushHost{TenantID: 1, Name: "node", Address: "127.0.0.1", Username: "root", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := st.CreatePushServerBinding(PushServerBinding{TenantID: 1, HostID: host.ID, ServerID: "111", Labels: "test", RemoteRoot: "/srv/game"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := st.CreatePushDeploymentTask(PushDeploymentTask{TenantID: 1, ProjectID: project.ID, Name: "保留任务"})
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := st.CreatePushDeployment(PushDeployment{TenantID: 1, ProjectID: project.ID, TaskID: task.ID, TaskName: task.Name, Version: "1.0.0", RequestedBy: "root", Selector: `{}`}, []PushServerBinding{*binding})
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-31 * 24 * time.Hour).Format(timeLayout)
	if _, err := st.db.Exec(`UPDATE push_deployments SET created_at=? WHERE id=?`, old, deployment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`INSERT INTO audit_logs(at,username,action) VALUES(?,?,?)`, old, "root", "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`INSERT INTO release_hook_events(tenant_id,project_id,hook_id,hook_name,kind,source,version,status,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, 1, project.ID, 999, "expired-hook", "trigger", "test", "1.0.0", "scheduled", old); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordProjectLog(ProjectLog{TenantID: 1, ProjectID: project.ID, Action: "pull", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	result, err := st.PruneLogs(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.PushDeployments != 1 || result.AuditLogs != 1 || result.ReleaseHookEvents != 1 {
		t.Fatalf("prune result: %+v", result)
	}
	if _, err := st.GetPushDeployment(1, project.ID, deployment.ID); err != sql.ErrNoRows {
		t.Fatalf("old deployment retained: %v", err)
	}
	var targetCount int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM push_deployment_targets WHERE deployment_id=?`, deployment.ID).Scan(&targetCount); err != nil || targetCount != 0 {
		t.Fatalf("old per-target output retained: count=%d err=%v", targetCount, err)
	}
	if _, err := st.GetPushDeploymentTask(1, project.ID, task.ID); err != nil {
		t.Fatalf("task definition was pruned: %v", err)
	}
	logs, err := st.ListProjectLogs(ProjectLogFilter{TenantID: 1, ProjectID: project.ID})
	if err != nil || len(logs) != 1 {
		t.Fatalf("recent project logs: %+v err=%v", logs, err)
	}
}
