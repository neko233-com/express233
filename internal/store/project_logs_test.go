package store

import (
	"testing"
	"time"
)

func TestProjectLogsRetentionAndFilter(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	p, err := st.CreateProject(1, 1, "log-game")
	if err != nil {
		t.Fatal(err)
	}
	oldAt := time.Now().Add(-31 * 24 * time.Hour).Format(timeLayout)
	if err := st.RecordProjectLog(ProjectLog{At: oldAt, TenantID: 1, ProjectID: p.ID, Action: "pull", ServerID: "old", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordProjectLog(ProjectLog{TenantID: 1, ProjectID: p.ID, Username: "root", Action: "pull", ServerID: "s1", Version: "2.0.0", Status: "ok"}); err != nil {
		t.Fatal(err)
	}

	logs, err := st.ListProjectLogs(ProjectLogFilter{TenantID: 1, ProjectID: p.ID, ServerID: "s1", Version: "2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].ServerID != "s1" || logs[0].Version != "2.0.0" {
		t.Fatalf("logs: %+v", logs)
	}
	all, err := st.ListProjectLogs(ProjectLogFilter{TenantID: 1, ProjectID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("retention logs: %+v", all)
	}
}
