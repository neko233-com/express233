package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/docs/*
var docsFS embed.FS

func (s *Server) docsHandler() http.Handler {
	sub, err := fs.Sub(docsFS, "web/docs")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
