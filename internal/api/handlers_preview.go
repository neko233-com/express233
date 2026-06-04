package api

import "net/http"

// handleDeployPreview Web 登录后预览（可预览草稿版本）。
// GET /api/deploy/preview?project=&version=&server_id=
func (s *Server) handleDeployPreview(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	version := r.URL.Query().Get("version")
	serverID := r.URL.Query().Get("server_id")
	if project == "" || version == "" || serverID == "" {
		errJSON(w, http.StatusBadRequest, "project, version, server_id required")
		return
	}
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	report, status, msg := s.buildDeployPreview(tid, project, version, serverID)
	if status != 0 {
		errJSON(w, status, msg)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
