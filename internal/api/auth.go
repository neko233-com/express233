package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

type session struct {
	UserID     int64
	Username   string
	IsAdmin    bool
	TenantID   int64
	TenantSlug string
	Expires    time.Time
}

type sessionStore struct {
	mu   sync.RWMutex
	data map[string]session
}

func newSessionStore() *sessionStore {
	return &sessionStore{data: make(map[string]session)}
}

func (ss *sessionStore) create(userID int64, username string, isAdmin bool, tenantID int64, tenantSlug string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	ss.mu.Lock()
	ss.data[id] = session{
		UserID:     userID,
		Username:   username,
		IsAdmin:    isAdmin,
		TenantID:   tenantID,
		TenantSlug: tenantSlug,
		Expires:    time.Now().Add(24 * time.Hour),
	}
	ss.mu.Unlock()
	return id, nil
}

func (ss *sessionStore) get(id string) (session, bool) {
	ss.mu.RLock()
	s, ok := ss.data[id]
	ss.mu.RUnlock()
	if !ok || time.Now().After(s.Expires) {
		return session{}, false
	}
	return s, true
}

func (ss *sessionStore) delete(id string) {
	ss.mu.Lock()
	delete(ss.data, id)
	ss.mu.Unlock()
}

const sessionCookie = "express233_session"

func (s *Server) currentSession(r *http.Request) (session, bool) {
	return s.sessionFromRequest(r)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
