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
	if _, err := st.CreatePushServerBinding(store.PushServerBinding{TenantID: 1, HostID: host.ID, ServerID: "21", Labels: "test", RemoteRoot: "/srv/game"}); err != nil {
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
	tasks := mustGET[[]store.PushDeploymentTask](t, ts, jar, base+"/push-tasks")
	if len(tasks) != 1 || tasks[0].RunCount != 2 {
		t.Fatalf("tasks after repeat: %+v", tasks)
	}
	doJSON(http.MethodDelete, fmt.Sprintf("%s/push-tasks/%d", base, task.ID), nil, http.StatusNoContent, nil)
	logs := mustGET[[]store.PushDeployment](t, ts, jar, base+"/push-deployments")
	if len(logs) != 2 || logs[0].TaskName != task.Name {
		t.Fatalf("logs after task deletion: %+v", logs)
	}
	doJSON(http.MethodDelete, fmt.Sprintf("%s/push-deployments/%d", base, logs[0].ID), nil, http.StatusConflict, nil)
}
