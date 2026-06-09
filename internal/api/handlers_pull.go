package api

import (
	"net/http"
	"strings"

	"github.com/neko233-com/express233/internal/pull"
	"github.com/neko233-com/express233/internal/store"
)

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	project := r.URL.Query().Get("project")
	serverID := r.URL.Query().Get("server_id")
	version := r.URL.Query().Get("version")
	osName := r.URL.Query().Get("os")
	arch := r.URL.Query().Get("arch")
	extraTags := r.URL.Query()["tags"]

	if project == "" || serverID == "" {
		errJSON(w, http.StatusBadRequest, "project, server_id required")
		return
	}
	uid, tid, username, ok := s.pullIdentity(r, token)
	if !ok {
		metrics.pullErrors.Add(1)
		errJSON(w, http.StatusUnauthorized, "invalid pull credentials")
		return
	}

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
			s.recordPullProjectLog(r, tid, p.ID, username, project, version, serverID, osName, arch, extraTags, "error", "no published version")
			errJSON(w, http.StatusNotFound, "no published version")
			return
		}
	}
	v, err := s.Store.GetVersion(p.ID, version)
	if err != nil {
		s.recordPullProjectLog(r, tid, p.ID, username, project, version, serverID, osName, arch, extraTags, "error", "version not found")
		errJSON(w, http.StatusNotFound, "version not found")
		return
	}
	if v.Status != "published" {
		s.recordPullProjectLog(r, tid, p.ID, username, project, version, serverID, osName, arch, extraTags, "error", "version not published")
		errJSON(w, http.StatusBadRequest, "version not published")
		return
	}

	if s.getServerFile(tid).Entry(serverID) == nil {
		s.recordPullProjectLog(r, tid, p.ID, username, project, version, serverID, osName, arch, extraTags, "error", "server_id not configured")
		errJSON(w, http.StatusBadRequest, "server_id not configured in server.yaml")
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename="+project+"-"+version+".tar.gz")
	opts := pull.BundleOptions{OS: osName, Arch: arch, Tags: extraTags}
	if err := pull.BuildBundleWithOptions(s.Store, tid, s.getServerFile(tid), project, version, serverID, opts, w); err != nil {
		metrics.pullErrors.Add(1)
		s.recordPullProjectLog(r, tid, p.ID, username, project, version, serverID, osName, arch, extraTags, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	metrics.pullTotal.Add(1)
	s.recordPullProjectLog(r, tid, p.ID, username, project, version, serverID, osName, arch, extraTags, "ok", "")
	s.audit(r, username, "pull", "project="+project+" version="+version+" server_id="+serverID+" os="+osName+" arch="+arch)
}

func (s *Server) pullIdentity(r *http.Request, token string) (userID, tenantID int64, username string, ok bool) {
	if token != "" {
		uid, tid, ok := s.pullUserID(token)
		if !ok {
			return 0, 0, "", false
		}
		u, _ := s.Store.GetUserByID(uid)
		if u != nil {
			username = u.Username
		}
		return uid, tid, username, true
	}
	sess, ok := s.currentSession(r)
	if !ok {
		return 0, 0, "", false
	}
	return sess.UserID, sess.TenantID, sess.Username, true
}

func (s *Server) recordPullProjectLog(r *http.Request, tenantID, projectID int64, username, project, version, serverID, osName, arch string, tags []string, status, msg string) {
	_ = s.Store.RecordProjectLog(store.ProjectLog{
		TenantID:  tenantID,
		ProjectID: projectID,
		Project:   project,
		Username:  username,
		Action:    "pull",
		ServerID:  serverID,
		Version:   version,
		OS:        osName,
		Arch:      arch,
		Tags:      strings.Join(tags, ","),
		Status:    status,
		Error:     msg,
		IP:        r.RemoteAddr,
	})
}
