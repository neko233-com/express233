package api

import (
	"net/http"
)

// requireMutableVersion makes the published-version contract explicit at every
// mutating HTTP endpoint. Every non-draft artifact is immutable
// so a rollback always refers to the exact bytes that were originally released.
func (s *Server) requireMutableVersion(w http.ResponseWriter, pc projectCtx, version string) bool {
	item, err := s.Store.GetVersion(pc.ProjectID, version)
	if err != nil {
		errJSON(w, http.StatusNotFound, "version not found")
		return false
	}
	if item.Status == "draft" {
		return true
	}
	errJSON(w, http.StatusConflict, "published or pending-review versions are immutable; create a new version for changes")
	return false
}
