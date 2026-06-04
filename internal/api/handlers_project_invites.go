package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

type createInviteReq struct {
	Role       string `json:"role"`        // admin | viewer
	ValidHours int    `json:"valid_hours"` // default 168
}

func (s *Server) handleCreateProjectInvite(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if !store.CanWriteProject(pc.ProjectRole) {
		errJSON(w, http.StatusForbidden, "project admin required")
		return
	}
	var req createInviteReq
	if err := readJSON(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	role := req.Role
	if role == "" {
		role = store.ProjectRoleViewer
	}
	sess, _ := s.currentSession(r)
	inv, err := s.Store.CreateProjectInvite(pid, sess.UserID, role, req.ValidHours)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	base := requestOrigin(r)
	writeJSON(w, http.StatusCreated, map[string]any{
		"invite": inv,
		"url":    fmt.Sprintf("%s/#invite?token=%s", base, inv.Token),
	})
}

func (s *Server) handleListProjectInvites(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	if _, err := s.projectByID(r, pid); err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	list, err := s.Store.ListProjectInvites(pid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	base := requestOrigin(r)
	out := make([]map[string]any, 0, len(list))
	for _, inv := range list {
		out = append(out, map[string]any{
			"id":         inv.ID,
			"role":       inv.Role,
			"expires_at": inv.ExpiresAt,
			"created_at": inv.CreatedAt,
			"url":        fmt.Sprintf("%s/#invite?token=%s", base, inv.Token),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListProjectMembers(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	if _, err := s.projectByID(r, pid); err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	members, err := s.Store.ListProjectMembers(pid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (s *Server) handleRemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconvParseInt(chi.URLParam(r, "id"))
	uid, _ := strconvParseInt(chi.URLParam(r, "uid"))
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if !store.CanWriteProject(pc.ProjectRole) {
		errJSON(w, http.StatusForbidden, "project admin required")
		return
	}
	if err := s.Store.RemoveProjectMember(pid, uid); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/project-invites/{token} — 登录后可预览。
func (s *Server) handlePreviewProjectInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	info, err := s.Store.ProjectInvitePreview(token)
	if err != nil {
		errJSON(w, http.StatusNotFound, "invite not found")
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// POST /api/project-invites/{token}/accept
func (s *Server) handleAcceptProjectInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	sess, ok := s.currentSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	p, err := s.Store.AcceptProjectInvite(token, sess.UserID)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "project.invite.accept", "project="+p.Name)
	writeJSON(w, http.StatusOK, p)
}

func requestOrigin(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		host := r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
		return proto + "://" + host
	}
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}
