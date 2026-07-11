package store

import (
	"strings"
	"testing"
	"time"
)

func TestDashboardAggregatesAndScopesProjects(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	p1, err := st.CreateProject(1, st.TestRootUserID(), "dashboard-game")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := st.CreateProject(1, st.TestRootUserID(), "dashboard-hidden")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*Project{p1, p2} {
		v, createErr := st.CreateVersion(1, p.ID, p.Name, "1.0.0")
		if createErr != nil {
			t.Fatal(createErr)
		}
		if writeErr := st.WriteVersionFile(1, p.Name, v.Version, "app.txt", strings.NewReader("ok")); writeErr != nil {
			t.Fatal(writeErr)
		}
		if publishErr := st.PublishVersion(1, p.ID, v.Version); publishErr != nil {
			t.Fatal(publishErr)
		}
	}
	if err := st.RecordUploadEvent(UploadEvent{TenantID: 1, ProjectID: p1.ID, Version: "1.0.0", Kind: "tar", Bytes: 2048, FileCount: 12, Status: "success", Username: "ci"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordUploadEvent(UploadEvent{TenantID: 1, ProjectID: p1.ID, Version: "1.0.0", Kind: "file", Bytes: 7, Status: "failed", Error: "bad archive"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordUploadEvent(UploadEvent{TenantID: 1, ProjectID: p2.ID, Version: "1.0.0", Bytes: 9999, FileCount: 99}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordProjectLog(ProjectLog{TenantID: 1, ProjectID: p1.ID, Action: "pull", Status: "ok", ServerID: "111"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordProjectLog(ProjectLog{TenantID: 1, ProjectID: p1.ID, Action: "pull", Status: "error", Error: "network"}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(timeLayout)
	if _, err := st.db.Exec(`INSERT INTO push_deployments(tenant_id,project_id,version,requested_by,selector,status,created_at,started_at,completed_at) VALUES(1,?,?,?,?,?,?,?,?)`, p1.ID, "1.0.0", "ci", "111", "success", now, now, now); err != nil {
		t.Fatal(err)
	}

	dashboard, err := st.Dashboard([]int64{p1.ID}, 7)
	if err != nil {
		t.Fatal(err)
	}
	summary := dashboard.Summary
	if summary.Uploads != 2 || summary.UploadFailures != 1 || summary.UploadBytes != 2055 || summary.UploadedFiles != 12 {
		t.Fatalf("upload summary: %+v", summary)
	}
	if summary.Publishes != 1 || summary.Pulls != 2 || summary.PullFailures != 1 || summary.Deployments != 1 {
		t.Fatalf("delivery summary: %+v", summary)
	}
	if len(dashboard.Series) != 7 || len(dashboard.Recent) < 5 {
		t.Fatalf("dashboard shape: series=%d recent=%d", len(dashboard.Series), len(dashboard.Recent))
	}
	for _, record := range dashboard.Recent {
		if record.ProjectID == p2.ID {
			t.Fatalf("inaccessible project leaked into dashboard: %+v", record)
		}
	}
}
