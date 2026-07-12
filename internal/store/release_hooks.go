package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const defaultReleaseHookDebounceSeconds = 30

var safeReleaseHookSource = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,40}$`)

// ReleaseHook connects a newly published project version to one reusable
// deployment task. Pending state is persisted so restarts do not lose the
// trailing debounce window.
type ReleaseHook struct {
	ID                 int64  `json:"id"`
	TenantID           int64  `json:"tenant_id,omitempty"`
	ProjectID          int64  `json:"project_id"`
	TaskID             int64  `json:"task_id"`
	TaskName           string `json:"task_name,omitempty"`
	Name               string `json:"name"`
	Enabled            bool   `json:"enabled"`
	DebounceSeconds    int    `json:"debounce_seconds"`
	PendingVersion     string `json:"pending_version,omitempty"`
	PendingSource      string `json:"pending_source,omitempty"`
	PendingSince       string `json:"pending_since,omitempty"`
	DueAt              string `json:"due_at,omitempty"`
	PendingEvents      int64  `json:"pending_events"`
	TriggerCount       int64  `json:"trigger_count"`
	MergeCount         int64  `json:"merge_count"`
	RunCount           int64  `json:"run_count"`
	LastTriggerAt      string `json:"last_trigger_at,omitempty"`
	LastRunAt          string `json:"last_run_at,omitempty"`
	LastDeploymentID   int64  `json:"last_deployment_id,omitempty"`
	LastStatus         string `json:"last_status,omitempty"`
	LastError          string `json:"last_error,omitempty"`
	CreatedBy          string `json:"created_by,omitempty"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	pendingRequestedBy string
}

type ReleaseHookEvent struct {
	ID               int64  `json:"id"`
	HookID           int64  `json:"hook_id"`
	HookName         string `json:"hook_name"`
	Kind             string `json:"kind"`
	Source           string `json:"source"`
	Version          string `json:"version"`
	Status           string `json:"status"`
	MergedEvents     int64  `json:"merged_events"`
	DeploymentID     int64  `json:"deployment_id,omitempty"`
	DeploymentStatus string `json:"deployment_status,omitempty"`
	Detail           string `json:"detail,omitempty"`
	CreatedAt        string `json:"created_at"`
	CompletedAt      string `json:"completed_at,omitempty"`
	RequestedBy      string `json:"-"`
}

type ReleaseHookRun struct {
	Hook        ReleaseHook
	Version     string
	RequestedBy string
	Source      string
	EventCount  int64
	EventID     int64
}

func (s *Store) migrateReleaseHooks() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS release_hooks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  project_id INTEGER NOT NULL,
  task_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  debounce_seconds INTEGER NOT NULL DEFAULT 30,
  pending_version TEXT NOT NULL DEFAULT '',
  pending_requested_by TEXT NOT NULL DEFAULT '',
  pending_source TEXT NOT NULL DEFAULT '',
  pending_since TEXT NOT NULL DEFAULT '',
  due_at TEXT NOT NULL DEFAULT '',
  pending_events INTEGER NOT NULL DEFAULT 0,
  trigger_count INTEGER NOT NULL DEFAULT 0,
  merge_count INTEGER NOT NULL DEFAULT 0,
  run_count INTEGER NOT NULL DEFAULT 0,
  last_trigger_at TEXT NOT NULL DEFAULT '',
  last_run_at TEXT NOT NULL DEFAULT '',
  last_deployment_id INTEGER NOT NULL DEFAULT 0,
  last_status TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(project_id, name),
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS release_hook_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  project_id INTEGER NOT NULL,
  hook_id INTEGER NOT NULL,
  hook_name TEXT NOT NULL,
  kind TEXT NOT NULL,
  source TEXT NOT NULL,
  version TEXT NOT NULL,
  status TEXT NOT NULL,
  merged_events INTEGER NOT NULL DEFAULT 0,
  deployment_id INTEGER NOT NULL DEFAULT 0,
  detail TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  completed_at TEXT NOT NULL DEFAULT '',
  requested_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_release_hooks_project ON release_hooks(project_id, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_release_hooks_project_task_unique ON release_hooks(project_id, task_id);
CREATE INDEX IF NOT EXISTS idx_release_hooks_due ON release_hooks(enabled, due_at);
CREATE INDEX IF NOT EXISTS idx_release_hook_events_project ON release_hook_events(project_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_release_hook_events_created ON release_hook_events(created_at);
`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE release_hook_events ADD COLUMN requested_by TEXT NOT NULL DEFAULT ''`)
	return nil
}

