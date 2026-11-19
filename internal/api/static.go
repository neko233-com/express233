package api

import (
	"bytes"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Embed the directory so Go-only tests and tools can compile from a clean checkout.
// The server build pipeline must generate web/dist before packaging.
//
//go:embed web
var webFS embed.FS

var webDir = resolveWebDir(os.Getenv("EXPRESS233_WEB_DIR"))

func resolveWebDir(dir string) string {
	if dir == "" {
		return ""
	}
	if absolute, err := filepath.Abs(dir); err == nil {
		return absolute
	}
	return dir
}

// DevWebDir returns the optional frontend build directory served from disk.
func DevWebDir() string { return webDir }

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if webDir != "" {
		root, err := os.OpenRoot(webDir)
		if err != nil {
			http.Error(w, "console build directory unavailable", http.StatusServiceUnavailable)
			return
		}
		defer func() { _ = root.Close() }()
		serveConsole(w, r, root.FS(), true)
		return
	}

	dist, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		http.Error(w, "console build unavailable", http.StatusServiceUnavailable)
		return
	}
	serveConsole(w, r, dist, false)
}

func serveConsole(w http.ResponseWriter, r *http.Request, assets fs.FS, dev bool) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	// Assets and API paths must not resolve to the SPA shell when absent.
	isAsset := strings.HasPrefix(name, "assets/") || name == "assets" || filepath.Ext(name) != ""
	if name == "api" || strings.HasPrefix(name, "api/") || strings.Contains(name, "\\") ||
		(name != "." && !fs.ValidPath(strings.TrimSuffix(name, "/"))) {
		http.NotFound(w, r)
		return
	}
	if !isAsset {
		name = "index.html"
	}
	data, err := fs.ReadFile(assets, name)
	if err != nil {
		if name == "index.html" {
			http.Error(w, "console assets missing: run npm --prefix frontend ci and npm --prefix frontend run build before building the server", http.StatusServiceUnavailable)
		} else {
			http.NotFound(w, r)
		}
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	switch filepath.Ext(name) {
	case ".html":
		contentType = "text/html"
	case ".js":
		contentType = "application/javascript"
	case ".css":
		contentType = "text/css"
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch {
	case dev:
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	case strings.HasPrefix(name, "assets/"):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
