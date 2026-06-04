package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/neko233-com/express233/internal/config"
	"gopkg.in/yaml.v3"
)

func (s *Server) handleGetServerYAML(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	path, err := s.Store.ServerYAMLPath(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]string{"content": "servers: {}\n"})
			return
		}
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": string(data)})
}

func (s *Server) handlePutServerYAML(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := readJSON(r, &body); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	var sf config.ServerFile
	if err := yaml.Unmarshal([]byte(body.Content), &sf); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid yaml: "+err.Error())
		return
	}
	if err := sf.Validate(); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	path, err := s.Store.ServerYAMLPath(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(body.Content), 0o644); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.reloadServerYAML(tid)
	s.auditSession(r, "server_yaml.update", "bytes="+fmt.Sprint(len(body.Content)))
	w.WriteHeader(http.StatusNoContent)
}

// handlePreviewReplacements 兼容旧 URL，返回完整 deploy preview。
func (s *Server) handlePreviewReplacements(w http.ResponseWriter, r *http.Request) {
	s.handleDeployPreview(w, r)
}
