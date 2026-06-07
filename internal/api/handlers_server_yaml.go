package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/config"
	"gopkg.in/yaml.v3"
)

type serverEntryEnvelope struct {
	ServerID string             `json:"server_id"`
	Entry    config.ServerEntry `json:"entry"`
}

func (s *Server) tenantServerFile(tenantID int64) (*config.ServerFile, error) {
	path, err := s.Store.ServerYAMLPath(tenantID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &config.ServerFile{Servers: make(map[string]config.ServerEntry)}, nil
		}
		return nil, err
	}
	var sf config.ServerFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, err
	}
	if sf.Servers == nil {
		sf.Servers = make(map[string]config.ServerEntry)
	}
	return &sf, nil
}

func (s *Server) saveTenantServerFile(tenantID int64, sf *config.ServerFile) error {
	if sf == nil {
		sf = &config.ServerFile{}
	}
	if sf.Servers == nil {
		sf.Servers = make(map[string]config.ServerEntry)
	}
	if err := sf.Validate(); err != nil {
		return err
	}
	path, err := s.Store.ServerYAMLPath(tenantID)
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(sf)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	s.reloadServerYAML(tenantID)
	return nil
}

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

func (s *Server) handleListServerEntries(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	sf, err := s.tenantServerFile(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]serverEntryEnvelope, 0, len(sf.Servers))
	for _, sid := range sf.ServerIDs() {
		items = append(items, serverEntryEnvelope{ServerID: sid, Entry: sf.Servers[sid]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": items})
}

func (s *Server) handleGetServerEntry(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	serverID := chi.URLParam(r, "serverID")
	sf, err := s.tenantServerFile(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	entry, exists := sf.Servers[serverID]
	if !exists {
		errJSON(w, http.StatusNotFound, "server not found")
		return
	}
	writeJSON(w, http.StatusOK, serverEntryEnvelope{ServerID: serverID, Entry: entry})
}

func (s *Server) handlePutServerEntry(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	serverID := chi.URLParam(r, "serverID")
	if serverID == "" {
		errJSON(w, http.StatusBadRequest, "server_id required")
		return
	}
	var entry config.ServerEntry
	if err := readJSON(r, &entry); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	sf, err := s.tenantServerFile(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sf.Servers == nil {
		sf.Servers = make(map[string]config.ServerEntry)
	}
	sf.Servers[serverID] = entry
	if err := s.saveTenantServerFile(tid, sf); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "server.entry.upsert", "server_id="+serverID)
	writeJSON(w, http.StatusOK, serverEntryEnvelope{ServerID: serverID, Entry: entry})
}

func (s *Server) handleDeleteServerEntry(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	serverID := chi.URLParam(r, "serverID")
	sf, err := s.tenantServerFile(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, exists := sf.Servers[serverID]; !exists {
		errJSON(w, http.StatusNotFound, "server not found")
		return
	}
	delete(sf.Servers, serverID)
	if err := s.saveTenantServerFile(tid, sf); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "server.entry.delete", "server_id="+serverID)
	w.WriteHeader(http.StatusNoContent)
}

// handlePreviewReplacements 兼容旧 URL，返回完整 deploy preview。
func (s *Server) handlePreviewReplacements(w http.ResponseWriter, r *http.Request) {
	s.handleDeployPreview(w, r)
}