const releaseHookColumns = `h.id,h.tenant_id,h.project_id,h.task_id,COALESCE(t.name,''),h.name,h.enabled,h.debounce_seconds,h.pending_version,h.pending_requested_by,h.pending_source,h.pending_since,h.due_at,h.pending_events,h.trigger_count,h.merge_count,h.run_count,h.last_trigger_at,h.last_run_at,h.last_deployment_id,h.last_status,h.last_error,h.created_by,h.created_at,h.updated_at`

func scanReleaseHook(scanner pushHostScanner, hook *ReleaseHook) error {
	return scanner.Scan(&hook.ID, &hook.TenantID, &hook.ProjectID, &hook.TaskID, &hook.TaskName, &hook.Name, &hook.Enabled, &hook.DebounceSeconds, &hook.PendingVersion, &hook.pendingRequestedBy, &hook.PendingSource, &hook.PendingSince, &hook.DueAt, &hook.PendingEvents, &hook.TriggerCount, &hook.MergeCount, &hook.RunCount, &hook.LastTriggerAt, &hook.LastRunAt, &hook.LastDeploymentID, &hook.LastStatus, &hook.LastError, &hook.CreatedBy, &hook.CreatedAt, &hook.UpdatedAt)
}

func normalizeReleaseHook(hook *ReleaseHook) error {
	hook.Name = strings.TrimSpace(hook.Name)
	if hook.Name == "" || len(hook.Name) > 100 {
		return fmt.Errorf("hook name is required and must not exceed 100 characters")
	}
	if hook.TaskID <= 0 {
		return fmt.Errorf("release task is required")
	}
	if hook.DebounceSeconds == 0 {
		hook.DebounceSeconds = defaultReleaseHookDebounceSeconds
	}
	if hook.DebounceSeconds < 1 || hook.DebounceSeconds > 3600 {
		return fmt.Errorf("debounce_seconds must be between 1 and 3600")
	}
	return nil
}

func (s *Store) validateHookTask(tenantID, projectID, taskID int64) error {
	_, err := s.GetPushDeploymentTask(tenantID, projectID, taskID)
	if err != nil {
		return fmt.Errorf("release task not found")
	}
	return nil
}

