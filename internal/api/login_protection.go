package api

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

func loginIPPolicy() store.LoginIPPolicy {
	return store.LoginIPPolicy{
		FailureLimit: envPositiveInt("EXPRESS233_LOGIN_FAILURE_LIMIT", 5),
		Window:       envPositiveDuration("EXPRESS233_LOGIN_FAILURE_WINDOW", time.Minute),
		BaseBan:      envPositiveDuration("EXPRESS233_LOGIN_BAN_BASE", 10*time.Second),
		MaxBan:       envPositiveDuration("EXPRESS233_LOGIN_BAN_MAX", 10*time.Second),
	}
}

func (s *Server) loginProtectionEnabled() bool {
	enabled, err := s.Store.LoginProtectionEnabled()
	return err == nil && enabled
}

func envPositiveInt(name string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return fallback
}
func envPositiveDuration(name string, fallback time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return fallback
}

// clientIP trusts forwarding headers only when the deployment explicitly opts
// in. Without this guard, an attacker can rotate X-Forwarded-For to bypass IP
// bans on a directly reachable server.
func clientIP(r *http.Request) string {
	if ip, reliable := loginProtectionIP(r); reliable {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// loginProtectionIP rejects shared loopback proxy addresses without a forwarded IP.
func loginProtectionIP(r *http.Request) (string, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if os.Getenv("EXPRESS233_TRUST_PROXY") != "1" || peer == nil || !peer.IsLoopback() {
		return host, peer != nil
	}
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String(), true
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for index := len(parts) - 1; index >= 0; index-- {
		if ip := net.ParseIP(strings.TrimSpace(parts[index])); ip != nil {
			return ip.String(), true
		}
	}
	return "", false
}

func (s *Server) rejectBlockedLogin(w http.ResponseWriter, r *http.Request, username string) bool {
	if !s.loginProtectionEnabled() {
		return false
	}
	ip, reliable := loginProtectionIP(r)
	if !reliable {
		return false
	}
	retry, blocked, err := s.Store.LoginIPBlocked(ip, time.Now())
	if err != nil || !blocked {
		return false
	}
	if retry < time.Second {
		retry = time.Second
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
	errJSON(w, http.StatusTooManyRequests, "too many login attempts; try again later")
	return true
}

func (s *Server) recordLoginFailure(r *http.Request, username string) (banned bool, retry time.Duration, remaining int) {
	ip, reliable := loginProtectionIP(r)
	if !reliable {
		return false, 0, 0
	}
	policy := loginIPPolicy()
	enabled := s.loginProtectionEnabled()
	if !enabled {
		policy.FailureLimit = 0
	}
	ban, err := s.Store.RecordLoginIPFailure(ip, username, time.Now(), policy)
	if err != nil {
		return false, 0, 0
	}
	if !enabled {
		return false, 0, 0
	}
	remaining = policy.FailureLimit - ban.Failures
	if remaining < 0 {
		remaining = 0
	}
	if ban.BannedUntil != "" {
		until, err := time.Parse(time.RFC3339Nano, ban.BannedUntil)
		if err == nil && until.After(time.Now()) {
			retry = time.Until(until).Round(time.Second)
			banned = true
		}
	}
	if banned {
		s.audit(r, username, "login.ban", "ip="+ip+" duration="+retry.String())
	}
	return banned, retry, remaining
}

// recordAgentTokenFailure keeps pull-agent credentials independently protected.
// Interactive login enforcement remains controlled by the administrator toggle.
func (s *Server) recordAgentTokenFailure(r *http.Request) (banned bool, retry time.Duration, remaining int) {
	ip, reliable := loginProtectionIP(r)
	if !reliable {
		return false, 0, 0
	}
	policy := loginIPPolicy()
	ban, err := s.Store.RecordLoginIPFailure(ip, "agent-token", time.Now(), policy)
	if err != nil {
		return false, 0, 0
	}
	remaining = policy.FailureLimit - ban.Failures
	if remaining < 0 {
		remaining = 0
	}
	if ban.BannedUntil == "" {
		return false, 0, remaining
	}
	until, err := time.Parse(time.RFC3339Nano, ban.BannedUntil)
	if err != nil || !until.After(time.Now()) {
		return false, 0, remaining
	}
	return true, time.Until(until).Round(time.Second), remaining
}

type loginProtectionSettingsResponse struct {
	Enabled      bool `json:"enabled"`
	FailureLimit int  `json:"failure_limit"`
	BanSeconds   int  `json:"ban_seconds"`
}

func loginProtectionSettingsPayload(enabled bool) loginProtectionSettingsResponse {
	policy := loginIPPolicy()
	return loginProtectionSettingsResponse{
		Enabled:      enabled,
		FailureLimit: policy.FailureLimit,
		BanSeconds:   max(1, int(policy.BaseBan.Seconds())),
	}
}

func (s *Server) handleGetLoginProtection(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, loginProtectionSettingsPayload(s.loginProtectionEnabled()))
}

func (s *Server) handlePutLoginProtection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.Store.SetLoginProtectionEnabled(req.Enabled); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditSession(r, "login_protection.update", "enabled="+strconv.FormatBool(req.Enabled))
	writeJSON(w, http.StatusOK, loginProtectionSettingsPayload(req.Enabled))
}

func (s *Server) clearLoginFailures(r *http.Request) {
	if ip, reliable := loginProtectionIP(r); reliable {
		_ = s.Store.ClearLoginIPFailures(ip)
	}
}

func (s *Server) handleListLoginIPBans(w http.ResponseWriter, r *http.Request) {
	bans, err := s.Store.ListLoginIPBans(time.Now(), 200)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bans)
}

func (s *Server) handleDeleteLoginIPBan(w http.ResponseWriter, r *http.Request) {
	ip := chi.URLParam(r, "ip")
	if net.ParseIP(ip) == nil {
		errJSON(w, http.StatusBadRequest, "invalid IP address")
		return
	}
	if err := s.Store.DeleteLoginIPHistory(ip); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditSession(r, "login_ban.clear", "ip="+ip)
	w.WriteHeader(http.StatusNoContent)
}
