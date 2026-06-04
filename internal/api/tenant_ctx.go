package api

import (
	"net/http"

	"github.com/neko233-com/express233/internal/config"
)

func (s *Server) tenantFromSession(r *http.Request) (int64, bool) {
	sess, ok := s.currentSession(r)
	if !ok {
		return 0, false
	}
	if sess.TenantID > 0 {
		return sess.TenantID, true
	}
	tid, err := s.Store.UserTenantID(sess.UserID)
	if err != nil {
		return 0, false
	}
	return tid, true
}

func (s *Server) loadServerFile(tenantID int64) *config.ServerFile {
	path, err := s.Store.ServerYAMLPath(tenantID)
	if err != nil {
		return &config.ServerFile{Servers: map[string]config.ServerEntry{}}
	}
	sf, err := config.LoadServerFile(path)
	if err != nil {
		return &config.ServerFile{Servers: map[string]config.ServerEntry{}}
	}
	return sf
}

func (s *Server) getServerFile(tenantID int64) *config.ServerFile {
	s.serverMu.RLock()
	if s.serverByTenant != nil {
		if sf, ok := s.serverByTenant[tenantID]; ok && sf != nil {
			s.serverMu.RUnlock()
			return sf
		}
	}
	s.serverMu.RUnlock()
	return s.loadServerFile(tenantID)
}

func (s *Server) reloadServerYAML(tenantID int64) {
	sf := s.loadServerFile(tenantID)
	s.serverMu.Lock()
	if s.serverByTenant == nil {
		s.serverByTenant = make(map[int64]*config.ServerFile)
	}
	s.serverByTenant[tenantID] = sf
	s.serverMu.Unlock()
}

func (s *Server) pullTenant(token string) (tenantID int64, username string, ok bool) {
	uid, tid, err := s.Store.LookupPullToken(token)
	if err != nil {
		return 0, "", false
	}
	_ = uid
	u, _ := s.Store.GetUserByID(uid)
	name := ""
	if u != nil {
		name = u.Username
	}
	return tid, name, true
}
