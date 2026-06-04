package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestE2E_ReviewAndDiff(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")

	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "rev"})
	projects := mustGET[[]map[string]any](t, ts, jar, "/api/projects")
	var pid float64
	for _, p := range projects {
		if p["name"] == "rev" {
			pid = p["id"].(float64)
		}
	}
	pidStr := fmt.Sprintf("%d", int64(pid))
	mustPOST(t, ts, jar, "/api/projects/"+pidStr+"/versions", map[string]string{"name": "1.0.0"})
	mustPOST(t, ts, jar, "/api/projects/"+pidStr+"/versions", map[string]string{"name": "2.0.0"})
	uploadVersionFile(t, ts, jar, pidStr, "1.0.0", "app.properties", "port=8080\n")
	uploadVersionFile(t, ts, jar, pidStr, "2.0.0", "app.properties", "port=9090\n")

	_ = os.WriteFile(mustServerYAML(st), []byte(`servers:
  node-a:
    replacements:
      app.properties:
        port: "9001"
`), 0o644)
	srv.reloadServerYAML(1)

	mustPOST(t, ts, jar, "/api/projects/"+pidStr+"/versions/1.0.0/submit-review", nil)
	vers := mustGET[[]map[string]any](t, ts, jar, "/api/projects/"+pidStr+"/versions")
	var st10 string
	for _, v := range vers {
		if v["version"] == "1.0.0" {
			st10 = v["status"].(string)
		}
	}
	if st10 != "pending_review" {
		t.Fatalf("expected pending_review, got %q", st10)
	}
	mustPOST(t, ts, jar, "/api/projects/"+pidStr+"/versions/1.0.0/publish", nil)

	users, _ := st.ListUsers(1)
	token := users[0].Token
	diff := mustGETPublic(t, ts, "/api/pull/diff?token="+token+"&project=rev&from=1.0.0&to=2.0.0&server_id=node-a")
	if diff["from_version"] != "1.0.0" {
		t.Fatalf("diff: %v", diff)
	}
}

func uploadVersionFile(t *testing.T, ts *httptest.Server, jar []*http.Cookie, pid, ver, name, content string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write([]byte(content))
	_ = mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/"+pid+"/versions/"+ver+"/files", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("upload %s %d", ver, resp.StatusCode)
	}
}
