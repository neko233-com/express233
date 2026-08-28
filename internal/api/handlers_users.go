package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	users, err := s.Store.ListUsers(tid)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	type userSummary struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		Role      string `json:"role"`
		IsAdmin   bool   `json:"is_admin"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]userSummary, 0, len(users))
	for _, user := range users {
		out = append(out, userSummary{
			ID: user.ID, Username: user.Username, Role: user.Role,
			IsAdmin: user.IsAdmin, CreatedAt: user.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type createUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	IsAdmin  bool   `json:"is_admin"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserReq
	if err := readJSON(r, &req); err != nil || req.Username == "" || req.Password == "" {
		errJSON(w, http.StatusBadRequest, "username and password required")
		return
	}
	tid, ok := s.tenantFromSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	u, err := s.Store.CreateUser(tid, req.Username, req.Password, req.Role, req.IsAdmin)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditSession(r, "user.create", "username="+req.Username)
	// 拉取 Token 只在创建或主动刷新时返回一次；用户列表永不回传完整 Token。
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": u.ID, "username": u.Username, "role": u.Role,
		"is_admin": u.IsAdmin, "created_at": u.CreatedAt, "token": u.Token,
	})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.Store.DeleteUser(id); err != nil {
		errJSON(w, http.StatusNotFound, "user not found")
		return
	}
	s.auditSession(r, "user.delete", "id="+chi.URLParam(r, "id"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	token, err := s.Store.RefreshUserToken(id)
	if err != nil {
		errJSON(w, http.StatusNotFound, "user not found")
		return
	}
	s.auditSession(r, "user.refresh_token", "id="+chi.URLParam(r, "id"))
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
