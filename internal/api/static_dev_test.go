package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestServeStaticFromDisk(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"index.html": "<html>dev</html>", "app.js": "window.__dev=1"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	old := webDir
	webDir = dir
	t.Cleanup(func() { webDir = old })

	s := &Server{}
	for _, tc := range []struct {
		path, body, mime string
	}{
		{"/", "<html>dev</html>", "text/html"},
		{"/projects/42/preview", "<html>dev</html>", "text/html"},
		{"/app.js", "window.__dev=1", "application/javascript"},
	} {
		rec := httptest.NewRecorder()
		s.handleStatic(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != tc.body || rec.Header().Get("Content-Type") != tc.mime {
			t.Fatalf("%s: status=%d body=%q content-type=%q", tc.path, rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
			t.Fatalf("%s: expected no-cache, got %q", tc.path, cc)
		}
	}
}

func TestConsoleAssetCachingAndMissingPaths(t *testing.T) {
	var assets fs.FS = fstest.MapFS{
		"index.html":    {Data: []byte("<html>console</html>")},
		"assets/app.js": {Data: []byte("export const app = 1;")},
	}
	for _, tc := range []struct {
		path, cache string
		code        int
	}{
		{"/projects/1/versions", "no-cache", http.StatusOK},
		{"/assets/app.js", "public, max-age=31536000, immutable", http.StatusOK},
		{"/assets/missing.js", "", http.StatusNotFound},
		{"/api/undefined", "", http.StatusNotFound},
		{"/../index.html", "", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		serveConsole(rec, httptest.NewRequest(http.MethodGet, tc.path, nil), assets, false)
		if rec.Code != tc.code || rec.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s: %d %q", tc.path, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
}
