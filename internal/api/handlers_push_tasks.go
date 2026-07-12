package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

type pushTaskRequest struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	ServerIDs []string `json:"server_ids"`
	Tags      []string `json:"tags"`
	TagMatch  string   `json:"tag_match"`
}

type runPushTaskRequest struct {
	Version string `json:"version"`
	DryRun  bool   `json:"dry_run"`
}

func pushTaskID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "taskID"), 10, 64)
	return id
}

func (s *Server) validateTaskVersion(projectID int64, version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil
	}
	item, err := s.Store.GetVersion(projectID, version)
	if err != nil || item.Status != "published" {
		return fmt.Errorf("task version must be published or empty for latest")
	}
	return nil
}

func (s *Server) handleCreatePushTask(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	var request pushTaskRequest
	if err := readJSON(r, &request); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.validateTaskVersion(projectID, request.Version); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	session, _ := s.currentSession(r)
	task, err := s.Store.CreatePushDeploymentTask(store.PushDeploymentTask{TenantID: project.TenantID, ProjectID: projectID, Name: request.Name, Version: request.Version, ServerIDs: request.ServerIDs, Tags: request.Tags, TagMatch: request.TagMatch, CreatedBy: session.Username})
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "push.task.create", fmt.Sprintf("project_id=%d task_id=%d name=%s", projectID, task.ID, task.Name))
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleListPushTasks(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	items, err := s.Store.ListPushDeploymentTasks(project.TenantID, projectID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetPushTask(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	task, err := s.Store.GetPushDeploymentTask(project.TenantID, projectID, pushTaskID(r))
	if err != nil {
		errJSON(w, http.StatusNotFound, "release task not found")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleUpdatePushTask(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	var request pushTaskRequest
	if err := readJSON(r, &request); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.validateTaskVersion(projectID, request.Version); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	err = s.Store.UpdatePushDeploymentTask(store.PushDeploymentTask{ID: pushTaskID(r), TenantID: project.TenantID, ProjectID: projectID, Name: request.Name, Version: request.Version, ServerIDs: request.ServerIDs, Tags: request.Tags, TagMatch: request.TagMatch})
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "push.task.update", fmt.Sprintf("project_id=%d task_id=%d", projectID, pushTaskID(r)))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeletePushTask(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if err := s.Store.DeletePushDeploymentTask(project.TenantID, projectID, pushTaskID(r)); err != nil {
		errJSON(w, http.StatusNotFound, "release task not found")
		return
	}
	s.auditSession(r, "push.task.delete", fmt.Sprintf("project_id=%d task_id=%d logs=retained", projectID, pushTaskID(r)))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRunPushTask(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	task, err := s.Store.GetPushDeploymentTask(project.TenantID, projectID, pushTaskID(r))
	if err != nil {
		errJSON(w, http.StatusNotFound, "release task not found")
		return
	}
	var request runPushTaskRequest
	if r.ContentLength != 0 {
		if err := readJSON(r, &request); err != nil {
			errJSON(w, http.StatusBadRequest, "invalid body")
			return
		}
	}
	version := task.Version
	if strings.TrimSpace(request.Version) != "" {
		version = request.Version
	}
	deployment, err := s.createPushDeployment(r, project, createPushDeploymentRequest{Version: version, ServerIDs: task.ServerIDs, Tags: task.Tags, TagMatch: task.TagMatch, DryRun: request.DryRun}, task.ID, task.Name)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, deployment)
}
