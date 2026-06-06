package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

// ── helpers ───────────────────────────────────────────────────────────

func doGet2(t *testing.T, ts *httptest.Server, jar []*http.Cookie, path string) (int, map[string]any) {
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
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func doPost2(t *testing.T, ts *httptest.Server, jar []*http.Cookie, path string, payload any) (int, map[string]any) {
	t.Helper()
	var r io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		r = bytes.NewReader(b)
	} else {
		r = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest("POST", ts.URL+path, r)
	if payload != nil {
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
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func doPut2(t *testing.T, ts *httptest.Server, jar []*http.Cookie, path string, payload any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", ts.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func uploadFile2(t *testing.T, ts *httptest.Server, jar []*http.Cookie, pid int64, ver, name, content string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", name)
	_, _ = fw.Write([]byte(content))
	_ = w.WriteField("path", name)
	w.Close()
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("%s/api/projects/%d/versions/%s/files", ts.URL, pid, ver), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("upload got %d", resp.StatusCode)
	}
}

func setupProjectVersion(t *testing.T, ts *httptest.Server, jar []*http.Cookie, projName string) {
	t.Helper()
	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": projName})
	mustPOST(t, ts, jar, "/api/projects/1/versions", map[string]string{"name": "1.0.0"})
}

// ── Tests ─────────────────────────────────────────────────────────────

func TestErr_DeployPreview_UnknownServerID_Returns404(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	setupProjectVersion(t, ts, jar, "err-sid")

	status, body := doGet2(t, ts, jar,
		"/api/deploy/preview?project=err-sid&version=1.0.0&server_id=ghost-server")
	if status != http.StatusNotFound {
		t.Fatalf("want 404, got %d", status)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "server_id") {
		t.Fatalf("error should mention server_id, got %q", msg)
	}
}

func TestErr_DeployPreview_MissingParams(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")

	for _, tc := range []struct{ name, url string }{
		{"all missing", "/api/deploy/preview"},
		{"no server_id", "/api/deploy/preview?project=x&version=1"},
		{"no project", "/api/deploy/preview?version=1&server_id=s"},
		{"no version", "/api/deploy/preview?project=x&server_id=s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := doGet2(t, ts, jar, tc.url)
			if status != http.StatusBadRequest {
				t.Fatalf("want 400, got %d", status)
			}
		})
	}
}

func TestErr_Login_WrongPassword(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	status, body := doPost2(t, ts, nil, "/api/login",
		map[string]string{"username": "root", "password": "bad"})
	if status != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", status)
	}
	if _, ok := body["error"]; !ok {
		t.Fatal("want error in body")
	}
}

func TestErr_Login_EmptyUsername(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	// 空用户名被当作无效凭证，返回 401
	status, _ := doPost2(t, ts, nil, "/api/login",
		map[string]string{"username": "", "password": "root"})
	if status != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", status)
	}
}

func TestErr_CreateProject_EmptyName(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	status, _ := doPost2(t, ts, jar, "/api/projects", map[string]string{"name": ""})
	if status != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", status)
	}
}

func TestErr_ValidateBeforePublish_EmptyVersion(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	setupProjectVersion(t, ts, jar, "empty-ver")

	// Validate with no files → should return ok=false
	status, body := doGet2(t, ts, jar, "/api/projects/1/versions/1.0.0/validate")
	if status != 200 {
		t.Fatalf("want 200, got %d", status)
	}
	if ok, _ := body["ok"].(bool); ok {
		t.Fatal("want ok=false for version with no files")
	}
}

func TestDeletePublishedVersion_Allowed(t *testing.T) {
	// 已发布版本允许删除，用于节省磁盘空间。
	// 如果丢失了，需要重新上传同步。
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	setupProjectVersion(t, ts, jar, "del-pub")
	uploadFile2(t, ts, jar, 1, "1.0.0", "app.properties", "k=v\n")

	mustPOST(t, ts, jar, "/api/projects/1/versions/1.0.0/publish", nil)

	// 删除已发布版本 — 允许（204）
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/projects/1/versions/1.0.0?confirm=yes", nil)
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	// 删除后文件列表应为空
	status, body := doGet2(t, ts, jar, "/api/projects/1/versions/1.0.0/files")
	if status != 200 {
		t.Fatalf("want 200 listing files after delete, got %d", status)
	}
	files, _ := body["files"].([]any)
	if len(files) != 0 {
		t.Fatalf("want 0 files after delete, got %d", len(files))
	}
}

func TestErr_ViewerCannotCreateVersion(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "viewer-proj"})

	// Create viewer user
	doPost2(t, ts, jar, "/api/users", map[string]any{
		"username": "v1", "password": "v1", "role": "viewer",
	})
	viewerJar := login(t, ts, "v1", "v1")

	// Viewer tries to create version — should be rejected
	status, _ := doPost2(t, ts, viewerJar, "/api/projects/1/versions",
		map[string]string{"name": "1.0.0"})
	if status < 400 {
		t.Fatalf("viewer should not create versions, got %d", status)
	}
}

func TestErr_UnauthorizedAccess(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	// Access API without login
	status, _ := doGet2(t, ts, nil, "/api/projects")
	if status != http.StatusUnauthorized {
		t.Fatalf("want 401 without login, got %d", status)
	}
}

func TestServerYAML_RoundTrip(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")

	yaml := "servers:\n  alpha:\n    replacements: {}\n  beta:\n    replacements: {}\n"
	status, _ := doPut2(t, ts, jar, "/api/server-yaml", map[string]string{"content": yaml})
	if status != 200 && status != 204 {
		t.Fatalf("save want 200/204, got %d", status)
	}

	status, body := doGet2(t, ts, jar, "/api/server-yaml")
	if status != 200 {
		t.Fatalf("get want 200, got %d", status)
	}
	if c, _ := body["content"].(string); !strings.Contains(c, "alpha") {
		t.Fatal("content should contain alpha")
	}

	// Server IDs
	status, body = doGet2(t, ts, jar, "/api/server-ids")
	if status != 200 {
		t.Fatalf("ids want 200, got %d", status)
	}
	ids, _ := body["server_ids"].([]any)
	if len(ids) != 2 {
		t.Fatalf("want 2 server_ids, got %d", len(ids))
	}
}

func TestServerYAML_PreviewUnknownID(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")

	yaml := "servers:\n  known-srv:\n    replacements: {}\n"
	doPut2(t, ts, jar, "/api/server-yaml", map[string]string{"content": yaml})

	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "prev-proj"})
	mustPOST(t, ts, jar, "/api/projects/1/versions", map[string]string{"name": "1.0.0"})

	// Preview with unknown server_id via deploy/preview endpoint
	status, body := doGet2(t, ts, jar,
		"/api/deploy/preview?project=prev-proj&version=1.0.0&server_id=unknown-srv")
	if status != http.StatusNotFound {
		t.Fatalf("want 404 for unknown server_id, got %d", status)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "server_id") {
		t.Fatalf("error should mention server_id, got %q", msg)
	}

	// Preview with known server_id — should not be 404
	// (might be 500 if no files, but NOT 404)
	status, _ = doGet2(t, ts, jar,
		"/api/deploy/preview?project=prev-proj&version=1.0.0&server_id=known-srv")
	if status == http.StatusNotFound {
		t.Fatal("known server_id should not return 404")
	}
}
