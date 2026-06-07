package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestE2E_BasicAuthServerCRUD(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	putReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/servers/s1", bytes.NewReader(mustJSON(t, map[string]any{
		"replacements": map[string]any{
			"game.properties": map[string]any{
				"server.port": "9001",
			},
		},
		"post_hook": "scripts/restart-{{SERVER_ID}}.sh",
		"post_hook_env": map[string]string{
			"ZONE": "demo",
		},
	})))
	putReq.Header.Set("Content-Type", "application/json")
	setBasicRootAuth(putReq)
	resp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put server %d", resp.StatusCode)
	}

	projectsReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/projects", bytes.NewReader(mustJSON(t, map[string]string{"name": "demo"})))
	projectsReq.Header.Set("Content-Type", "application/json")
	setBasicRootAuth(projectsReq)
	resp, err = http.DefaultClient.Do(projectsReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project %d", resp.StatusCode)
	}

	listReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/servers", nil)
	setBasicRootAuth(listReq)
	resp, err = http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list servers %d", resp.StatusCode)
	}
	var listed struct {
		Servers []struct {
			ServerID string `json:"server_id"`
			Entry    struct {
				PostHook string `json:"post_hook"`
			} `json:"entry"`
		} `json:"servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Servers) != 1 || listed.Servers[0].ServerID != "s1" || listed.Servers[0].Entry.PostHook != "scripts/restart-{{SERVER_ID}}.sh" {
		t.Fatalf("unexpected servers: %+v", listed)
	}

	getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/server-ids", nil)
	setBasicRootAuth(getReq)
	resp, err = http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("server ids %d", resp.StatusCode)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/servers/s1", nil)
	setBasicRootAuth(delReq)
	resp, err = http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete server %d", resp.StatusCode)
	}

	missingReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/servers/s1", nil)
	setBasicRootAuth(missingReq)
	resp, err = http.DefaultClient.Do(missingReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing server %d", resp.StatusCode)
	}
}

func setBasicRootAuth(r *http.Request) {
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("root:root")))
}