package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/neko233-com/express233/internal/store"
)

// StartReleaseHookWorker dispatches persisted, due hooks. A one-second scan
// keeps scheduling precise while the 30-second trailing debounce remains in
// SQLite and therefore survives process restarts.
func (s *Server) StartReleaseHookWorker(ctx context.Context) {
	s.releaseHookOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case now := <-ticker.C:
					if err := s.processDueReleaseHooks(now); err != nil {
						slog.Error("dispatch release hooks", "error", err)
					}
				}
			}
		}()
	})
}

func (s *Server) processDueReleaseHooks(now time.Time) error {
	s.releaseHookMu.Lock()
	defer s.releaseHookMu.Unlock()
	staleRuns, err := s.Store.ListStaleReleaseHookRuns(now, 2*time.Minute, 20)
	if err != nil {
		return err
	}
	runs, err := s.Store.ClaimDueReleaseHooks(now, 20)
	if err != nil {
		return err
	}
	for _, run := range staleRuns {
		slog.Warn("recover stale release hook dispatch", "hook_id", run.Hook.ID, "event_id", run.EventID, "version", run.Version)
		s.dispatchReleaseHook(run, now)
	}
	for _, run := range runs {
		s.dispatchReleaseHook(run, now)
	}
	return nil
}

func (s *Server) dispatchReleaseHook(run store.ReleaseHookRun, now time.Time) {
	deploymentID := int64(0)
	fail := func(err error) {
		metrics.hookFailures.Add(1)
		_ = s.Store.CompleteReleaseHookRun(run, deploymentID, "failed", err.Error(), time.Now())
		_ = s.Store.RecordAudit(run.RequestedBy, "release.hook.failed", fmt.Sprintf("hook_id=%d version=%s error=%s", run.Hook.ID, run.Version, err), "")
	}
	task, err := s.Store.GetPushDeploymentTask(run.Hook.TenantID, run.Hook.ProjectID, run.Hook.TaskID)
	if err != nil {
		fail(fmt.Errorf("release task not found: %w", err))
		return
	}
	version, err := s.Store.GetVersion(run.Hook.ProjectID, run.Version)
	if err != nil || version.Status != "published" {
		fail(fmt.Errorf("triggered version is not published"))
		return
	}
	deployment, err := s.Store.GetPushDeploymentByHookEvent(run.EventID)
	created := false
	if err == sql.ErrNoRows {
		targets, targetErr := s.Store.ListPushBindingsForSelector(run.Hook.TenantID, task.ServerIDs, task.Tags, task.TagMatch == "all")
		if targetErr != nil {
			fail(targetErr)
			return
		}
		selector, _ := json.Marshal(createPushDeploymentRequest{Version: run.Version, ServerIDs: task.ServerIDs, Tags: task.Tags, TagMatch: task.TagMatch})
		deployment, err = s.Store.CreatePushDeployment(store.PushDeployment{TenantID: run.Hook.TenantID, ProjectID: run.Hook.ProjectID, TaskID: task.ID, TaskName: task.Name, HookEventID: run.EventID, Version: run.Version, RequestedBy: run.RequestedBy, Selector: string(selector)}, targets)
		created = err == nil
	}
	if err != nil {
		fail(err)
		return
	}
	deploymentID = deployment.ID
	if err := s.Store.CompleteReleaseHookRun(run, deployment.ID, "running", fmt.Sprintf("coalesced_triggers=%d deployment_id=%d", run.EventCount, deployment.ID), now); err != nil {
		slog.Error("complete release hook dispatch", "hook_id", run.Hook.ID, "deployment_id", deployment.ID, "error", err)
	}
	if created {
		metrics.hookDispatches.Add(1)
		_ = s.Store.RecordAudit(run.RequestedBy, "release.hook.dispatched", fmt.Sprintf("project_id=%d hook_id=%d task_id=%d version=%s triggers=%d deployment_id=%d", run.Hook.ProjectID, run.Hook.ID, task.ID, run.Version, run.EventCount, deployment.ID), "")
	}
	projectName, err := s.Store.ProjectNameInTenant(run.Hook.TenantID, run.Hook.ProjectID)
	if err != nil {
		fail(err)
		return
	}
	// Hook deployments are globally serialized. This extends the existing
	// per-task serial target policy and prevents two due hooks from restarting
	// overlapping game-server fleets at the same time.
	if deployment.Status == "queued" || deployment.Status == "running" {
		s.runPushDeployment(deployment.ID, run.Hook.TenantID, projectName)
	}
	completed, err := s.Store.GetPushDeployment(run.Hook.TenantID, run.Hook.ProjectID, deployment.ID)
	if err != nil {
		fail(err)
		return
	}
	status := completed.Status
	if status != "success" {
		status = "failed"
		metrics.hookFailures.Add(1)
	}
	if err := s.Store.CompleteReleaseHookRun(run, deployment.ID, status, fmt.Sprintf("coalesced_triggers=%d deployment_id=%d deployment_status=%s", run.EventCount, deployment.ID, completed.Status), time.Now()); err != nil {
		slog.Error("record release hook result", "hook_id", run.Hook.ID, "deployment_id", deployment.ID, "error", err)
	}
}