func (s *Store) CreateReleaseHook(hook ReleaseHook) (*ReleaseHook, error) {
	if err := normalizeReleaseHook(&hook); err != nil {
		return nil, err
	}
	if err := s.validateHookTask(hook.TenantID, hook.ProjectID, hook.TaskID); err != nil {
		return nil, err
	}
	now := time.Now().Format(timeLayout)
	res, err := s.db.Exec(`INSERT INTO release_hooks(tenant_id,project_id,task_id,name,enabled,debounce_seconds,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, hook.TenantID, hook.ProjectID, hook.TaskID, hook.Name, hook.Enabled, hook.DebounceSeconds, hook.CreatedBy, now, now)
	if err != nil {
		return nil, err
	}
	hook.ID, _ = res.LastInsertId()
	return s.GetReleaseHook(hook.TenantID, hook.ProjectID, hook.ID)
}

func (s *Store) ListReleaseHooks(tenantID, projectID int64) ([]ReleaseHook, error) {
	rows, err := s.db.Query(`SELECT `+releaseHookColumns+` FROM release_hooks h LEFT JOIN push_deployment_tasks t ON t.id=h.task_id WHERE h.tenant_id=? AND h.project_id=? ORDER BY h.id DESC`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]ReleaseHook, 0)
	for rows.Next() {
		var hook ReleaseHook
		if err := scanReleaseHook(rows, &hook); err != nil {
			return nil, err
		}
		items = append(items, hook)
	}
	return items, rows.Err()
}

func (s *Store) GetReleaseHook(tenantID, projectID, id int64) (*ReleaseHook, error) {
	var hook ReleaseHook
	err := scanReleaseHook(s.db.QueryRow(`SELECT `+releaseHookColumns+` FROM release_hooks h LEFT JOIN push_deployment_tasks t ON t.id=h.task_id WHERE h.tenant_id=? AND h.project_id=? AND h.id=?`, tenantID, projectID, id), &hook)
	return &hook, err
}

func (s *Store) UpdateReleaseHook(hook ReleaseHook) error {
	if err := normalizeReleaseHook(&hook); err != nil {
		return err
	}
	if err := s.validateHookTask(hook.TenantID, hook.ProjectID, hook.TaskID); err != nil {
		return err
	}
	now := time.Now().Format(timeLayout)
	res, err := s.db.Exec(`UPDATE release_hooks SET task_id=?,name=?,enabled=?,debounce_seconds=?,pending_version=CASE WHEN ? THEN pending_version ELSE '' END,pending_requested_by=CASE WHEN ? THEN pending_requested_by ELSE '' END,pending_source=CASE WHEN ? THEN pending_source ELSE '' END,pending_since=CASE WHEN ? THEN pending_since ELSE '' END,due_at=CASE WHEN ? THEN due_at ELSE '' END,pending_events=CASE WHEN ? THEN pending_events ELSE 0 END,last_status=CASE WHEN ? THEN last_status ELSE 'disabled' END,updated_at=? WHERE tenant_id=? AND project_id=? AND id=?`, hook.TaskID, hook.Name, hook.Enabled, hook.DebounceSeconds, hook.Enabled, hook.Enabled, hook.Enabled, hook.Enabled, hook.Enabled, hook.Enabled, hook.Enabled, now, hook.TenantID, hook.ProjectID, hook.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteReleaseHook(tenantID, projectID, id int64) error {
	res, err := s.db.Exec(`DELETE FROM release_hooks WHERE tenant_id=? AND project_id=? AND id=?`, tenantID, projectID, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) QueueProjectReleaseHooks(tenantID, projectID int64, version, requestedBy, source string, now time.Time) ([]ReleaseHook, error) {
	version, requestedBy, source = strings.TrimSpace(version), strings.TrimSpace(requestedBy), strings.TrimSpace(source)
	if version == "" || requestedBy == "" || !safeReleaseHookSource.MatchString(source) {
		return nil, fmt.Errorf("version, requested_by, and a safe source are required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.Query(`SELECT id,name,debounce_seconds,pending_events FROM release_hooks WHERE tenant_id=? AND project_id=? AND enabled=1 ORDER BY id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	type queuedHook struct {
		id, pending int64
		name        string
		debounce    int
	}
	var selected []queuedHook
	for rows.Next() {
		var item queuedHook
		if err := rows.Scan(&item.id, &item.name, &item.debounce, &item.pending); err != nil {
			_ = rows.Close()
			return nil, err
		}
		selected = append(selected, item)
	}
	_ = rows.Close()
	createdAt := now.Format(timeLayout)
	for _, item := range selected {
		dueAt := now.Add(time.Duration(item.debounce) * time.Second).Format(timeLayout)
		status := "scheduled"
		if item.pending > 0 {
			status = "merged"
		}
		if _, err := tx.Exec(`UPDATE release_hooks SET pending_version=?,pending_requested_by=?,pending_source=?,pending_since=CASE WHEN pending_events=0 THEN ? ELSE pending_since END,due_at=?,pending_events=pending_events+1,trigger_count=trigger_count+1,merge_count=merge_count+CASE WHEN pending_events>0 THEN 1 ELSE 0 END,last_trigger_at=?,last_status='waiting',last_error='',updated_at=? WHERE id=? AND enabled=1`, version, requestedBy, source, createdAt, dueAt, createdAt, createdAt, item.id); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT INTO release_hook_events(tenant_id,project_id,hook_id,hook_name,kind,source,version,status,merged_events,detail,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, tenantID, projectID, item.id, item.name, "trigger", source, version, status, item.pending+1, fmt.Sprintf("due_at=%s", dueAt), createdAt); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	queued := make([]ReleaseHook, 0, len(selected))
	for _, item := range selected {
		hook, err := s.GetReleaseHook(tenantID, projectID, item.id)
		if err != nil {
			return nil, err
		}
		queued = append(queued, *hook)
	}
	return queued, nil
}

func (s *Store) QueueReleaseHook(tenantID, projectID, id int64, version, requestedBy, source string, now time.Time) (*ReleaseHook, error) {
	version, requestedBy, source = strings.TrimSpace(version), strings.TrimSpace(requestedBy), strings.TrimSpace(source)
	if version == "" || requestedBy == "" {
		return nil, fmt.Errorf("version and requested_by are required")
	}
	if source == "" {
		source = "manual"
	}
	if !safeReleaseHookSource.MatchString(source) {
		return nil, fmt.Errorf("source must contain only letters, digits, dot, underscore, colon, or dash and not exceed 40 characters")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var name string
	var debounce int
	var pending int64
	if err := tx.QueryRow(`SELECT name,debounce_seconds,pending_events FROM release_hooks WHERE tenant_id=? AND project_id=? AND id=? AND enabled=1`, tenantID, projectID, id).Scan(&name, &debounce, &pending); err != nil {
		return nil, err
	}
	createdAt := now.Format(timeLayout)
	dueAt := now.Add(time.Duration(debounce) * time.Second).Format(timeLayout)
	status := "scheduled"
	if pending > 0 {
		status = "merged"
	}
	_, err = tx.Exec(`UPDATE release_hooks SET pending_version=?,pending_requested_by=?,pending_source=?,pending_since=CASE WHEN pending_events=0 THEN ? ELSE pending_since END,due_at=?,pending_events=pending_events+1,trigger_count=trigger_count+1,merge_count=merge_count+CASE WHEN pending_events>0 THEN 1 ELSE 0 END,last_trigger_at=?,last_status='waiting',last_error='',updated_at=? WHERE tenant_id=? AND project_id=? AND id=? AND enabled=1`, version, requestedBy, source, createdAt, dueAt, createdAt, createdAt, tenantID, projectID, id)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO release_hook_events(tenant_id,project_id,hook_id,hook_name,kind,source,version,status,merged_events,detail,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, tenantID, projectID, id, name, "trigger", source, version, status, pending+1, fmt.Sprintf("due_at=%s", dueAt), createdAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetReleaseHook(tenantID, projectID, id)
}

func (s *Store) ClaimDueReleaseHooks(now time.Time, limit int) ([]ReleaseHookRun, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.Query(`SELECT id,tenant_id,project_id,name,task_id,pending_version,pending_requested_by,pending_source,pending_events,due_at FROM release_hooks WHERE enabled=1 AND due_at!='' AND due_at<=? ORDER BY due_at,id LIMIT ?`, now.Format(timeLayout), limit)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		id, tenantID, projectID, taskID, events   int64
		name, version, requestedBy, source, dueAt string
	}
	var candidates []candidate
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.tenantID, &item.projectID, &item.name, &item.taskID, &item.version, &item.requestedBy, &item.source, &item.events, &item.dueAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		candidates = append(candidates, item)
	}
	_ = rows.Close()
	runs := make([]ReleaseHookRun, 0, len(candidates))
	claimedAt := now.Format(timeLayout)
	for _, item := range candidates {
		result, err := tx.Exec(`UPDATE release_hooks SET pending_version='',pending_requested_by='',pending_source='',pending_since='',due_at='',pending_events=0,run_count=run_count+1,last_run_at=?,last_status='running',last_error='',updated_at=? WHERE id=? AND enabled=1 AND due_at=?`, claimedAt, claimedAt, item.id, item.dueAt)
		if err != nil {
			return nil, err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			continue
		}
		event, err := tx.Exec(`INSERT INTO release_hook_events(tenant_id,project_id,hook_id,hook_name,kind,source,version,status,merged_events,detail,created_at,requested_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, item.tenantID, item.projectID, item.id, item.name, "dispatch", item.source, item.version, "running", item.events, fmt.Sprintf("coalesced_triggers=%d", item.events), claimedAt, item.requestedBy)
		if err != nil {
			return nil, err
		}
		eventID, _ := event.LastInsertId()
		runs = append(runs, ReleaseHookRun{Hook: ReleaseHook{ID: item.id, TenantID: item.tenantID, ProjectID: item.projectID, TaskID: item.taskID, Name: item.name}, Version: item.version, RequestedBy: item.requestedBy, Source: item.source, EventCount: item.events, EventID: eventID})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *Store) ListStaleReleaseHookRuns(now time.Time, staleAfter time.Duration, limit int) ([]ReleaseHookRun, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	cutoff := now.Add(-staleAfter).Format(timeLayout)
	rows, err := s.db.Query(`SELECT e.id,e.hook_id,h.tenant_id,h.project_id,h.task_id,h.name,e.version,e.requested_by,e.source,e.merged_events FROM release_hook_events e JOIN release_hooks h ON h.id=e.hook_id WHERE e.kind='dispatch' AND e.status='running' AND e.completed_at='' AND e.created_at<=? ORDER BY e.id LIMIT ?`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	runs := make([]ReleaseHookRun, 0)
	for rows.Next() {
		var run ReleaseHookRun
		if err := rows.Scan(&run.EventID, &run.Hook.ID, &run.Hook.TenantID, &run.Hook.ProjectID, &run.Hook.TaskID, &run.Hook.Name, &run.Version, &run.RequestedBy, &run.Source, &run.EventCount); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) CompleteReleaseHookRun(run ReleaseHookRun, deploymentID int64, status, detail string, now time.Time) error {
	completedAt := now.Format(timeLayout)
	if status == "running" {
		completedAt = ""
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE release_hook_events SET status=?,deployment_id=?,detail=?,completed_at=? WHERE id=? AND hook_id=?`, status, deploymentID, detail, completedAt, run.EventID, run.Hook.ID); err != nil {
		return err
	}
	lastError := ""
	if status == "failed" {
		lastError = detail
	}
	if _, err := tx.Exec(`UPDATE release_hooks SET last_deployment_id=?,last_status=?,last_error=?,updated_at=? WHERE id=?`, deploymentID, status, lastError, completedAt, run.Hook.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RecordReleaseHookCancellation(hook ReleaseHook, now time.Time) error {
	if hook.PendingEvents <= 0 {
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO release_hook_events(tenant_id,project_id,hook_id,hook_name,kind,source,version,status,merged_events,detail,created_at,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, hook.TenantID, hook.ProjectID, hook.ID, hook.Name, "dispatch", hook.PendingSource, hook.PendingVersion, "cancelled", hook.PendingEvents, "disabled before debounce window elapsed", now.Format(timeLayout), now.Format(timeLayout))
	return err
}

func (s *Store) ListReleaseHookEvents(tenantID, projectID int64, limit int) ([]ReleaseHookEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT e.id,e.hook_id,e.hook_name,e.kind,e.source,e.version,e.status,e.merged_events,e.deployment_id,COALESCE(d.status,''),e.detail,e.created_at,e.completed_at FROM release_hook_events e LEFT JOIN push_deployments d ON d.id=e.deployment_id WHERE e.tenant_id=? AND e.project_id=? AND e.created_at>=? ORDER BY e.id DESC LIMIT ?`, tenantID, projectID, time.Now().Add(-LogRetention).Format(timeLayout), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]ReleaseHookEvent, 0)
	for rows.Next() {
		var event ReleaseHookEvent
		if err := rows.Scan(&event.ID, &event.HookID, &event.HookName, &event.Kind, &event.Source, &event.Version, &event.Status, &event.MergedEvents, &event.DeploymentID, &event.DeploymentStatus, &event.Detail, &event.CreatedAt, &event.CompletedAt); err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	return items, rows.Err()
}
