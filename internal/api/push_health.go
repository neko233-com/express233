package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/express233/internal/store"
)

const (
	pushHealthScheduleTick = 30 * time.Second
	pushHealthCheckTimeout = 25 * time.Second
)

// StartSSHHealthMonitor starts one process-local, serial scheduler. Each due
// host receives one connection attempt and is then scheduled for its full
// configured interval, regardless of the result.
func (s *Server) StartSSHHealthMonitor(ctx context.Context) {
	s.pushHealthOnce.Do(func() {
		go func() {
			s.runDuePushHealthChecks(ctx)
			ticker := time.NewTicker(pushHealthScheduleTick)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.runDuePushHealthChecks(ctx)
				}
			}
		}()
	})
}

func (s *Server) runDuePushHealthChecks(ctx context.Context) {
	hosts, err := s.Store.ListDuePushHosts(time.Now(), 100)
	if err != nil {
		slog.Error("list due SSH health checks", "error", err)
		return
	}
	for i := range hosts {
		if ctx.Err() != nil {
			return
		}
		if _, err := s.checkPushHost(ctx, &hosts[i], "scheduled"); err != nil {
			slog.Error("record SSH health check", "host_id", hosts[i].ID, "error", err)
		}
	}
}

func (s *Server) checkPushHost(ctx context.Context, host *store.PushHost, trigger string) (*store.PushHostCheck, error) {
	// Serial execution avoids creating an hourly connection burst across a large
	// game cluster and also prevents a manual check racing a scheduled one.
	s.pushHealthMu.Lock()
	defer s.pushHealthMu.Unlock()

	started := time.Now()
	checkContext, cancel := context.WithTimeout(ctx, pushHealthCheckTimeout)
	defer cancel()
	err := s.pushHealthChecker.Check(withAPIStore(checkContext, s.Store), *host)
	check := store.PushHostCheck{
		TenantID:  host.TenantID,
		HostID:    host.ID,
		Status:    "ok",
		LatencyMS: time.Since(started).Milliseconds(),
		Trigger:   trigger,
	}
	metrics.sshCheckTotal.Add(1)
	if err != nil {
		check.Status = "failed"
		check.Error = sanitizeSSHCheckError(err.Error())
		metrics.sshCheckErrors.Add(1)
	}
	return s.Store.RecordPushHostCheck(check)
}

func sanitizeSSHCheckError(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\x00", ""))
	if len(message) > 1000 {
		message = message[:1000]
	}
	return message
}

func (s *Server) handleCheckPushHost(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := s.tenantFromSession(r)
	host, _, err := s.Store.GetPushHost(tenantID, pushHostID(r))
	if err != nil {
		errJSON(w, http.StatusNotFound, "host not found")
		return
	}
	check, err := s.checkPushHost(r.Context(), host, "manual")
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "failed to record SSH check")
		return
	}
	s.auditSession(r, "push.host.check", "host_id="+strconv.FormatInt(host.ID, 10)+" status="+check.Status)
	writeJSON(w, http.StatusOK, check)
}

func (s *Server) handleListPushHostChecks(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := s.tenantFromSession(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	checks, err := s.Store.ListPushHostChecks(tenantID, pushHostID(r), limit)
	if err != nil {
		errJSON(w, http.StatusNotFound, "host not found")
		return
	}
	writeJSON(w, http.StatusOK, checks)
}
