package api

import (
	"net/http"

	"github.com/neko233-com/express233/internal/pull"
)

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	project := r.URL.Query().Get("project")
	serverID := r.URL.Query().Get("server_id")
	version := r.URL.Query().Get("version")
	osName := r.URL.Query().Get("os")
	arch := r.URL.Query().Get("arch")
	extraTags := r.URL.Query()["tags"]

	if token == "" || project == "" || serverID == "" {
		errJSON(w, http.StatusBadRequest, "token, project, server_id required")
		return
	}
	uid, tid, ok := s.pullUserID(token)
	if !ok {
		metrics.pullErrors.Add(1)
		errJSON(w, http.StatusUnauthorized, "invalid token")
		return
	}
	user, _ := s.Store.GetUserByID(uid)

	p, err := s.Store.GetProjectByName(tid, project)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if !s.pullCanAccessProject(w, tid, uid, p.ID) {
		return
	}
	if version == "" {
		version, err = s.Store.LatestPublishedVersion(p.ID)
		if err != nil {
			errJSON(w, http.StatusNotFound, "no published version")
			return
		}
	}
	v, err := s.Store.GetVersion(p.ID, version)
	if err != nil {
		errJSON(w, http.StatusNotFound, "version not found")
		return
	}
	if v.Status != "published" {
		errJSON(w, http.StatusBadRequest, "version not published")
		return
	}

	if s.getServerFile(tid).Entry(serverID) == nil {
		errJSON(w, http.StatusBadRequest, "server_id not configured in server.yaml")
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename="+project+"-"+version+".tar.gz")
	opts := pull.BundleOptions{OS: osName, Arch: arch, Tags: extraTags}
	if err := pull.BuildBundleWithOptions(s.Store, tid, s.getServerFile(tid), project, version, serverID, opts, w); err != nil {
		metrics.pullErrors.Add(1)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	metrics.pullTotal.Add(1)
	uname := ""
	if user != nil {
		uname = user.Username
	}
	s.audit(r, uname, "pull", "project="+project+" version="+version+" server_id="+serverID+" os="+osName+" arch="+arch)
}
