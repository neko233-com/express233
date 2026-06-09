package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

func (s *Server) handleListProjectLogs(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || projectID <= 0 {
		errJSON(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if !s.pullCanAccessProject(w, sess.TenantID, sess.UserID, projectID) {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	logs, err := s.Store.ListProjectLogs(store.ProjectLogFilter{
		TenantID:  sess.TenantID,
		ProjectID: projectID,
		ServerID:  r.URL.Query().Get("server_id"),
		Version:   r.URL.Query().Get("version"),
		Limit:     limit,
	})
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}
