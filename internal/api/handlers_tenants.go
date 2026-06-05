package api

import "net/http"

type tenantReq struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (s *Server) handleListTenants(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok || sess.Username != "root" {
		errJSON(w, http.StatusForbidden, "system root required")
		return
	}
	list, err := s.Store.ListTenants()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok || sess.Username != "root" {
		errJSON(w, http.StatusForbidden, "system root required")
		return
	}
	var req tenantReq
	if err := readJSON(r, &req); err != nil || req.Slug == "" || req.Name == "" {
		errJSON(w, http.StatusBadRequest, "slug and name required")
		return
	}
	t, err := s.Store.CreateTenant(req.Slug, req.Name)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "tenant.create", "slug="+req.Slug)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) mePayload(sess session, token string) map[string]any {
	role, _ := s.Store.UserRole(sess.UserID)
	slug := sess.TenantSlug
	if slug == "" {
		if t, err := s.Store.TenantByID(sess.TenantID); err == nil {
			slug = t.Slug
		}
	}
	out := map[string]any{
		"username":    sess.Username,
		"is_admin":    sess.IsAdmin,
		"role":        role,
		"tenant_id":   sess.TenantID,
		"tenant_slug": slug,
	}
	if token != "" {
		out["token"] = token
	}
	return out
}
