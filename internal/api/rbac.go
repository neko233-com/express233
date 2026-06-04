package api

import (
	"net/http"

	"github.com/neko233-com/express233/internal/store"
)

func (s *Server) sessionRole(r *http.Request) (string, bool) {
	sess, ok := s.currentSession(r)
	if !ok {
		return "", false
	}
	role, err := s.Store.UserRole(sess.UserID)
	if err != nil {
		return store.RoleViewer, true
	}
	return role, true
}

func (s *Server) requireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := s.sessionRole(r)
			if !ok {
				errJSON(w, http.StatusUnauthorized, "login required")
				return
			}
			if !allowed[role] && role != store.RoleAdmin {
				errJSON(w, http.StatusForbidden, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) requireMutator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		role, ok := s.sessionRole(r)
		if !ok {
			errJSON(w, http.StatusUnauthorized, "login required")
			return
		}
		if role == store.RoleViewer {
			errJSON(w, http.StatusForbidden, "viewer cannot modify")
			return
		}
		next.ServeHTTP(w, r)
	})
}
