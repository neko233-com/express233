package api

import "net/http"

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleServerIDs(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	sf := s.getServerFile(tid)
	writeJSON(w, http.StatusOK, map[string]any{
		"server_ids": sf.ServerIDs(),
	})
}
