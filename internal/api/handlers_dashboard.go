package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/neko233-com/express233/internal/store"
)

func (s *Server) recordUploadEvent(r *http.Request, pc projectCtx, version, kind string, size, files int64, detail string, uploadErr error) {
	if size < 0 {
		size = 0
	}
	sess, _ := s.currentSession(r)
	event := store.UploadEvent{
		TenantID: pc.TenantID, ProjectID: pc.ProjectID, Version: version,
		Kind: kind, Bytes: size, FileCount: files, Status: "success",
		Username: sess.Username, Detail: detail, IP: clientIP(r),
	}
	if uploadErr != nil {
		event.Status = "failed"
		event.Error = uploadErr.Error()
	}
	_ = s.Store.RecordUploadEvent(event)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	sess, _ := s.currentSession(r)
	tenantRole, _ := s.Store.UserRole(sess.UserID)
	projects, err := s.Store.ListProjects(tid, sess.UserID, tenantRole)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	allowed := make(map[int64]struct{}, len(projects))
	ids := make([]int64, 0, len(projects))
	for _, project := range projects {
		allowed[project.ID] = struct{}{}
		ids = append(ids, project.ID)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("project_id")); raw != "" {
		id, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			errJSON(w, http.StatusBadRequest, "invalid project_id")
			return
		}
		if _, exists := allowed[id]; !exists {
			errJSON(w, http.StatusNotFound, "project not found")
			return
		}
		ids = []int64{id}
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days == 0 {
		days = 30
	}
	if days < 1 || days > 30 {
		errJSON(w, http.StatusBadRequest, "days must be between 1 and 30 because logs use a 30-day rolling retention window")
		return
	}
	dashboard, err := s.Store.Dashboard(tid, ids, days)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}
