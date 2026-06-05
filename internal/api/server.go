package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/store"
)

// Server HTTP API 与静态管理页。
type Server struct {
	Store          *store.Store
	sessions       *sessionStore
	jwt            *jwtAuth
	serverMu       sync.RWMutex
	serverByTenant map[int64]*config.ServerFile
}

// New 创建 API 服务。
func New(st *store.Store) *Server {
	s := &Server{Store: st, sessions: newSessionStore(), jwt: newJWTAuth(), serverByTenant: make(map[int64]*config.ServerFile)}
	if t, err := st.TenantByID(1); err == nil {
		_ = t
	}
	s.reloadServerYAML(1)
	return s
}

// Router 返回 chi 路由。
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)

	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleHealthz)
	r.Get("/metrics", s.handleMetrics)
	r.Get("/api/openapi.yaml", s.handleOpenAPISpec)

	r.Post("/api/login", s.handleLogin)
	r.Post("/api/logout", s.handleLogout)
	r.Get("/api/me", s.handleMe)
	r.Get("/api/project-invites/{token}", s.handlePreviewProjectInvite)

	r.Route("/api", func(r chi.Router) {
		r.Use(s.requireLogin)
		r.Group(func(r chi.Router) {
			r.Post("/project-invites/{token}/accept", s.handleAcceptProjectInvite)
		})
		r.Group(func(r chi.Router) {
			r.Use(s.requireMutator)
		r.Get("/server-ids", s.handleServerIDs)
		r.Get("/status", s.handleStatus)
		r.Post("/me/password", s.handleChangeMyPassword)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/audit-logs", s.handleListAuditLogs)
			r.Get("/tenants", s.handleListTenants)
			r.Post("/tenants", s.handleCreateTenant)
		})
		r.Get("/server-yaml", s.handleGetServerYAML)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Put("/server-yaml", s.handlePutServerYAML)
		})
		r.Get("/server-yaml/preview", s.handlePreviewReplacements)
		r.Get("/deploy/preview", s.handleDeployPreview)
		r.Get("/deploy/diff", s.handleDeployDiff)

		r.Route("/users", func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/", s.handleListUsers)
			r.Post("/", s.handleCreateUser)
			r.Delete("/{id}", s.handleDeleteUser)
			r.Post("/{id}/refresh-token", s.handleRefreshToken)
			r.Put("/{id}/password", s.handleAdminChangePassword)
		})

		r.Get("/projects", s.handleListProjects)
		r.Group(func(r chi.Router) {
			r.Use(s.requireRole(store.RoleAdmin, store.RoleOperator))
			r.Post("/projects", s.handleCreateProject)
		})
		r.Delete("/projects/{id}", s.handleDeleteProject)

		r.Get("/projects/{id}/members", s.handleListProjectMembers)
		r.Route("/projects/{id}", func(r chi.Router) {
			r.Use(s.requireProjectWriter)
			r.Post("/invites", s.handleCreateProjectInvite)
			r.Get("/invites", s.handleListProjectInvites)
			r.Delete("/members/{uid}", s.handleRemoveProjectMember)
			r.Post("/versions", s.handleCreateVersion)
			r.Post("/versions/{ver}/submit-review", s.handleSubmitReview)
			r.Post("/versions/{ver}/publish", s.handlePublishVersion)
			r.Post("/versions/{ver}/reject", s.handleRejectReview)
			r.Delete("/versions/{ver}", s.handleDeleteVersion)
			r.Post("/versions/{ver}/files", s.handleUploadFile)
			r.Delete("/versions/{ver}/files", s.handleDeleteVersionFile)
		})

		r.Get("/projects/{id}/versions", s.handleListVersions)
		r.Get("/projects/{id}/versions/{ver}/validate", s.handleValidateVersion)
		r.Get("/projects/{id}/versions/{ver}/download", s.handleDownloadVersion)
		r.Get("/projects/{id}/versions/{ver}/files", s.handleListVersionFiles)
		r.Get("/projects/{id}/versions/{ver}/config-files", s.handleListConfigFiles)
		})
	})

	r.Get("/api/pull", s.handlePull)
	r.Get("/api/pull/preview", s.handlePullPreview)
	r.Get("/api/pull/server-ids", s.handlePullServerIDs)
	r.Get("/api/pull/versions", s.handlePullVersions)
	r.Get("/api/pull/diff", s.handlePullDiff)

	r.Handle("/docs/*", http.StripPrefix("/docs/", s.docsHandler()))
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusFound)
	})

	r.Get("/*", s.handleStatic)
	return r
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
