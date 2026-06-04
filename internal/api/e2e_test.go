package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestE2E_PullPreviewAndServerIDs(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")

	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "e2e"})
	projects := mustGET[[]map[string]any](t, ts, jar, "/api/projects")
	var pid float64
	for _, p := range projects {
		if p["name"] == "e2e" {
			pid = p["id"].(float64)
		}
	}
	pidStr := fmt.Sprintf("%d", int64(pid))
	mustPOST(t, ts, jar, "/api/projects/"+pidStr+"/versions", map[string]string{"name": "1.0.0"})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "app.properties")
	_, _ = fw.Write([]byte("port=8080\n"))
	_ = mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/"+pidStr+"/versions/1.0.0/files", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Fatalf("upload %d", resp.StatusCode)
	}
	resp.Body.Close()

	_ = os.WriteFile(mustServerYAML(st), []byte(`servers:
  node-a:
    replacements:
      app.properties:
        port: "9001"
    post_hook: scripts/restart-{{SERVER_ID}}.sh
    post_hook_env:
      INSTANCE: "a"
`), 0o644)
	srv.reloadServerYAML(1)

	ids := mustGET[map[string]any](t, ts, jar, "/api/server-ids")
	if len(ids["server_ids"].([]any)) != 1 {
		t.Fatalf("server_ids: %v", ids)
	}

	prev := mustGET[map[string]any](t, ts, jar, "/api/deploy/preview?project=e2e&version=1.0.0&server_id=node-a")
	if prev["post_hook"] != "scripts/restart-node-a.sh" {
		t.Fatalf("post_hook template: %v", prev["post_hook"])
	}

	mustPOST(t, ts, jar, "/api/projects/"+pidStr+"/versions/1.0.0/publish", nil)

	users, _ := st.ListUsers(1)
	token := users[0].Token

	pullPrev := mustGETPublic(t, ts, "/api/pull/preview?token="+token+"&project=e2e&version=1.0.0&server_id=node-a")
	files := pullPrev["files"].([]any)
	if len(files) == 0 {
		t.Fatal("no preview files")
	}

	pullIDs := mustGETPublic(t, ts, "/api/pull/server-ids?token="+token)
	if len(pullIDs["server_ids"].([]any)) != 1 {
		t.Fatal("pull server-ids")
	}

	vers := mustGETPublic(t, ts, "/api/pull/versions?token="+token+"&project=e2e")
	if len(vers["versions"].([]any)) != 1 {
		t.Fatalf("pull versions: %v", vers)
	}

	resp, err = http.Get(ts.URL + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("healthz %v %d", err, resp.StatusCode)
	}
	resp.Body.Close()
}

func login(t *testing.T, ts *httptest.Server, user, pass string) []*http.Cookie {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := http.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login %d", resp.StatusCode)
	}
	return resp.Cookies()
}

func mustPOST(t *testing.T, ts *httptest.Server, jar []*http.Cookie, path string, body any) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest("POST", ts.URL+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("POST %s %d", path, resp.StatusCode)
	}
}

func mustGET[T any](t *testing.T, ts *httptest.Server, jar []*http.Cookie, path string) T {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+path, nil)
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s %d", path, resp.StatusCode)
	}
	var v T
	_ = json.NewDecoder(resp.Body).Decode(&v)
	return v
}

func mustGETPublic(t *testing.T, ts *httptest.Server, path string) map[string]any {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s %d", path, resp.StatusCode)
	}
	var v map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&v)
	return v
}
