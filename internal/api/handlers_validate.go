package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleValidateVersion(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	if _, err := s.Store.GetVersion(pid, ver); err != nil {
		errJSON(w, http.StatusNotFound, "version not found")
		return
	}
	report, err := s.Store.ValidateBeforePublish(tid, pname, pid, ver, s.getServerFile(tid))
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}
