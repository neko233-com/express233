package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestReusablePushTaskAPIAndImmutableLogs(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	users, _ := st.ListUsers(1)
	project, err := st.CreateProject(1, users[0].ID, "push-task-api")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVersion(1, project.ID, project.Name, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(1, project.Name, "1.0.0", "game.bin", strings.NewReader("release")); err != nil {
		t.Fatal(err)
	}
	if err := st.PublishVersion(1, project.ID, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	host, err := st.CreatePushHost(store.PushHost{TenantID: 1, Name: "node", Address: "127.0.0.1", Username: "root", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePushServerBinding(store.PushServerBinding{TenantID: 1, HostID: host.ID, ProjectName: project.Name, ServerID: "21", Labels: "test", RemoteRoot: "/srv/game"}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()
	jar := login(t, ts, "root", "root")
	doJSON := func(method, path string, body any, want int, out any) {
		t.Helper()
		encoded, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, ts.URL+path, bytes.NewReader(encoded))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range jar {
			req.AddCookie(cookie)
		}
		response, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("%s %s status=%d want=%d", method, path, response.StatusCode, want)
		}
		if out != nil {
			_ = json.NewDecoder(response.Body).Decode(out)
		}
	}

	base := fmt.Sprintf("/api/projects/%d", project.ID)
	var task store.PushDeploymentTask
	doJSON(http.MethodPost, base+"/push-tasks", map[string]any{"name": "21 服重复发布", "server_ids": []string{"21"}, "tags": []string{"test"}}, http.StatusCreated, &task)
	for range 2 {
		var deployment store.PushDeployment
		doJSON(http.MethodPost, fmt.Sprintf("%s/push-tasks/%d/run", base, task.ID), map[string]any{"dry_run": true}, http.StatusCreated, &deployment)
		if deployment.TaskID != task.ID || deployment.TaskName != task.Name || deployment.Version != "1.0.0" || deployment.Status != "success" {
			t.Fatalf("deployment snapshot: %+v", deployment)
		}
	}
	var firstReplay, secondReplay store.PushDeployment
	requestRun := func(want int, out *store.PushDeployment) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s%s/push-tasks/%d/run", ts.URL, base, task.ID), strings.NewReader(`{"dry_run":true}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "ui-release-request-001")
		for _, cookie := range jar {
			req.AddCookie(cookie)
		}
		response, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("idempotent run status=%d want=%d", response.StatusCode, want)
		}
		if err := json.NewDecoder(response.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	requestRun(http.StatusCreated, &firstReplay)
	requestRun(http.StatusOK, &secondReplay)
	if firstReplay.ID != secondReplay.ID || !secondReplay.Replayed {
		t.Fatalf("idempotent API replay: first=%+v second=%+v", firstReplay, secondReplay)
	}
	tasks := mustGET[[]store.PushDeploymentTask](t, ts, jar, base+"/push-tasks")
	if len(tasks) != 1 || tasks[0].RunCount != 3 {
		t.Fatalf("tasks after repeat: %+v", tasks)
	}
	doJSON(http.MethodDelete, fmt.Sprintf("%s/push-tasks/%d", base, task.ID), nil, http.StatusNoContent, nil)
	logs := mustGET[[]store.PushDeployment](t, ts, jar, base+"/push-deployments")
	if len(logs) != 3 || logs[0].TaskName != task.Name {
		t.Fatalf("logs after task deletion: %+v", logs)
	}
	doJSON(http.MethodDelete, fmt.Sprintf("%s/push-deployments/%d", base, logs[0].ID), nil, http.StatusConflict, nil)
}

func TestDirectPushDeploymentCanFollowAllCurrentProjectTargets(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	users, _ := st.ListUsers(1)
	project, _ := st.CreateProject(1, users[0].ID, "quick-release-project")
	otherProject, _ := st.CreateProject(1, users[0].ID, "other-project")
	if _, err := st.CreateVersion(1, project.ID, project.Name, "2.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(1, project.Name, "2.0.0", "game.bin", strings.NewReader("release")); err != nil {
		t.Fatal(err)
	}
	if err := st.PublishVersion(1, project.ID, "2.0.0"); err != nil {
		t.Fatal(err)
	}
	host, _ := st.CreatePushHost(store.PushHost{TenantID: 1, Name: "node", Address: "127.0.0.1", Username: "root", AuthMode: "agent"}, "")
	_, _ = st.CreatePushServerBinding(store.PushServerBinding{TenantID: 1, HostID: host.ID, ProjectName: project.Name, ServerID: "1001", Labels: "prod,logic", RemoteRoot: "/srv/game/1001", SkipBackup: true})
	_, _ = st.CreatePushServerBinding(store.PushServerBinding{TenantID: 1, HostID: host.ID, ProjectName: otherProject.Name, ServerID: "2001", Labels: "prod,logic", RemoteRoot: "/srv/game/2001"})

	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()
	jar := login(t, ts, "root", "root")
	base := fmt.Sprintf("/api/projects/%d", project.ID)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+base+"/push-deployments", strings.NewReader(`{"all_project_targets":true,"dry_run":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "quick-release-project-001")
	for _, cookie := range jar {
		req.AddCookie(cookie)
	}
	response, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("quick release status=%d", response.StatusCode)
	}
	var deployment store.PushDeployment
	if err := json.NewDecoder(response.Body).Decode(&deployment); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	replayReq, _ := http.NewRequest(http.MethodPost, ts.URL+base+"/push-deployments", strings.NewReader(`{"all_project_targets":true,"dry_run":true}`))
	replayReq.Header.Set("Content-Type", "application/json")
	replayReq.Header.Set("Idempotency-Key", "quick-release-project-001")
	for _, cookie := range jar {
		replayReq.AddCookie(cookie)
	}
	replayResponse, err := ts.Client().Do(replayReq)
	if err != nil {
		t.Fatal(err)
	}
	defer replayResponse.Body.Close()
	if replayResponse.StatusCode != http.StatusOK {
		t.Fatalf("quick release replay status=%d", replayResponse.StatusCode)
	}
	var replay store.PushDeployment
	if err := json.NewDecoder(replayResponse.Body).Decode(&replay); err != nil {
		t.Fatal(err)
	}
	if replay.ID != deployment.ID || !replay.Replayed {
		t.Fatalf("quick release replay created duplicate: first=%+v replay=%+v", deployment, replay)
	}
	detail := mustGET[store.PushDeployment](t, ts, jar, fmt.Sprintf("%s/push-deployments/%d", base, deployment.ID))
	if len(detail.Targets) != 1 || detail.Targets[0].ServerID != "1001" {
		t.Fatalf("quick release must stay inside project: %+v", detail.Targets)
	}
	// The existing persisted selector is the API contract for distinguishing
	// previews in creation, replay, detail and list responses.
	logs := mustGET[[]store.PushDeployment](t, ts, jar, base+"/push-deployments")
	if len(logs) != 1 {
		t.Fatalf("expected one idempotent preview, got %d records", len(logs))
	}
	for _, record := range []store.PushDeployment{deployment, replay, detail, logs[0]} {
		var selector map[string]any
		if err := json.Unmarshal([]byte(record.Selector), &selector); err != nil {
			t.Fatal(err)
		}
		if selector["dry_run"] != true {
			t.Fatalf("preview %d lost its boolean dry_run marker", record.ID)
		}
	}
	nodes := mustGET[[]deliveryNodeView](t, ts, jar, base+"/delivery-nodes")
	if len(nodes) != 1 || nodes[0].ServerID != "1001" || !nodes[0].SkipBackup {
		t.Fatalf("quick release backup policy view: %+v", nodes)
	}
	if nodes[0].CurrentVersion != "" {
		t.Fatalf("preview must not report a deployed version: %q", nodes[0].CurrentVersion)
	}
}
