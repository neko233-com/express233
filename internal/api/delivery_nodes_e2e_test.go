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

func TestDeliveryNodeDesiredStateEndToEnd(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	users, _ := st.ListUsers(1)
	project, err := st.CreateProject(1, users[0].ID, "fictional-cluster")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVersion(1, project.ID, project.Name, "4.2.0"); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(1, project.Name, "4.2.0", "bin/fictional-server", strings.NewReader("synthetic executable")); err != nil {
		t.Fatal(err)
	}
	if err := st.PublishVersion(1, project.ID, "4.2.0"); err != nil {
		t.Fatal(err)
	}

	server := New(st)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()
	jar := login(t, ts, "root", "root")
	status, _ := doPut2(t, ts, jar, "/api/server-yaml", map[string]string{
		"content": "servers:\n  logic-fictional-21:\n    replacements: {}\n",
	})
	if status != http.StatusOK && status != http.StatusNoContent {
		t.Fatalf("server yaml status=%d", status)
	}

	heartbeat := map[string]any{
		"project": "fictional-cluster", "server_id": "logic-fictional-21",
		"role": "logic", "environment": "staging", "labels": []string{"role:logic", "env:staging"},
		"os": "linux", "arch": "amd64", "status": "ok", "heartbeat_interval_seconds": 30,
	}
	first := heartbeatDeliveryNode(t, ts, users[0].Token, heartbeat)
	if first.Desired.NeedsDeploy {
		t.Fatal("new node should have no desired version")
	}

	path := fmt.Sprintf("/api/projects/%d/delivery-nodes/logic-fictional-21/desired", project.ID)
	putDeliveryDesired(t, ts, jar, path, map[string]any{"version": "4.2.0", "auto_follow": true})
	second := heartbeatDeliveryNode(t, ts, users[0].Token, heartbeat)
	if !second.Desired.NeedsDeploy || second.Desired.Version != "4.2.0" || second.Desired.Generation != 1 {
		t.Fatalf("desired response = %+v", second.Desired)
	}
	heartbeat["current_version"] = "4.2.0"
	heartbeat["applied_generation"] = second.Desired.Generation
	ack := heartbeatDeliveryNode(t, ts, users[0].Token, heartbeat)
	if ack.Desired.NeedsDeploy {
		t.Fatal("acknowledged node must be converged")
	}

	nodes := mustGET[[]deliveryNodeView](t, ts, jar, fmt.Sprintf("/api/projects/%d/delivery-nodes", project.ID))
	if len(nodes) != 1 || nodes[0].DeliveryMode != "pull" || !nodes[0].Online || nodes[0].Drift {
		t.Fatalf("node inventory = %+v", nodes)
	}

	if _, err := st.CreateVersion(1, project.ID, project.Name, "4.3.0"); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(1, project.Name, "4.3.0", "bin/fictional-server", strings.NewReader("synthetic executable v2")); err != nil {
		t.Fatal(err)
	}
	publishPath := fmt.Sprintf("/api/projects/%d/versions/4.3.0/publish", project.ID)
	mustPOST(t, ts, jar, publishPath, nil)
	mustPOST(t, ts, jar, publishPath, nil) // CI replay must not bump desired generation twice.
	node, err := st.GetDeliveryNode(1, project.ID, "logic-fictional-21")
	if err != nil {
		t.Fatal(err)
	}
	if node.DesiredVersion != "4.3.0" || node.DesiredGeneration != 2 {
		t.Fatalf("publish convergence = %+v", node)
	}
}

func TestDeliveryAgentInvalidTokenBlocksIPAfterFiveAttempts(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()
	body, _ := json.Marshal(map[string]any{"project": "fictional", "server_id": "logic-fictional", "status": "ok", "heartbeat_interval_seconds": 30})
	for attempt := 1; attempt <= 5; attempt++ {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/agent/nodes/heartbeat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Express233-Token", "invalid-fictional-token")
		response, requestErr := ts.Client().Do(req)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		_ = response.Body.Close()
		want := http.StatusUnauthorized
		if attempt == 5 {
			want = http.StatusTooManyRequests
		}
		if response.StatusCode != want {
			t.Fatalf("attempt %d status=%d want=%d", attempt, response.StatusCode, want)
		}
		if attempt == 5 && response.Header.Get("Retry-After") == "" {
			t.Fatal("blocked agent response requires Retry-After")
		}
	}
}

type heartbeatTestResponse struct {
	Desired deliveryDesiredResponse `json:"desired"`
}

func heartbeatDeliveryNode(t *testing.T, ts *httptest.Server, token string, body any) heartbeatTestResponse {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/agent/nodes/heartbeat", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Express233-Token", token)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status=%d", resp.StatusCode)
	}
	var result heartbeatTestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func putDeliveryDesired(t *testing.T, ts *httptest.Server, jar []*http.Cookie, path string, body any) {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range jar {
		req.AddCookie(cookie)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT %s status=%d", path, resp.StatusCode)
	}
}
