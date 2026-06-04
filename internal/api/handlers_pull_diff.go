package api

import "net/http"

func (s *Server) handlePullDiff(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		errJSON(w, http.StatusBadRequest, "token required")
		return
	}
	uid, tid, ok := s.pullUserID(token)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "invalid token")
		return
	}
	s.serveDeployDiff(w, r, tid, uid)
}
