package api

import (
	"net/http"

	"github.com/neko233-com/express233/internal/version"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	sess, _ := s.currentSession(r)
	tenantRole, _ := s.Store.UserRole(sess.UserID)
	projects, _ := s.Store.ListProjects(tid, sess.UserID, tenantRole)
	serverIDs := s.getServerFile(tid).ServerIDs()
	writeJSON(w, http.StatusOK, map[string]any{
		"server": map[string]string{
			"version": version.Version,
			"commit":  version.Commit,
		},
		"projects_count":   len(projects),
		"server_ids_count": len(serverIDs),
		"metrics": map[string]uint64{
			"pull_total":    metrics.pullTotal.Load(),
			"pull_errors":   metrics.pullErrors.Load(),
			"preview_total": metrics.previewTotal.Load(),
			"publish_total": metrics.publishTotal.Load(),
		},
	})
}
