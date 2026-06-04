package api

import (
	"net/http"

	"github.com/neko233-com/express233/internal/store"
)

func (s *Server) pullCanAccessProject(w http.ResponseWriter, tenantID, userID int64, projectID int64) bool {
	tenantRole, err := s.Store.UserRole(userID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return false
	}
	role, err := s.Store.ProjectAccess(projectID, userID, tenantRole)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return false
	}
	if role == "" {
		errJSON(w, http.StatusForbidden, "no access to project")
		return false
	}
	return true
}

func (s *Server) pullUserID(token string) (userID, tenantID int64, ok bool) {
	uid, tid, err := s.Store.LookupPullToken(token)
	if err != nil {
		return 0, 0, false
	}
	return uid, tid, true
}

// ensure pull uses store.CanWriteProject only for write paths — pull is read.
var _ = store.ProjectRoleViewer
