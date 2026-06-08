package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

type fileTagsReq struct {
	Path string   `json:"path"`
	Tags []string `json:"tags"`
}

type fileTagsBatchReq struct {
	Paths    []string `json:"paths"`
	Patterns []string `json:"patterns"`
	Tags     []string `json:"tags"`
	Mode     string   `json:"mode"`
}

func (s *Server) handleListVersionFileTags(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	rows, err := s.Store.ListVersionFileTags(pc.TenantID, pc.ProjectName, ver)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handlePutVersionFileTags(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	var req fileTagsReq
	if err := readJSON(r, &req); err != nil || req.Path == "" {
		errJSON(w, http.StatusBadRequest, "path required")
		return
	}
	if err := s.Store.SetVersionFileTags(pc.TenantID, pc.ProjectName, ver, req.Path, req.Tags); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	tags, _ := s.Store.TagsForVersionFile(pc.TenantID, pc.ProjectName, ver, req.Path)
	writeJSON(w, http.StatusOK, store.VersionFileTag{Path: req.Path, Tags: tags})
	s.auditSession(r, "version.file_tags.set", "project="+pc.ProjectName+" version="+ver+" path="+req.Path)
}

func (s *Server) handleDeleteVersionFileTags(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	ver := chi.URLParam(r, "ver")
	path := r.URL.Query().Get("path")
	if path == "" {
		errJSON(w, http.StatusBadRequest, "path query required")
		return
	}
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if err := s.Store.ClearVersionFileTags(pc.TenantID, pc.ProjectName, ver, path); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.auditSession(r, "version.file_tags.clear", "project="+pc.ProjectName+" version="+ver+" path="+path)
}

func (s *Server) handleBatchVersionFileTags(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	ver := chi.URLParam(r, "ver")
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	var req fileTagsBatchReq
	if err := readJSON(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	mode := store.FileTagBatchMode(req.Mode)
	if mode == "" {
		mode = store.FileTagSet
	}
	switch mode {
	case store.FileTagSet, store.FileTagAdd, store.FileTagRemove, store.FileTagClear:
	default:
		errJSON(w, http.StatusBadRequest, "mode must be set/add/remove/clear")
		return
	}
	rows, err := s.Store.BatchUpdateVersionFileTags(pc.TenantID, pc.ProjectName, ver, req.Paths, req.Patterns, req.Tags, mode)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": rows})
	s.auditSession(r, "version.file_tags.batch", "project="+pc.ProjectName+" version="+ver+" mode="+string(mode))
}
