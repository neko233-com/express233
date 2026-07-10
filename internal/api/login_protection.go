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
		Window:       envPositiveDuration("EXPRESS233_LOGIN_FAILURE_WINDOW", 15*time.Minute),
		BaseBan:      envPositiveDuration("EXPRESS233_LOGIN_BAN_BASE", 15*time.Minute),
		MaxBan:       envPositiveDuration("EXPRESS233_LOGIN_BAN_MAX", 24*time.Hour),
	}
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
	if os.Getenv("EXPRESS233_TRUST_PROXY") == "1" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if ip := net.ParseIP(strings.TrimSpace(strings.Split(xff, ",")[0])); ip != nil {
				return ip.String()
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) rejectBlockedLogin(w http.ResponseWriter, r *http.Request, username string) bool {
	ip := clientIP(r)
	retry, blocked, err := s.Store.LoginIPBlocked(ip, time.Now())
	if err != nil || !blocked {
		return false
	}
	if retry < time.Second {
		retry = time.Second
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
	s.audit(r, username, "login.blocked", "retry_after="+retry.String())
	errJSON(w, http.StatusTooManyRequests, "too many login attempts; try again later")
	return true
}

func (s *Server) recordLoginFailure(r *http.Request, username string) (banned bool, retry time.Duration, remaining int) {
	ban, err := s.Store.RecordLoginIPFailure(clientIP(r), time.Now(), loginIPPolicy())
	if err != nil {
		return false, 0, 0
	}
	remaining = loginIPPolicy().FailureLimit - ban.Failures
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
	detail := "failed"
	if banned {
		detail += " banned_for=" + retry.String()
	}
	s.audit(r, username, "login.failure", detail)
	return banned, retry, remaining
}

func (s *Server) clearLoginFailures(r *http.Request) { _ = s.Store.ClearLoginIPFailures(clientIP(r)) }

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
	if err := s.Store.ClearLoginIPFailures(ip); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditSession(r, "login_ban.clear", "ip="+ip)
	w.WriteHeader(http.StatusNoContent)
}
