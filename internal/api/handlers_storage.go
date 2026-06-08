package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/neko233-com/express233/internal/store"
)

func (s *Server) handleStorageOverview(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	ov, err := s.Store.StorageOverviewForTenant(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ov.IndexEntryCount == 0 {
		if n, err := s.Store.RebuildStorageIndex(tid); err == nil {
			ov.IndexEntryCount = n
			if meta, err := s.Store.StorageIndexMeta(); err == nil {
				ov.IndexUpdatedAt = meta.UpdatedAt
			}
		}
	}
	writeJSON(w, http.StatusOK, ov)
}

func (s *Server) handleStorageTree(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	node, err := s.Store.StorageTreeAt(tid, path)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleStorageSearch(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"hits": []any{}})
		return
	}
	meta, _ := s.Store.StorageIndexMeta()
	if meta.EntryCount == 0 {
		_, _ = s.Store.RebuildStorageIndex(tid)
	}
	hits, err := s.Store.SearchStorageIndex(tid, q, 80)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hits": hits})
}

func (s *Server) handleStorageReindex(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	n, err := s.Store.RebuildStorageIndex(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	meta, _ := s.Store.StorageIndexMeta()
	s.auditSession(r, "storage.reindex", "entries="+strconv.Itoa(n))
	writeJSON(w, http.StatusOK, map[string]any{"entries": n, "updated_at": meta.UpdatedAt})
}

func (s *Server) handleStorageDeletePlan(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		errJSON(w, http.StatusBadRequest, "path required")
		return
	}
	sess, _ := s.currentSession(r)
	tenantRole, _ := s.Store.UserRole(sess.UserID)
	plan, err := s.Store.PlanStorageDelete(tid, path, tenantRole)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if plan.Kind == "version" || plan.Kind == "project" || plan.Kind == "file" {
		if plan.Allowed && (plan.Kind == "project" || plan.Kind == "version" || plan.Kind == "file") {
			parts := strings.Split(path, "/")
			if len(parts) >= 4 {
				if p, err := s.Store.GetProjectByName(tid, parts[3]); err == nil {
					pc, err := s.projectByID(r, p.ID)
					if err != nil || !store.CanWriteProject(pc.ProjectRole) {
						plan.Allowed = false
						plan.DenyReason = "project write access required"
					}
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, plan)
}

type storageDeleteReq struct {
	Path string `json:"path"`
}

func (s *Server) handleStorageDelete(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	var req storageDeleteReq
	if err := readJSON(r, &req); err != nil || req.Path == "" {
		errJSON(w, http.StatusBadRequest, "path required")
		return
	}
	sess, _ := s.currentSession(r)
	tenantRole, _ := s.Store.UserRole(sess.UserID)
	plan, err := s.Store.PlanStorageDelete(tid, req.Path, tenantRole)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if plan.Kind == "version" || plan.Kind == "project" || plan.Kind == "file" {
		parts := strings.Split(req.Path, "/")
		if len(parts) >= 4 {
			if p, err := s.Store.GetProjectByName(tid, parts[3]); err == nil {
				pc, err := s.projectByID(r, p.ID)
				if err != nil || !store.CanWriteProject(pc.ProjectRole) {
					errJSON(w, http.StatusForbidden, "project write access required")
					return
				}
			}
		}
	} else if tenantRole != store.RoleAdmin {
		errJSON(w, http.StatusForbidden, "admin required")
		return
	}
	if !plan.Allowed {
		errJSON(w, http.StatusBadRequest, plan.DenyReason)
		return
	}
	if err := s.Store.ExecuteStorageDelete(tid, req.Path); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	_, _ = s.Store.RebuildStorageIndex(tid)
	s.auditSession(r, "storage.delete", "path="+req.Path)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
