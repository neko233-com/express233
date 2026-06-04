package store

import "testing"

func TestAuditLog(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.RecordAudit("root", "test.action", "detail", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	logs, err := st.ListAuditLogs(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Action != "test.action" {
		t.Fatalf("logs: %+v", logs)
	}
}
