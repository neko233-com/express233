package api

import (
	"net/http"

	"github.com/neko233-com/express233/internal/compare"
)

// GET /api/deploy/diff?project=&from=&to=&server_id=
func (s *Server) handleDeployDiff(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	sess, _ := s.currentSession(r)
	s.serveDeployDiff(w, r, tid, sess.UserID)
}

func (s *Server) serveDeployDiff(w http.ResponseWriter, r *http.Request, tenantID, userID int64) {
	project := r.URL.Query().Get("project")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	serverID := r.URL.Query().Get("server_id")
	if project == "" || from == "" || to == "" || serverID == "" {
		errJSON(w, http.StatusBadRequest, "project, from, to, server_id required")
		return
	}
	p, err := s.Store.GetProjectByName(tenantID, project)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if !s.pullCanAccessProject(w, tenantID, userID, p.ID) {
		return
	}
	for _, ver := range []string{from, to} {
		if _, err := s.Store.GetVersion(p.ID, ver); err != nil {
			errJSON(w, http.StatusNotFound, "version not found: "+ver)
			return
		}
	}
	entry := s.getServerFile(tenantID).Entry(serverID)
	if entry == nil {
		errJSON(w, http.StatusNotFound, "server_id not in server.yaml")
		return
	}
	fromRoot, err := s.Store.VersionDir(tenantID, project, from)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	toRoot, err := s.Store.VersionDir(tenantID, project, to)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	report, err := compare.DiffVersions(fromRoot, toRoot, project, from, to, serverID, entry)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}
