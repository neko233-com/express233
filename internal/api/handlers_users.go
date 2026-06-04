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
	writeJSON(w, http.StatusOK, users)
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
	writeJSON(w, http.StatusCreated, u)
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
