package api

import "net/http"

func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		return x
	}
	if x := r.Header.Get("X-Real-Ip"); x != "" {
		return x
	}
	return r.RemoteAddr
}

func (s *Server) audit(r *http.Request, username, action, detail string) {
	_ = s.Store.RecordAudit(username, action, detail, clientIP(r))
}

func (s *Server) auditSession(r *http.Request, action, detail string) {
	if sess, ok := s.currentSession(r); ok {
		s.audit(r, sess.Username, action, detail)
	}
}

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.Store.ListAuditLogs(200)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}
