package store

import (
	"testing"
	"time"
)

func TestPushHostHealthCheckSchedulesWithoutRetry(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	host, err := st.CreatePushHost(PushHost{
		TenantID: 1, Name: "node-1", Address: "127.0.0.1", Port: 22,
		Username: "root", AuthMode: "agent", HealthCheckEnabled: true,
		HealthCheckIntervalSeconds: 3600,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	due, err := st.ListDuePushHosts(time.Now().Add(time.Second), 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("due=%+v err=%v", due, err)
	}
	check, err := st.RecordPushHostCheck(PushHostCheck{TenantID: 1, HostID: host.ID, Status: "failed", Error: "connection refused", Trigger: "scheduled", LatencyMS: 12})
	if err != nil {
		t.Fatal(err)
	}
	if check.Status != "failed" || check.ID == 0 {
		t.Fatalf("check=%+v", check)
	}
	// A failure must wait for the complete interval instead of being retried.
	due, err = st.ListDuePushHosts(time.Now().Add(59*time.Minute), 10)
	if err != nil || len(due) != 0 {
		t.Fatalf("unexpected retry: due=%+v err=%v", due, err)
	}
	history, err := st.ListPushHostChecks(1, host.ID, 10)
	if err != nil || len(history) != 1 || history[0].Error != "connection refused" {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	updated, secret, err := st.GetPushHost(1, host.ID)
	if err != nil || secret != "" || updated.LastCheckStatus != "failed" || updated.NextCheckAt == "" {
		t.Fatalf("host=%+v secret=%q err=%v", updated, secret, err)
	}
}

func TestPushHostHealthIntervalValidation(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	_, err = st.CreatePushHost(PushHost{TenantID: 1, Name: "bad", Address: "127.0.0.1", Username: "root", AuthMode: "agent", HealthCheckEnabled: true, HealthCheckIntervalSeconds: 59}, "")
	if err == nil {
		t.Fatal("expected invalid interval")
	}
}
