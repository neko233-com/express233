package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

type releaseHookRequest struct {
	Name            string `json:"name"`
	TaskID          int64  `json:"task_id"`
	Enabled         *bool  `json:"enabled"`
	DebounceSeconds int    `json:"debounce_seconds"`
}

type triggerReleaseHookRequest struct {
	Version string `json:"version"`
	Source  string `json:"source"`
}

func releaseHookID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "hookID"), 10, 64)
	return id
}

func (s *Server) handleCreateReleaseHook(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	var request releaseHookRequest
	if err := readJSON(r, &request); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	session, _ := s.currentSession(r)
	hook, err := s.Store.CreateReleaseHook(store.ReleaseHook{TenantID: project.TenantID, ProjectID: projectID, TaskID: request.TaskID, Name: request.Name, Enabled: enabled, DebounceSeconds: request.DebounceSeconds, CreatedBy: session.Username})
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "release.hook.create", fmt.Sprintf("project_id=%d hook_id=%d task_id=%d enabled=%t debounce=%ds", projectID, hook.ID, hook.TaskID, hook.Enabled, hook.DebounceSeconds))
	writeJSON(w, http.StatusCreated, hook)
}

func (s *Server) handleListReleaseHooks(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	hooks, err := s.Store.ListReleaseHooks(project.TenantID, projectID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, hooks)
}

func (s *Server) handleUpdateReleaseHook(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	current, err := s.Store.GetReleaseHook(project.TenantID, projectID, releaseHookID(r))
	if err != nil {
		errJSON(w, http.StatusNotFound, "release hook not found")
		return
	}
	var request releaseHookRequest
	if err := readJSON(r, &request); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		request.Name = current.Name
	}
	if request.TaskID == 0 {
		request.TaskID = current.TaskID
	}
	if request.DebounceSeconds == 0 {
		request.DebounceSeconds = current.DebounceSeconds
	}
	enabled := current.Enabled
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	err = s.Store.UpdateReleaseHook(store.ReleaseHook{ID: current.ID, TenantID: project.TenantID, ProjectID: projectID, TaskID: request.TaskID, Name: request.Name, Enabled: enabled, DebounceSeconds: request.DebounceSeconds})
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if !enabled && current.Enabled && current.PendingEvents > 0 {
		if err := s.Store.RecordReleaseHookCancellation(*current, time.Now()); err != nil {
			errJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.auditSession(r, "release.hook.update", fmt.Sprintf("project_id=%d hook_id=%d task_id=%d enabled=%t debounce=%ds", projectID, current.ID, request.TaskID, enabled, request.DebounceSeconds))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteReleaseHook(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if err := s.Store.DeleteReleaseHook(project.TenantID, projectID, releaseHookID(r)); err != nil {
		errJSON(w, http.StatusNotFound, "release hook not found")
		return
	}
	s.auditSession(r, "release.hook.delete", fmt.Sprintf("project_id=%d hook_id=%d", projectID, releaseHookID(r)))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTriggerReleaseHook(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	var request triggerReleaseHookRequest
	if r.ContentLength != 0 {
		if err := readJSON(r, &request); err != nil {
			errJSON(w, http.StatusBadRequest, "invalid body")
			return
		}
	}
	if strings.TrimSpace(request.Version) == "" {
		request.Version, err = s.Store.LatestPublishedVersion(projectID)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "no published version exists")
			return
		}
	}
	version, err := s.Store.GetVersion(projectID, request.Version)
	if err != nil || version.Status != "published" {
		errJSON(w, http.StatusBadRequest, "version must be published")
		return
	}
	session, _ := s.currentSession(r)
	request.Source = strings.TrimSpace(request.Source)
	if request.Source == "" {
		request.Source = "manual"
	}
	hook, err := s.Store.QueueReleaseHook(project.TenantID, projectID, releaseHookID(r), request.Version, session.Username, request.Source, time.Now())
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	metrics.hookTriggers.Add(1)
	if hook.PendingEvents > 1 {
		metrics.hookMerges.Add(1)
	}
	s.auditSession(r, "release.hook.trigger", fmt.Sprintf("project_id=%d hook_id=%d version=%s source=%s due_at=%s pending=%d", projectID, hook.ID, request.Version, request.Source, hook.DueAt, hook.PendingEvents))
	writeJSON(w, http.StatusAccepted, hook)
}

func (s *Server) handleListReleaseHookEvents(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := s.Store.ListReleaseHookEvents(project.TenantID, projectID, limit)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}
