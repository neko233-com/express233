package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web/*
var webFS embed.FS

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		http.Error(w, "static fs", 500)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || path == "/" {
		path = "index.html"
	}
	data, err := fs.ReadFile(sub, path)
	if err != nil {
		data, err = fs.ReadFile(sub, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	ctype := "text/html"
	switch {
	case strings.HasSuffix(path, ".js"):
		ctype = "application/javascript"
	case strings.HasSuffix(path, ".css"):
		ctype = "text/css"
	}
	w.Header().Set("Content-Type", ctype)
	_, _ = w.Write(data)
}
