package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestReadVersionFileContent(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	setupProjectVersion(t, ts, jar, "file-preview")
	uploadFile2(t, ts, jar, 1, "1.0.0", "app.yaml", "server:\n  id: s1\n")

	status, body := getJSONMap(t, ts, jar, "/api/projects/1/versions/1.0.0/files/content?path="+url.QueryEscape("app.yaml"))
	if status != http.StatusOK {
		t.Fatalf("status = %d body = %+v", status, body)
	}
	if body["path"] != "app.yaml" || !strings.Contains(body["content"].(string), "id: s1") {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestReadVersionFileContentRejectsBinary(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	setupProjectVersion(t, ts, jar, "file-preview-bin")
	uploadFile2(t, ts, jar, 1, "1.0.0", "bin.dat", "a\x00b")

	status, body := getJSONMap(t, ts, jar, "/api/projects/1/versions/1.0.0/files/content?path=bin.dat")
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d body = %+v", status, body)
	}
	if !strings.Contains(body["error"].(string), "binary") {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func getJSONMap(t *testing.T, ts *httptest.Server, jar []*http.Cookie, path string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}
