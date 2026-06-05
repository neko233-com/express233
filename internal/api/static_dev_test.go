package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeStaticFromDisk(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>dev</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("window.__dev=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := webDir
	webDir = dir
	t.Cleanup(func() { webDir = old })

	s := &Server{}
	for _, tc := range []struct {
		path     string
		wantBody string
		wantType string
	}{
		{"/", "<html>dev</html>", "text/html"},
		{"/app.js", "window.__dev=1", "application/javascript"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		s.handleStatic(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), tc.wantBody) {
			t.Fatalf("%s: body %q", tc.path, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); got != tc.wantType {
			t.Fatalf("%s: content-type %q", tc.path, got)
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
			t.Fatalf("%s: expected no-cache, got %q", tc.path, cc)
		}
	}
}

func TestSafeStaticFileRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	if _, err := safeStaticFile(dir, "../etc/passwd"); err == nil {
		t.Fatal("expected error for traversal")
	}
}
