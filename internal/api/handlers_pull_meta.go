package api

import "net/http"

// handlePullServerIDs 使用拉取 token 列出 server.yaml 中的 server_id。
func (s *Server) handlePullServerIDs(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		errJSON(w, http.StatusBadRequest, "token required")
		return
	}
	tid, _, ok := s.pullTenant(token)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "invalid token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"server_ids": s.getServerFile(tid).ServerIDs(),
	})
}
