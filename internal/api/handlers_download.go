package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleDownloadVersion 下载版本原始包（tar.gz，未做 server_id 替换）。
func (s *Server) handleDownloadVersion(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	v, err := s.Store.GetVersion(pid, ver)
	if err != nil {
		errJSON(w, http.StatusNotFound, "version not found")
		return
	}
	if v.Status != "published" {
		errJSON(w, http.StatusBadRequest, "only published versions can be downloaded")
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s-raw.tar.gz", pname, ver))
	if err := s.Store.StreamVersionArchive(tid, pname, ver, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.auditSession(r, "version.download", "project="+pname+" version="+ver)
}
