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

func TestAPIFlow(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()
	c := ts.Client()

	loginBody, _ := json.Marshal(map[string]string{"username": "root", "password": "root"})
	resp, err := c.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("login %d", resp.StatusCode)
	}
	jar := resp.Cookies()
	resp.Body.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/projects", bytes.NewReader(mustJSON(t, map[string]string{"name": "game1"})))
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range jar {
		req.AddCookie(ck)
	}
	resp, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var proj struct {
		ID int64 `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&proj)
	resp.Body.Close()

	req, _ = http.NewRequest("POST", ts.URL+fmt.Sprintf("/api/projects/%d/versions", proj.ID), bytes.NewReader(mustJSON(t, map[string]string{"name": "1.0.0"})))
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range jar {
		req.AddCookie(ck)
	}
	resp, _ = c.Do(req)
	resp.Body.Close()

	var mbuf bytes.Buffer
	mw := multipart.NewWriter(&mbuf)
	fw, _ := mw.CreateFormFile("file", "game.properties")
	_, _ = fw.Write([]byte("server.port=1\n"))
	_ = mw.Close()
	req, _ = http.NewRequest("POST", ts.URL+fmt.Sprintf("/api/projects/%d/versions/1.0.0/files", proj.ID), &mbuf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, ck := range jar {
		req.AddCookie(ck)
	}
	resp, _ = c.Do(req)
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload %d %s", resp.StatusCode, b)
	}
	resp.Body.Close()

	_ = os.WriteFile(mustServerYAML(st), []byte(`servers:
  s1:
    replacements:
      game.properties:
        server.port: "9001"
    post_hook: restart.sh
`), 0o644)
	srv.reloadServerYAML(1)

	// preview before publish (draft)
	req, _ = http.NewRequest("GET", ts.URL+"/api/deploy/preview?project=game1&version=1.0.0&server_id=s1", nil)
	for _, ck := range jar {
		req.AddCookie(ck)
	}
	resp, _ = c.Do(req)
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("preview %d %s", resp.StatusCode, b)
	}
	var prev struct {
		Files []struct {
			Changes []struct {
				Key    string `json:"key"`
				Before string `json:"before"`
				After  string `json:"after"`
			} `json:"changes"`
		} `json:"files"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&prev)
	resp.Body.Close()
	if len(prev.Files) == 0 || prev.Files[0].Changes[0].After != "9001" {
		t.Fatalf("preview: %+v", prev)
	}

	req, _ = http.NewRequest("POST", ts.URL+fmt.Sprintf("/api/projects/%d/versions/1.0.0/publish", proj.ID), nil)
	for _, ck := range jar {
		req.AddCookie(ck)
	}
	resp, _ = c.Do(req)
	resp.Body.Close()

	users, _ := st.ListUsers(1)
	token := users[0].Token

	pullURL := ts.URL + "/api/pull?token=" + token + "&project=game1&server_id=s1"
	resp, err = http.Get(pullURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("pull %d %s", resp.StatusCode, b)
	}
	resp.Body.Close()

	prevURL := ts.URL + "/api/pull/preview?token=" + token + "&project=game1&version=1.0.0&server_id=s1"
	resp, err = http.Get(prevURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("pull preview %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
