package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestE2E_ProjectInvite(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")

	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "shared"})
	projects := mustGET[[]map[string]any](t, ts, jar, "/api/projects")
	var pid float64
	for _, p := range projects {
		if p["name"] == "shared" {
			pid = p["id"].(float64)
		}
	}
	pidStr := fmt.Sprintf("%d", int64(pid))

	invBody, _ := json.Marshal(map[string]any{"role": "viewer", "valid_hours": 24})
	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/"+pidStr+"/invites", bytes.NewReader(invBody))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var invResp map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&invResp)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create invite %d", resp.StatusCode)
	}
	invObj := invResp["invite"].(map[string]any)
	token := invObj["token"].(string)

	if _, err := st.CreateUser(1, "guest", "guest", store.RoleViewer, false); err != nil {
		t.Fatal(err)
	}
	guestJar := login(t, ts, "guest", "guest")

	prev := mustGET[map[string]any](t, ts, guestJar, "/api/project-invites/"+token)
	if prev["project_name"] != "shared" {
		t.Fatalf("preview: %v", prev)
	}

	accReq, _ := http.NewRequest("POST", ts.URL+"/api/project-invites/"+token+"/accept", nil)
	for _, c := range guestJar {
		accReq.AddCookie(c)
	}
	accResp, err := http.DefaultClient.Do(accReq)
	if err != nil {
		t.Fatal(err)
	}
	var acc map[string]any
	_ = json.NewDecoder(accResp.Body).Decode(&acc)
	accResp.Body.Close()
	if acc["name"] != "shared" {
		t.Fatalf("accept: %v", acc)
	}

	list := mustGET[[]map[string]any](t, ts, guestJar, "/api/projects")
	found := false
	for _, p := range list {
		if p["name"] == "shared" {
			found = true
		}
	}
	if !found {
		t.Fatal("guest should see shared project")
	}

	verBody, _ := json.Marshal(map[string]string{"name": "9.9.9"})
	verReq, _ := http.NewRequest("POST", ts.URL+"/api/projects/"+pidStr+"/versions", bytes.NewReader(verBody))
	verReq.Header.Set("Content-Type", "application/json")
	for _, c := range guestJar {
		verReq.AddCookie(c)
	}
	verResp, _ := http.DefaultClient.Do(verReq)
	if verResp.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer create version expected 403, got %d", verResp.StatusCode)
	}
	verResp.Body.Close()
}
