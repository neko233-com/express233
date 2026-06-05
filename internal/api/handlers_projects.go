package api

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

type nameReq struct {
	Name string `json:"name"`
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	sess, _ := s.currentSession(r)
	tenantRole, _ := s.Store.UserRole(sess.UserID)
	list, err := s.Store.ListProjects(tid, sess.UserID, tenantRole)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req nameReq
	if err := readJSON(r, &req); err != nil || req.Name == "" {
		errJSON(w, http.StatusBadRequest, "name required")
		return
	}
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	sess, _ := s.currentSession(r)
	p, err := s.Store.CreateProject(tid, sess.UserID, req.Name)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "project.create", "name="+req.Name)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	pc, err := s.projectByID(r, id)
	if err != nil || !store.CanWriteProject(pc.ProjectRole) {
		errJSON(w, http.StatusForbidden, "project admin required")
		return
	}
	if err := s.Store.DeleteProject(tid, id); err != nil {
		errJSON(w, http.StatusNotFound, "not found")
		return
	}
	s.auditSession(r, "project.delete", "id="+chi.URLParam(r, "id"))
	w.WriteHeader(http.StatusNoContent)
}


func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if _, err := s.projectByID(r, pid); err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	list, err := s.Store.ListVersions(pid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	if !store.CanWriteProject(pc.ProjectRole) {
		errJSON(w, http.StatusForbidden, "project admin required")
		return
	}
	var req nameReq
	if err := readJSON(r, &req); err != nil || req.Name == "" {
		errJSON(w, http.StatusBadRequest, "version name required")
		return
	}
	v, err := s.Store.CreateVersion(tid, pid, pname, req.Name)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) handlePublishVersion(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	if !store.CanWriteProject(pc.ProjectRole) {
		errJSON(w, http.StatusForbidden, "project admin required")
		return
	}
	if err := s.Store.PublishVersion(tid, pid, ver); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	metrics.publishTotal.Add(1)
	s.auditSession(r, "version.publish", "project_id="+chi.URLParam(r, "id")+" version="+ver)
	v, _ := s.Store.GetVersion(pid, ver)
	_ = pname
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	ver := chi.URLParam(r, "ver")
	confirm := r.URL.Query().Get("confirm")
	if confirm != "yes" {
		errJSON(w, http.StatusBadRequest, "add ?confirm=yes to confirm version deletion")
		return
	}
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	if !store.CanWriteProject(pc.ProjectRole) {
		errJSON(w, http.StatusForbidden, "project admin required")
		return
	}
	if err := s.Store.DeleteVersion(tid, pid, pname, ver); err != nil {
		errJSON(w, http.StatusNotFound, err.Error())
		return
	}
	s.auditSession(r, "version.delete", "project="+pname+" version="+ver)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListConfigFiles(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	entries, err := s.Store.ListConfigFileEntries(tid, pname, ver)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	dup := make(map[string]int)
	for _, e := range entries {
		dup[e.Basename]++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"files":      entries,
		"duplicates": dup,
	})
}

func (s *Server) handleListVersionFiles(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	files, err := s.Store.ListVersionFiles(tid, pname, ver)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	rel := r.FormValue("path")
	file, hdr, err := r.FormFile("file")
	if err != nil {
		errJSON(w, http.StatusBadRequest, "file required")
		return
	}
	defer func() { _ = file.Close() }()

	if rel == "" {
		rel = hdr.Filename
	}

	uploadDetail := "project=" + pname + " version=" + ver + " file=" + rel
	if strings.HasSuffix(strings.ToLower(hdr.Filename), ".zip") {
		buf, err := io.ReadAll(file)
		if err != nil {
			errJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
		ra := bytes.NewReader(buf)
		if err := s.Store.ExtractZipToVersion(tid, pname, ver, ra, int64(len(buf))); err != nil {
			errJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		s.auditSession(r, "version.upload.zip", uploadDetail)
		writeJSON(w, http.StatusOK, map[string]string{"status": "zip extracted"})
		return
	}

	if err := s.Store.WriteVersionFile(tid, pname, ver, rel, file); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "version.upload", uploadDetail)
	writeJSON(w, http.StatusOK, map[string]string{"path": rel})
}

func (s *Server) handleDeleteVersionFile(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	ver := chi.URLParam(r, "ver")
	rel := r.URL.Query().Get("path")
	if rel == "" {
		errJSON(w, http.StatusBadRequest, "path query required")
		return
	}
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	tid, pname := pc.TenantID, pc.ProjectName
	if err := s.Store.DeleteVersionFile(tid, pname, ver, rel); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
