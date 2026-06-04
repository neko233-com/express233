package api

import (
	"database/sql"
	"net/http"

	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/hookspec"
	"github.com/neko233-com/express233/internal/template"
)

// handlePullPreview 使用拉取 token 预览配置变更（无需 Web 登录，适合 SSH 节点/脚本）。
// GET /api/pull/preview?token=&project=&version=&server_id=
func (s *Server) handlePullPreview(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	project := r.URL.Query().Get("project")
	version := r.URL.Query().Get("version")
	serverID := r.URL.Query().Get("server_id")
	if token == "" || project == "" || serverID == "" {
		errJSON(w, http.StatusBadRequest, "token, project, server_id required")
		return
	}
	uid, tid, ok := s.pullUserID(token)
	if !ok {
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

	report, status, msg := s.buildDeployPreview(tid, project, version, serverID)
	if status != 0 {
		errJSON(w, status, msg)
		return
	}
	metrics.previewTotal.Add(1)
	uname := ""
	if user != nil {
		uname = user.Username
	}
	s.audit(r, uname, "pull.preview", "project="+project+" server_id="+serverID)
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) buildDeployPreview(tenantID int64, project, version, serverID string) (*template.PreviewReport, int, string) {
	entry := s.getServerFile(tenantID).Entry(serverID)
	if entry == nil {
		return nil, http.StatusNotFound, "server_id not in server.yaml"
	}
	p, err := s.Store.GetProjectByName(tenantID, project)
	if err != nil {
		return nil, http.StatusNotFound, "project not found"
	}
	if version == "" {
		version, err = s.Store.LatestPublishedVersion(p.ID)
		if err != nil {
			return nil, http.StatusNotFound, "no published version"
		}
	}
	if _, err := s.Store.GetVersion(p.ID, version); err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound, "version not found"
		}
		return nil, http.StatusInternalServerError, err.Error()
	}
	rep, err := config.PrepareReplacements(entry.Replacements)
	if err != nil {
		return nil, http.StatusBadRequest, err.Error()
	}
	root, err := s.Store.VersionDir(tenantID, project, version)
	if err != nil {
		return nil, http.StatusInternalServerError, err.Error()
	}
	report, err := template.BuildPreview(root, project, version, serverID, rep, entry.PostHook, entry.PostHookEnv)
	if err != nil {
		return nil, http.StatusInternalServerError, err.Error()
	}
	if plan, err := hookspec.PlanLines(root, hookspec.CurrentOS()); err == nil {
		report.PostHookPlan = plan
	}
	return report, 0, ""
}
