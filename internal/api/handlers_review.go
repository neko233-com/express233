package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

func (s *Server) handleSubmitReview(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if !store.CanWriteProject(pc.ProjectRole) {
		errJSON(w, http.StatusForbidden, "project admin required")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	report, err := s.Store.ValidateBeforePublish(tid, pname, pid, ver, s.getServerFile(tid))
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !report.OK {
		writeJSON(w, http.StatusBadRequest, report)
		return
	}
	if err := s.Store.SubmitVersionReview(pid, pname, ver); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "version.submit_review", "version="+ver)
	writeJSON(w, http.StatusOK, map[string]string{"status": "pending_review"})
}

func (s *Server) handleRejectReview(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if !store.CanWriteProject(pc.ProjectRole) {
		errJSON(w, http.StatusForbidden, "project admin required")
		return
	}
	if err := s.Store.RejectVersionReview(pid, ver); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "version.reject", "version="+ver)
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}
