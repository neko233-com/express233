package api

import (
	"net/http"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := readJSON(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	uid, admin, err := s.Store.Authenticate(req.Username, req.Password)
	if err != nil {
		errJSON(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	metrics.loginTotal.Add(1)
	s.audit(r, req.Username, "login", "success")
	tid, err := s.Store.UserTenantID(uid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "tenant error")
		return
	}
	t, err := s.Store.TenantByID(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "tenant error")
		return
	}
	sid, err := s.sessions.create(uid, req.Username, admin, tid, t.Slug)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "session error")
		return
	}
	s.setSessionCookie(w, sid)
	s.reloadServerYAML(tid)
	writeJSON(w, http.StatusOK, s.mePayload(session{UserID: uid, Username: req.Username, IsAdmin: admin, TenantID: tid, TenantSlug: t.Slug}))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.delete(c.Value)
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "not logged in")
		return
	}
	writeJSON(w, http.StatusOK, s.mePayload(sess))
}

func (s *Server) requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.currentSession(r); !ok {
			errJSON(w, http.StatusUnauthorized, "login required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.currentSession(r)
		if !ok {
			errJSON(w, http.StatusUnauthorized, "login required")
			return
		}
		if !sess.IsAdmin {
			errJSON(w, http.StatusForbidden, "admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
