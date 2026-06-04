package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type passwordChangeReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Server) handleChangeMyPassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return
	}
	var req passwordChangeReq
	if err := readJSON(r, &req); err != nil || req.NewPassword == "" {
		errJSON(w, http.StatusBadRequest, "new_password required")
		return
	}
	_, _, err := s.Store.Authenticate(sess.Username, req.OldPassword)
	if err != nil {
		errJSON(w, http.StatusUnauthorized, "invalid old password")
		return
	}
	if err := s.Store.UpdateUserPassword(sess.UserID, req.NewPassword); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, sess.Username, "password.change", "self")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminChangePassword(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil || req.Password == "" {
		errJSON(w, http.StatusBadRequest, "password required")
		return
	}
	if err := s.Store.UpdateUserPassword(id, req.Password); err != nil {
		errJSON(w, http.StatusNotFound, "user not found")
		return
	}
	s.auditSession(r, "password.reset", "user_id="+chi.URLParam(r, "id"))
	w.WriteHeader(http.StatusNoContent)
}
