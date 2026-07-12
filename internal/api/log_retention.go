package api

import (
	"context"
	"log/slog"
	"time"
)

// StartLogRetention prunes all operational logs on startup and every day.
func (s *Server) StartLogRetention(ctx context.Context) {
	s.logRetentionOnce.Do(func() {
		go func() {
			prune := func() {
				result, err := s.Store.PruneLogs(time.Now())
				if err != nil {
					slog.Error("prune 30-day logs", "error", err)
					return
				}
				slog.Info("30-day log retention completed", "audit", result.AuditLogs, "uploads", result.UploadEvents, "project", result.ProjectLogs, "deployments", result.PushDeployments, "ssh_checks", result.PushHostChecks, "hook_events", result.ReleaseHookEvents, "login_ip", result.LoginIPHistories)
			}
			prune()
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					prune()
				}
			}
		}()
	})
}
