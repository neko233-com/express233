package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/express233/internal/store"
)

func TestPublishedVersionQueuesAndCoalescesReleaseHook(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	users, _ := st.ListUsers(1)
	project, err := st.CreateProject(1, users[0].ID, "fictional-hook-api")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVersion(1, project.ID, project.Name, "3.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(1, project.Name, "3.0.0", "bin/logic-server", strings.NewReader("synthetic-binary")); err != nil {
		t.Fatal(err)
	}
	host, _ := st.CreatePushHost(store.PushHost{TenantID: 1, Name: "loopback-unreachable", Address: "127.0.0.1", Port: 1, Username: "nobody", AuthMode: "agent"}, "")
	if _, err := st.CreatePushServerBinding(store.PushServerBinding{TenantID: 1, HostID: host.ID, ServerID: "fictional-21", Labels: "test", RemoteRoot: "/tmp/fictional-game"}); err != nil {
		t.Fatal(err)
	}
	task, _ := st.CreatePushDeploymentTask(store.PushDeploymentTask{TenantID: 1, ProjectID: project.ID, Name: "fictional-auto-release", ServerIDs: []string{"fictional-21"}, Tags: []string{"test"}, CreatedBy: "root"})
	hook, err := st.CreateReleaseHook(store.ReleaseHook{TenantID: 1, ProjectID: project.ID, TaskID: task.ID, Name: "gitea-upload-hook", Enabled: true, DebounceSeconds: 30, CreatedBy: "root"})
	if err != nil {
		t.Fatal(err)
	}

	server := New(st)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()
	jar := login(t, ts, "root", "root")
	path := fmt.Sprintf("/api/projects/%d/versions/3.0.0/publish", project.ID)
	mustPOST(t, ts, jar, path, nil)
	mustPOST(t, ts, jar, path, nil) // idempotent CI retry: merged, not duplicated
	queued, err := st.GetReleaseHook(1, project.ID, hook.ID)
	if err != nil || queued.PendingEvents != 2 || queued.MergeCount != 1 {
		t.Fatalf("queued hook: %+v err=%v", queued, err)
	}
	due, err := time.Parse("2006-01-02 15:04:05", queued.DueAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.processDueReleaseHooks(due.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	deployments, err := st.ListPushDeployments(1, project.ID, 20)
	if err != nil || len(deployments) != 1 || deployments[0].TaskID != task.ID || deployments[0].Version != "3.0.0" {
		t.Fatalf("coalesced deployments: %+v err=%v", deployments, err)
	}
	events, err := st.ListReleaseHookEvents(1, project.ID, 20)
	if err != nil || len(events) != 3 || events[0].Kind != "dispatch" || events[0].MergedEvents != 2 {
		t.Fatalf("hook history: %+v err=%v", events, err)
	}
	if deployments[0].HookEventID != events[0].ID {
		t.Fatalf("deployment idempotency key=%d event=%d", deployments[0].HookEventID, events[0].ID)
	}
	targets, _ := st.ListPushBindingsForSelector(1, project.Name, []string{"fictional-21"}, []string{"test"}, true)
	if _, err := st.CreatePushDeployment(store.PushDeployment{TenantID: 1, ProjectID: project.ID, TaskID: task.ID, TaskName: task.Name, HookEventID: events[0].ID, Version: "3.0.0", RequestedBy: "root", Selector: `{}`}, targets); err == nil {
		t.Fatal("duplicate deployment accepted for the same hook event")
	}
	mustPOST(t, ts, jar, fmt.Sprintf("/api/projects/%d/release-hooks/%d/trigger", project.ID, hook.ID), map[string]string{"source": "test_ci"})
	body, _ := json.Marshal(map[string]any{"name": hook.Name, "task_id": task.ID, "enabled": false, "debounce_seconds": 30})
	request, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/projects/%d/release-hooks/%d", ts.URL, project.ID, hook.ID), bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range jar {
		request.AddCookie(cookie)
	}
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("disable hook status=%d", response.StatusCode)
	}
	disabled, _ := st.GetReleaseHook(1, project.ID, hook.ID)
	if disabled.Enabled || disabled.PendingEvents != 0 || disabled.DueAt != "" {
		t.Fatalf("disabled hook: %+v", disabled)
	}
	if err := server.processDueReleaseHooks(time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	deployments, _ = st.ListPushDeployments(1, project.ID, 20)
	if len(deployments) != 1 {
		t.Fatalf("cancelled hook dispatched: %+v", deployments)
	}
	events, _ = st.ListReleaseHookEvents(1, project.ID, 20)
	if len(events) != 5 || events[0].Status != "cancelled" {
		t.Fatalf("cancellation history: %+v", events)
	}
}
