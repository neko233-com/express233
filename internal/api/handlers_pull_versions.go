package api

import "net/http"

// handlePullVersions 列出项目下已发布版本（节点 token）。
func (s *Server) handlePullVersions(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	project := r.URL.Query().Get("project")
	if token == "" || project == "" {
		errJSON(w, http.StatusBadRequest, "token, project required")
		return
	}
	uid, tid, ok := s.pullUserID(token)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "invalid token")
		return
	}
	p, err := s.Store.GetProjectByName(tid, project)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if !s.pullCanAccessProject(w, tid, uid, p.ID) {
		return
	}
	versions, err := s.Store.ListPublishedVersions(p.ID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project":  project,
		"versions": versions,
	})
}
