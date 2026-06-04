package api

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

type projectCtx struct {
	TenantID    int64
	ProjectID   int64
	ProjectName string
	ProjectRole string
}

func (s *Server) projectCtxFromRequest(r *http.Request, projectID int64) (projectCtx, bool) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		return projectCtx{}, false
	}
	sess, ok := s.currentSession(r)
	if !ok {
		return projectCtx{}, false
	}
	tenantRole, _ := s.Store.UserRole(sess.UserID)
	name, err := s.Store.ProjectNameInTenant(tid, projectID)
	if err != nil {
		return projectCtx{}, false
	}
	pRole, err := s.Store.ProjectAccess(projectID, sess.UserID, tenantRole)
	if err != nil || pRole == "" {
		return projectCtx{}, false
	}
	return projectCtx{
		TenantID: tid, ProjectID: projectID, ProjectName: name, ProjectRole: pRole,
	}, true
}

func (s *Server) projectByID(r *http.Request, id int64) (projectCtx, error) {
	pc, ok := s.projectCtxFromRequest(r, id)
	if !ok {
		return projectCtx{}, os.ErrNotExist
	}
	return pc, nil
}

func (s *Server) requireProjectWriter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pid, _ := strconvParseInt(chi.URLParam(r, "id"))
		pc, ok := s.projectCtxFromRequest(r, pid)
		if !ok {
			errJSON(w, http.StatusNotFound, "project not found")
			return
		}
		if !store.CanWriteProject(pc.ProjectRole) {
			errJSON(w, http.StatusForbidden, "project admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
