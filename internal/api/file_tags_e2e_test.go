package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/pull"
	"github.com/neko233-com/express233/internal/store"
)

func TestFileTagsFilterPullBundle(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()
	jar := login(t, ts, "root", "root")

	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "platforms"})
	projects := mustGET[[]map[string]any](t, ts, jar, "/api/projects")
	pid := int(projects[0]["id"].(float64))
	mustPOST(t, ts, jar, "/api/projects/"+itoa(pid)+"/versions", map[string]string{"name": "1.0.0"})
	uploadTaggedFile(t, ts, jar, pid, "1.0.0", "README.txt", "common", "")
	uploadTaggedFile(t, ts, jar, pid, "1.0.0", "bin/server-linux-amd64", "linux", "linux-amd64")
	uploadTaggedFile(t, ts, jar, pid, "1.0.0", "bin/server-windows-amd64.exe", "windows", "windows-amd64")

	reqBody, _ := json.Marshal(map[string]any{
		"patterns": []string{"bin/server-linux-*"},
		"mode":     "add",
		"tags":     []string{"linux"},
	})
	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/"+itoa(pid)+"/versions/1.0.0/file-tags/batch", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch tags %d", resp.StatusCode)
	}

	_ = os.WriteFile(mustServerYAML(st), []byte("servers:\n  s1:\n    replacements: {}\n"), 0o644)
	srv.reloadServerYAML(1)
	mustPOST(t, ts, jar, "/api/projects/"+itoa(pid)+"/versions/1.0.0/publish", nil)
	users, _ := st.ListUsers(1)
	token := users[0].Token

	linux := pullBundleFiles(t, ts.URL+"/api/pull?token="+token+"&project=platforms&version=1.0.0&server_id=s1&os=linux&arch=amd64")
	if !linux["README.txt"] || !linux["bin/server-linux-amd64"] || linux["bin/server-windows-amd64.exe"] {
		t.Fatalf("linux files: %+v", linux)
	}
	win := pullBundleFiles(t, ts.URL+"/api/pull?token="+token+"&project=platforms&version=1.0.0&server_id=s1&os=windows&arch=amd64")
	if !win["README.txt"] || !win["bin/server-windows-amd64.exe"] || win["bin/server-linux-amd64"] {
		t.Fatalf("windows files: %+v", win)
	}
}

func uploadTaggedFile(t *testing.T, ts *httptest.Server, jar []*http.Cookie, pid int, ver, path, content, tags string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", path)
	_, _ = fw.Write([]byte(content))
	_ = mw.WriteField("path", path)
	if tags != "" {
		_ = mw.WriteField("tags", tags)
	}
	_ = mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/"+itoa(pid)+"/versions/"+ver+"/files", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload %s %d", path, resp.StatusCode)
	}
}

func pullBundleFiles(t *testing.T, url string) map[string]bool {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pull %d", resp.StatusCode)
	}
	dest := t.TempDir()
	if _, err := pull.ExtractBundle(resp.Body, dest); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(prefix string, names []os.DirEntry)
	walk = func(prefix string, names []os.DirEntry) {
		for _, e := range names {
			path := strings.TrimPrefix(prefix+"/"+e.Name(), "/")
			if e.IsDir() {
				sub, _ := os.ReadDir(filepath.Join(dest, filepath.FromSlash(path)))
				walk(path, sub)
				continue
			}
			out[path] = true
		}
	}
	walk("", entries)
	delete(out, ".express233/manifest.json")
	return out
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
