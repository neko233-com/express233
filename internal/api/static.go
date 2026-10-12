package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed web/*
var webFS embed.FS

// webDir 非空时从磁盘读取 web 静态文件（开发热重载），由 EXPRESS233_WEB_DIR 设置。
var webDir string

func init() {
	if d := os.Getenv("EXPRESS233_WEB_DIR"); d != "" {
		if abs, err := filepath.Abs(d); err == nil {
			webDir = abs
		} else {
			webDir = d
		}
	}
}

// DevWebDir 返回开发静态目录；空表示使用 go:embed。
func DevWebDir() string {
	return webDir
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if webDir != "" {
		s.serveStaticFromDisk(w, r)
		return
	}
	s.serveStaticEmbedded(w, r)
}

func (s *Server) serveStaticEmbedded(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		http.Error(w, "static fs", 500)
		return
	}
	path := staticPath(r.URL.Path)
	data, err := fs.ReadFile(sub, path)
	if err != nil {
		data, err = fs.ReadFile(sub, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	writeStatic(w, path, data, false)
}

func (s *Server) serveStaticFromDisk(w http.ResponseWriter, r *http.Request) {
	path := staticPath(r.URL.Path)
	full, err := safeStaticFile(webDir, path)
	if err != nil {
		full, err = safeStaticFile(webDir, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		path = "index.html"
	}
	data, err := os.ReadFile(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeStatic(w, path, data, true)
}

func staticPath(urlPath string) string {
	path := strings.TrimPrefix(urlPath, "/")
	if path == "" || path == "/" {
		return "index.html"
	}
	return path
}

func safeStaticFile(root, name string) (string, error) {
	clean := filepath.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", os.ErrInvalid
	}
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", os.ErrInvalid
	}
	if _, err := os.Stat(full); err != nil {
		return "", err
	}
	return full, nil
}

func writeStatic(w http.ResponseWriter, path string, data []byte, dev bool) {
	ctype := "text/html"
	switch {
	case strings.HasSuffix(path, ".js"):
		ctype = "application/javascript"
	case strings.HasSuffix(path, ".css"):
		ctype = "text/css"
	}
	w.Header().Set("Content-Type", ctype)
	if dev {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	}
	_, _ = w.Write(data)
}
