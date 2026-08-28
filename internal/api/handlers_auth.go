package api

import (
	"fmt"
	"net/http"
	"strconv"
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
	if s.rejectBlockedLogin(w, r, req.Username) {
		return
	}
	uid, admin, err := s.Store.Authenticate(req.Username, req.Password)
	if err != nil {
		banned, retry, remaining := s.recordLoginFailure(r, req.Username)
		if banned {
			w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retry.Seconds()))))
			errJSON(w, http.StatusTooManyRequests, "too many login attempts; try again later")
			return
		}
		if !s.loginProtectionEnabled() {
			errJSON(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		errJSON(w, http.StatusUnauthorized, fmt.Sprintf("invalid username or password; %d attempt(s) remaining before this IP is temporarily blocked", remaining))
		return
	}
	s.clearLoginFailures(r)
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
	authVersion, err := s.Store.UserAuthVersion(uid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "authentication state error")
		return
	}
	sess := session{UserID: uid, Username: req.Username, IsAdmin: admin, AuthVersion: authVersion, TenantID: tid, TenantSlug: t.Slug}
	tok, err := s.jwt.sign(sess, persistentSessionTTL)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "token error")
		return
	}
	s.setJWTCookie(w, tok)
	// 兼容依赖 cookie jar 的测试
	if sid, err := s.sessions.create(uid, req.Username, admin, authVersion, tid, t.Slug); err == nil {
		s.setSessionCookie(w, sid)
	}
	s.reloadServerYAML(tid)
	writeJSON(w, http.StatusOK, s.mePayload(sess, tok))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.delete(c.Value)
	}
	s.clearSessionCookie(w)
	s.clearJWTCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "not logged in")
		return
	}
	// 活跃浏览器滑动续签 30 天；密码变更会递增 auth_version，使旧 token 立即失效。
	tok, err := s.jwt.sign(sess, persistentSessionTTL)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "token error")
		return
	}
	s.setJWTCookie(w, tok)
	writeJSON(w, http.StatusOK, s.mePayload(sess, tok))
}

func (s *Server) requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, hasBasic := r.BasicAuth(); hasBasic && s.rejectBlockedLogin(w, r, "") {
			return
		}
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
