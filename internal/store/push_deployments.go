package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PushHost is an SSH endpoint used by the push deployment controller. The
// private key is deliberately never included in API responses.
type PushHost struct {
	ID                         int64  `json:"id"`
	TenantID                   int64  `json:"tenant_id,omitempty"`
	Name                       string `json:"name"`
	Address                    string `json:"address"`
	Port                       int    `json:"port"`
	Username                   string `json:"username"`
	AuthMode                   string `json:"auth_mode"`
	HostKey                    string `json:"host_key"`
	HostKeyFingerprint         string `json:"host_key_fingerprint,omitempty"`
	HealthCheckEnabled         bool   `json:"health_check_enabled"`
	HealthCheckIntervalSeconds int    `json:"health_check_interval_seconds"`
	LastCheckAt                string `json:"last_check_at,omitempty"`
	LastCheckStatus            string `json:"last_check_status"`
	LastCheckError             string `json:"last_check_error,omitempty"`
	LastCheckLatencyMS         int64  `json:"last_check_latency_ms"`
	NextCheckAt                string `json:"next_check_at,omitempty"`
	CreatedAt                  string `json:"created_at"`
	UpdatedAt                  string `json:"updated_at"`
}

// PushHostCheck is an immutable audit record for one SSH connection attempt.
// A failed check is recorded once and is never retried by the checker.
type PushHostCheck struct {
	ID        int64  `json:"id"`
	TenantID  int64  `json:"tenant_id,omitempty"`
	HostID    int64  `json:"host_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
	Trigger   string `json:"trigger"`
	CheckedAt string `json:"checked_at"`
}

const defaultPushHealthCheckIntervalSeconds = 3600
const pushHostCheckRetention = LogRetention

// PushServerBinding describes one independently deployable server on a host.
// Labels select targets (for example test, canary, cn), while ContentTags
// select tagged files inside a version bundle.
type PushServerBinding struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id,omitempty"`
	HostID      int64  `json:"host_id"`
	HostName    string `json:"host_name,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	ServerID    string `json:"server_id"`
	TargetTag   string `json:"target_tag,omitempty"`
	Labels      string `json:"labels"`
	ContentTags string `json:"content_tags,omitempty"`
	RemoteRoot  string `json:"remote_root"`
	OS          string `json:"os,omitempty"`
	Arch        string `json:"arch,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type PushDeployment struct {
	ID             int64                  `json:"id"`
	TenantID       int64                  `json:"tenant_id,omitempty"`
	ProjectID      int64                  `json:"project_id"`
	TaskID         int64                  `json:"task_id,omitempty"`
	TaskName       string                 `json:"task_name,omitempty"`
	HookEventID    int64                  `json:"hook_event_id,omitempty"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	Replayed       bool                   `json:"replayed,omitempty"`
	Version        string                 `json:"version"`
	RequestedBy    string                 `json:"requested_by"`
	Selector       string                 `json:"selector"`
	Status         string                 `json:"status"`
	CreatedAt      string                 `json:"created_at"`
	StartedAt      string                 `json:"started_at,omitempty"`
	CompletedAt    string                 `json:"completed_at,omitempty"`
	Targets        []PushDeploymentTarget `json:"targets,omitempty"`
}

type PushDeploymentTarget struct {
	ID           int64  `json:"id"`
	DeploymentID int64  `json:"deployment_id"`
	HostID       int64  `json:"host_id"`
	HostName     string `json:"host_name"`
	BindingID    int64  `json:"binding_id"`
	ServerID     string `json:"server_id"`
	Labels       string `json:"labels"`
	Status       string `json:"status"`
	Output       string `json:"output,omitempty"`
	CreatedAt    string `json:"created_at"`
	StartedAt    string `json:"started_at,omitempty"`
	CompletedAt  string `json:"completed_at,omitempty"`
}

func (s *Store) migratePushDeployments() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS push_hosts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  address TEXT NOT NULL,
  port INTEGER NOT NULL DEFAULT 22,
  username TEXT NOT NULL,
  auth_mode TEXT NOT NULL DEFAULT 'private_key',
  host_key TEXT NOT NULL DEFAULT '',
  host_key_fingerprint TEXT NOT NULL DEFAULT '',
  encrypted_private_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(tenant_id, name),
  FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS push_server_bindings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  host_id INTEGER NOT NULL,
	project_name TEXT NOT NULL DEFAULT '',
  server_id TEXT NOT NULL,
  labels TEXT NOT NULL DEFAULT 'test',
  content_tags TEXT NOT NULL DEFAULT '',
  remote_root TEXT NOT NULL,
  os TEXT NOT NULL DEFAULT 'linux',
  arch TEXT NOT NULL DEFAULT 'amd64',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
	UNIQUE(host_id, project_name, server_id, labels),
  FOREIGN KEY(host_id) REFERENCES push_hosts(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS push_deployments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  project_id INTEGER NOT NULL,
  task_id INTEGER NOT NULL DEFAULT 0,
  task_name TEXT NOT NULL DEFAULT '',
  hook_event_id INTEGER NOT NULL DEFAULT 0,
  idempotency_key TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL,
  requested_by TEXT NOT NULL,
  selector TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  started_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS push_deployment_tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  project_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  version TEXT NOT NULL DEFAULT '',
  server_ids TEXT NOT NULL DEFAULT '[]',
  tags TEXT NOT NULL DEFAULT '["test"]',
  tag_match TEXT NOT NULL DEFAULT 'all',
  created_by TEXT NOT NULL DEFAULT '',
  run_count INTEGER NOT NULL DEFAULT 0,
  last_run_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(project_id, name),
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS push_deployment_targets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  deployment_id INTEGER NOT NULL,
  host_id INTEGER NOT NULL,
  host_name TEXT NOT NULL,
  binding_id INTEGER NOT NULL,
  server_id TEXT NOT NULL,
  labels TEXT NOT NULL,
  status TEXT NOT NULL,
  output TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  started_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(deployment_id) REFERENCES push_deployments(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS push_host_checks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  host_id INTEGER NOT NULL,
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  latency_ms INTEGER NOT NULL DEFAULT 0,
  trigger TEXT NOT NULL,
  checked_at TEXT NOT NULL,
  FOREIGN KEY(host_id) REFERENCES push_hosts(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_push_bindings_tenant_server ON push_server_bindings(tenant_id, server_id);
CREATE INDEX IF NOT EXISTS idx_push_deployments_project ON push_deployments(project_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_push_tasks_project ON push_deployment_tasks(project_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_push_targets_deployment ON push_deployment_targets(deployment_id, id);
CREATE INDEX IF NOT EXISTS idx_push_host_checks_host ON push_host_checks(host_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_push_host_checks_checked_at ON push_host_checks(checked_at);
`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN auth_mode TEXT NOT NULL DEFAULT 'private_key'`)
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN host_key_fingerprint TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN health_check_enabled INTEGER NOT NULL DEFAULT 1`)
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN health_check_interval_seconds INTEGER NOT NULL DEFAULT 3600`)
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN last_check_at TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN last_check_status TEXT NOT NULL DEFAULT 'unknown'`)
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN last_check_error TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN last_check_latency_ms INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE push_hosts ADD COLUMN next_check_at TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE push_server_bindings ADD COLUMN project_name TEXT NOT NULL DEFAULT ''`)
	if err := s.migratePushBindingTargetTags(); err != nil {
		return err
	}
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_push_bindings_tenant_server ON push_server_bindings(tenant_id,server_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_push_bindings_tenant_project_server ON push_server_bindings(tenant_id,project_name,server_id)`)
	_, _ = s.db.Exec(`ALTER TABLE push_deployments ADD COLUMN task_id INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE push_deployments ADD COLUMN task_name TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE push_deployments ADD COLUMN hook_event_id INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE push_deployments ADD COLUMN idempotency_key TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_push_deployments_hook_event ON push_deployments(hook_event_id) WHERE hook_event_id>0`)
	_, _ = s.db.Exec(`DROP INDEX IF EXISTS idx_push_deployments_idempotency`)
	_, _ = s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_push_deployments_idempotency ON push_deployments(tenant_id,project_id,idempotency_key) WHERE idempotency_key<>''`)
	_, _ = s.db.Exec(`UPDATE push_hosts SET next_check_at=? WHERE health_check_enabled=1 AND next_check_at=''`, time.Now().Format(timeLayout))
	return nil
}

func (s *Store) migratePushBindingTargetTags() error {
	var tableSQL string
	if err := s.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='push_server_bindings'`).Scan(&tableSQL); err != nil {
		return err
	}
	if !strings.Contains(tableSQL, "UNIQUE(host_id, server_id, labels)") {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	statements := []string{
		`ALTER TABLE push_server_bindings RENAME TO push_server_bindings_legacy`,
		`CREATE TABLE push_server_bindings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  host_id INTEGER NOT NULL,
  project_name TEXT NOT NULL DEFAULT '',
  server_id TEXT NOT NULL,
  labels TEXT NOT NULL DEFAULT 'test',
  content_tags TEXT NOT NULL DEFAULT '',
  remote_root TEXT NOT NULL,
  os TEXT NOT NULL DEFAULT 'linux',
  arch TEXT NOT NULL DEFAULT 'amd64',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(host_id, project_name, server_id, labels),
  FOREIGN KEY(host_id) REFERENCES push_hosts(id) ON DELETE CASCADE
)`,
		`INSERT INTO push_server_bindings(id,tenant_id,host_id,project_name,server_id,labels,content_tags,remote_root,os,arch,created_at,updated_at)
SELECT id,tenant_id,host_id,project_name,server_id,labels,content_tags,remote_root,os,arch,created_at,updated_at FROM push_server_bindings_legacy`,
		`DROP TABLE push_server_bindings_legacy`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate push target tags: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) CreatePushHost(host PushHost, encryptedPrivateKey string) (*PushHost, error) {
	if host.Port == 0 {
		host.Port = 22
	}
	if host.AuthMode == "" {
		host.AuthMode = "private_key"
	}
	if host.HealthCheckIntervalSeconds == 0 {
		host.HealthCheckIntervalSeconds = defaultPushHealthCheckIntervalSeconds
	}
	if !validPushHealthCheckInterval(host.HealthCheckIntervalSeconds) {
		return nil, fmt.Errorf("health_check_interval_seconds must be between 60 and 604800")
	}
	if strings.TrimSpace(host.Name) == "" || strings.TrimSpace(host.Address) == "" || strings.TrimSpace(host.Username) == "" || !validPushAuthMode(host.AuthMode) || (host.AuthMode != "agent" && encryptedPrivateKey == "") {
		return nil, fmt.Errorf("name, address, username, auth_mode and credential required")
	}
	now := time.Now().Format(timeLayout)
	nextCheckAt := ""
	if host.HealthCheckEnabled {
		nextCheckAt = now
	}
	res, err := s.db.Exec(`INSERT INTO push_hosts(tenant_id,name,address,port,username,auth_mode,host_key,host_key_fingerprint,encrypted_private_key,health_check_enabled,health_check_interval_seconds,last_check_status,next_check_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, host.TenantID, host.Name, host.Address, host.Port, host.Username, host.AuthMode, host.HostKey, host.HostKeyFingerprint, encryptedPrivateKey, host.HealthCheckEnabled, host.HealthCheckIntervalSeconds, "unknown", nextCheckAt, now, now)
	if err != nil {
		return nil, err
	}
	host.ID, _ = res.LastInsertId()
	host.LastCheckStatus = "unknown"
	host.NextCheckAt = nextCheckAt
	host.CreatedAt, host.UpdatedAt = now, now
	return &host, nil
}

func (s *Store) ListPushHosts(tenantID int64) ([]PushHost, error) {
	rows, err := s.db.Query(`SELECT id,tenant_id,name,address,port,username,auth_mode,host_key,host_key_fingerprint,health_check_enabled,health_check_interval_seconds,last_check_at,last_check_status,last_check_error,last_check_latency_ms,next_check_at,created_at,updated_at FROM push_hosts WHERE tenant_id=? ORDER BY name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []PushHost
	for rows.Next() {
		var h PushHost
		if err := scanPushHost(rows, &h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) GetPushHost(tenantID, id int64) (*PushHost, string, error) {
	var h PushHost
	var key string
	err := s.db.QueryRow(`SELECT id,tenant_id,name,address,port,username,auth_mode,host_key,host_key_fingerprint,health_check_enabled,health_check_interval_seconds,last_check_at,last_check_status,last_check_error,last_check_latency_ms,next_check_at,encrypted_private_key,created_at,updated_at FROM push_hosts WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&h.ID, &h.TenantID, &h.Name, &h.Address, &h.Port, &h.Username, &h.AuthMode, &h.HostKey, &h.HostKeyFingerprint, &h.HealthCheckEnabled, &h.HealthCheckIntervalSeconds, &h.LastCheckAt, &h.LastCheckStatus, &h.LastCheckError, &h.LastCheckLatencyMS, &h.NextCheckAt, &key, &h.CreatedAt, &h.UpdatedAt)
	return &h, key, err
}

func (s *Store) UpdatePushHost(host PushHost, encryptedPrivateKey string) error {
	if host.Port == 0 {
		host.Port = 22
	}
	if host.AuthMode == "" {
		host.AuthMode = "private_key"
	}
	if host.HealthCheckIntervalSeconds == 0 {
		host.HealthCheckIntervalSeconds = defaultPushHealthCheckIntervalSeconds
	}
	if !validPushHealthCheckInterval(host.HealthCheckIntervalSeconds) {
		return fmt.Errorf("health_check_interval_seconds must be between 60 and 604800")
	}
	if host.HostKey == "" {
		current, _, err := s.GetPushHost(host.TenantID, host.ID)
		if err != nil {
			return err
		}
		host.HostKey, host.HostKeyFingerprint = current.HostKey, current.HostKeyFingerprint
	}
	if strings.TrimSpace(host.Name) == "" || strings.TrimSpace(host.Address) == "" || strings.TrimSpace(host.Username) == "" || !validPushAuthMode(host.AuthMode) {
		return fmt.Errorf("name, address, username and auth_mode required")
	}
	now := time.Now().Format(timeLayout)
	nextCheckAt := ""
	if host.HealthCheckEnabled {
		nextCheckAt = now
		if current, _, err := s.GetPushHost(host.TenantID, host.ID); err == nil && current.HealthCheckEnabled && current.HealthCheckIntervalSeconds == host.HealthCheckIntervalSeconds && current.NextCheckAt != "" {
			nextCheckAt = current.NextCheckAt
		}
	}
	query := `UPDATE push_hosts SET name=?,address=?,port=?,username=?,auth_mode=?,host_key=?,host_key_fingerprint=?,health_check_enabled=?,health_check_interval_seconds=?,next_check_at=?,updated_at=? WHERE tenant_id=? AND id=?`
	args := []any{host.Name, host.Address, host.Port, host.Username, host.AuthMode, host.HostKey, host.HostKeyFingerprint, host.HealthCheckEnabled, host.HealthCheckIntervalSeconds, nextCheckAt, now, host.TenantID, host.ID}
	if encryptedPrivateKey != "" {
		query = `UPDATE push_hosts SET name=?,address=?,port=?,username=?,auth_mode=?,host_key=?,host_key_fingerprint=?,encrypted_private_key=?,health_check_enabled=?,health_check_interval_seconds=?,next_check_at=?,updated_at=? WHERE tenant_id=? AND id=?`
		args = []any{host.Name, host.Address, host.Port, host.Username, host.AuthMode, host.HostKey, host.HostKeyFingerprint, encryptedPrivateKey, host.HealthCheckEnabled, host.HealthCheckIntervalSeconds, nextCheckAt, now, host.TenantID, host.ID}
	}
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeletePushHost(tenantID, id int64) error {
	res, err := s.db.Exec(`DELETE FROM push_hosts WHERE tenant_id=? AND id=?`, tenantID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type pushHostScanner interface {
	Scan(dest ...any) error
}

func scanPushHost(row pushHostScanner, h *PushHost) error {
	return row.Scan(&h.ID, &h.TenantID, &h.Name, &h.Address, &h.Port, &h.Username, &h.AuthMode, &h.HostKey, &h.HostKeyFingerprint, &h.HealthCheckEnabled, &h.HealthCheckIntervalSeconds, &h.LastCheckAt, &h.LastCheckStatus, &h.LastCheckError, &h.LastCheckLatencyMS, &h.NextCheckAt, &h.CreatedAt, &h.UpdatedAt)
}

func validPushHealthCheckInterval(seconds int) bool {
	return seconds >= 60 && seconds <= 7*24*60*60
}

// ListDuePushHosts returns enabled hosts whose next single-attempt check is due.
// The global scheduler deliberately processes this list serially.
func (s *Store) ListDuePushHosts(now time.Time, limit int) ([]PushHost, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id,tenant_id,name,address,port,username,auth_mode,host_key,host_key_fingerprint,health_check_enabled,health_check_interval_seconds,last_check_at,last_check_status,last_check_error,last_check_latency_ms,next_check_at,created_at,updated_at FROM push_hosts WHERE health_check_enabled=1 AND (next_check_at='' OR next_check_at<=?) ORDER BY next_check_at,id LIMIT ?`, now.Format(timeLayout), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]PushHost, 0)
	for rows.Next() {
		var host PushHost
		if err := scanPushHost(rows, &host); err != nil {
			return nil, err
		}
		items = append(items, host)
	}
	return items, rows.Err()
}

// RecordPushHostCheck persists one attempt and schedules only the next interval;
// it never creates an immediate retry after failure.
func (s *Store) RecordPushHostCheck(check PushHostCheck) (*PushHostCheck, error) {
	if check.Status != "ok" && check.Status != "failed" {
		return nil, fmt.Errorf("invalid SSH check status")
	}
	if check.Trigger != "manual" && check.Trigger != "scheduled" {
		return nil, fmt.Errorf("invalid SSH check trigger")
	}
	if len(check.Error) > 1000 {
		check.Error = check.Error[:1000]
	}
	checkedAt := time.Now()
	check.CheckedAt = checkedAt.Format(timeLayout)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var interval int
	var enabled bool
	if err := tx.QueryRow(`SELECT health_check_interval_seconds,health_check_enabled FROM push_hosts WHERE tenant_id=? AND id=?`, check.TenantID, check.HostID).Scan(&interval, &enabled); err != nil {
		return nil, err
	}
	res, err := tx.Exec(`INSERT INTO push_host_checks(tenant_id,host_id,status,error,latency_ms,trigger,checked_at) VALUES(?,?,?,?,?,?,?)`, check.TenantID, check.HostID, check.Status, check.Error, check.LatencyMS, check.Trigger, check.CheckedAt)
	if err != nil {
		return nil, err
	}
	check.ID, _ = res.LastInsertId()
	if _, err := tx.Exec(`DELETE FROM push_host_checks WHERE checked_at<?`, checkedAt.Add(-pushHostCheckRetention).Format(timeLayout)); err != nil {
		return nil, err
	}
	nextCheckAt := ""
	if enabled {
		if !validPushHealthCheckInterval(interval) {
			interval = defaultPushHealthCheckIntervalSeconds
		}
		nextCheckAt = checkedAt.Add(time.Duration(interval) * time.Second).Format(timeLayout)
	}
	if _, err := tx.Exec(`UPDATE push_hosts SET last_check_at=?,last_check_status=?,last_check_error=?,last_check_latency_ms=?,next_check_at=? WHERE tenant_id=? AND id=?`, check.CheckedAt, check.Status, check.Error, check.LatencyMS, nextCheckAt, check.TenantID, check.HostID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &check, nil
}

func (s *Store) ListPushHostChecks(tenantID, hostID int64, limit int) ([]PushHostCheck, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM push_hosts WHERE tenant_id=? AND id=?`, tenantID, hostID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, sql.ErrNoRows
	}
	rows, err := s.db.Query(`SELECT id,tenant_id,host_id,status,error,latency_ms,trigger,checked_at FROM push_host_checks WHERE tenant_id=? AND host_id=? ORDER BY id DESC LIMIT ?`, tenantID, hostID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]PushHostCheck, 0)
	for rows.Next() {
		var item PushHostCheck
		if err := rows.Scan(&item.ID, &item.TenantID, &item.HostID, &item.Status, &item.Error, &item.LatencyMS, &item.Trigger, &item.CheckedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreatePushServerBinding(b PushServerBinding) (*PushServerBinding, error) {
	if strings.TrimSpace(b.ServerID) == "" || strings.TrimSpace(b.RemoteRoot) == "" {
		return nil, fmt.Errorf("server_id and remote_root required")
	}
	if b.Labels == "" {
		b.Labels = "test"
	}
	if b.OS == "" {
		b.OS = "linux"
	}
	if b.Arch == "" {
		b.Arch = "amd64"
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM push_hosts WHERE tenant_id=? AND id=?`, b.TenantID, b.HostID).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, sql.ErrNoRows
	}
	now := time.Now().Format(timeLayout)
	res, err := s.db.Exec(`INSERT INTO push_server_bindings(tenant_id,host_id,project_name,server_id,labels,content_tags,remote_root,os,arch,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, b.TenantID, b.HostID, strings.TrimSpace(b.ProjectName), b.ServerID, normaliseLabels(b.Labels), normaliseLabels(b.ContentTags), b.RemoteRoot, b.OS, b.Arch, now, now)
	if err != nil {
		return nil, err
	}
	b.ID, _ = res.LastInsertId()
	b.CreatedAt, b.UpdatedAt = now, now
	b.TargetTag = pushTargetTag(b.ProjectName, b.ServerID)
	return &b, nil
}

func (s *Store) ListPushServerBindings(tenantID, hostID int64) ([]PushServerBinding, error) {
	rows, err := s.db.Query(`SELECT b.id,b.tenant_id,b.host_id,h.name,b.project_name,b.server_id,b.labels,b.content_tags,b.remote_root,b.os,b.arch,b.created_at,b.updated_at FROM push_server_bindings b JOIN push_hosts h ON h.id=b.host_id WHERE b.tenant_id=? AND b.host_id=? ORDER BY b.project_name,b.server_id`, tenantID, hostID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []PushServerBinding
	for rows.Next() {
		var b PushServerBinding
		if err := rows.Scan(&b.ID, &b.TenantID, &b.HostID, &b.HostName, &b.ProjectName, &b.ServerID, &b.Labels, &b.ContentTags, &b.RemoteRoot, &b.OS, &b.Arch, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.TargetTag = pushTargetTag(b.ProjectName, b.ServerID)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) ListPushBindingsForSelector(tenantID int64, projectName string, serverIDs, labels []string, all bool) ([]PushServerBinding, error) {
	rows, err := s.db.Query(`SELECT b.id,b.tenant_id,b.host_id,h.name,b.project_name,b.server_id,b.labels,b.content_tags,b.remote_root,b.os,b.arch,b.created_at,b.updated_at FROM push_server_bindings b JOIN push_hosts h ON h.id=b.host_id WHERE b.tenant_id=? ORDER BY h.name,b.project_name,b.server_id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []PushServerBinding
	serverSet := make(map[string]bool, len(serverIDs))
	for _, id := range serverIDs {
		serverSet[strings.TrimSpace(id)] = true
	}
	for rows.Next() {
		var b PushServerBinding
		if err := rows.Scan(&b.ID, &b.TenantID, &b.HostID, &b.HostName, &b.ProjectName, &b.ServerID, &b.Labels, &b.ContentTags, &b.RemoteRoot, &b.OS, &b.Arch, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		if b.ProjectName != "" && projectName != "" && b.ProjectName != projectName {
			continue
		}
		if len(serverSet) > 0 && !serverSet[b.ServerID] {
			continue
		}
		if labelsMatch(splitLabels(b.Labels), labels, all) {
			b.TargetTag = pushTargetTag(b.ProjectName, b.ServerID)
			out = append(out, b)
		}
	}
	return out, rows.Err()
}

func (s *Store) UpdatePushServerBinding(b PushServerBinding) error {
	if strings.TrimSpace(b.ServerID) == "" || strings.TrimSpace(b.RemoteRoot) == "" {
		return fmt.Errorf("server_id and remote_root required")
	}
	if b.Labels == "" {
		b.Labels = "test"
	}
	if b.OS == "" {
		b.OS = "linux"
	}
	if b.Arch == "" {
		b.Arch = "amd64"
	}
	res, err := s.db.Exec(`UPDATE push_server_bindings SET project_name=?,server_id=?,labels=?,content_tags=?,remote_root=?,os=?,arch=?,updated_at=? WHERE tenant_id=? AND host_id=? AND id=?`, strings.TrimSpace(b.ProjectName), b.ServerID, normaliseLabels(b.Labels), normaliseLabels(b.ContentTags), b.RemoteRoot, b.OS, b.Arch, time.Now().Format(timeLayout), b.TenantID, b.HostID, b.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (s *Store) DeletePushServerBinding(tenantID, hostID, id int64) error {
	res, err := s.db.Exec(`DELETE FROM push_server_bindings WHERE tenant_id=? AND host_id=? AND id=?`, tenantID, hostID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreatePushDeployment(d PushDeployment, targets []PushServerBinding) (*PushDeployment, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets matched selector")
	}
	if d.IdempotencyKey != "" {
		existing, err := s.GetPushDeploymentByIdempotencyKey(d.TenantID, d.ProjectID, d.IdempotencyKey)
		if err == nil {
			existing.Replayed = true
			return existing, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}
	now := time.Now().Format(timeLayout)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.Exec(`INSERT INTO push_deployments(tenant_id,project_id,task_id,task_name,hook_event_id,idempotency_key,version,requested_by,selector,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, d.TenantID, d.ProjectID, d.TaskID, d.TaskName, d.HookEventID, d.IdempotencyKey, d.Version, d.RequestedBy, d.Selector, "queued", now)
	if err != nil {
		if d.IdempotencyKey != "" {
			_ = tx.Rollback()
			if existing, lookupErr := s.GetPushDeploymentByIdempotencyKey(d.TenantID, d.ProjectID, d.IdempotencyKey); lookupErr == nil {
				existing.Replayed = true
				return existing, nil
			}
		}
		return nil, err
	}
	d.ID, _ = res.LastInsertId()
	d.Status, d.CreatedAt = "queued", now
	for _, b := range targets {
		if _, err := tx.Exec(`INSERT INTO push_deployment_targets(deployment_id,host_id,host_name,binding_id,server_id,labels,status,created_at) VALUES(?,?,?,?,?,?,?,?)`, d.ID, b.HostID, b.HostName, b.ID, b.ServerID, b.Labels, "queued", now); err != nil {
			return nil, err
		}
	}
	if d.TaskID > 0 {
		if _, err := tx.Exec(`UPDATE push_deployment_tasks SET run_count=run_count+1,last_run_at=?,updated_at=? WHERE tenant_id=? AND project_id=? AND id=?`, now, now, d.TenantID, d.ProjectID, d.TaskID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) ListPushDeployments(tenantID, projectID int64, limit int) ([]PushDeployment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id,tenant_id,project_id,task_id,task_name,hook_event_id,idempotency_key,version,requested_by,selector,status,created_at,started_at,completed_at FROM push_deployments WHERE tenant_id=? AND project_id=? AND created_at>=? ORDER BY id DESC LIMIT ?`, tenantID, projectID, time.Now().Add(-LogRetention).Format(timeLayout), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]PushDeployment, 0)
	for rows.Next() {
		var d PushDeployment
		if err := rows.Scan(&d.ID, &d.TenantID, &d.ProjectID, &d.TaskID, &d.TaskName, &d.HookEventID, &d.IdempotencyKey, &d.Version, &d.RequestedBy, &d.Selector, &d.Status, &d.CreatedAt, &d.StartedAt, &d.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) GetPushDeployment(tenantID, projectID, id int64) (*PushDeployment, error) {
	var d PushDeployment
	err := s.db.QueryRow(`SELECT id,tenant_id,project_id,task_id,task_name,hook_event_id,idempotency_key,version,requested_by,selector,status,created_at,started_at,completed_at FROM push_deployments WHERE tenant_id=? AND project_id=? AND id=? AND created_at>=?`, tenantID, projectID, id, time.Now().Add(-LogRetention).Format(timeLayout)).Scan(&d.ID, &d.TenantID, &d.ProjectID, &d.TaskID, &d.TaskName, &d.HookEventID, &d.IdempotencyKey, &d.Version, &d.RequestedBy, &d.Selector, &d.Status, &d.CreatedAt, &d.StartedAt, &d.CompletedAt)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id,deployment_id,host_id,host_name,binding_id,server_id,labels,status,output,created_at,started_at,completed_at FROM push_deployment_targets WHERE deployment_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var t PushDeploymentTarget
		if err := rows.Scan(&t.ID, &t.DeploymentID, &t.HostID, &t.HostName, &t.BindingID, &t.ServerID, &t.Labels, &t.Status, &t.Output, &t.CreatedAt, &t.StartedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		d.Targets = append(d.Targets, t)
	}
	return &d, rows.Err()
}
func (s *Store) GetPushDeploymentByHookEvent(eventID int64) (*PushDeployment, error) {
	var tenantID, projectID, id int64
	if err := s.db.QueryRow(`SELECT id,tenant_id,project_id FROM push_deployments WHERE hook_event_id=?`, eventID).Scan(&id, &tenantID, &projectID); err != nil {
		return nil, err
	}
	return s.GetPushDeployment(tenantID, projectID, id)
}
func (s *Store) GetPushDeploymentByIdempotencyKey(tenantID, projectID int64, key string) (*PushDeployment, error) {
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM push_deployments WHERE tenant_id=? AND project_id=? AND idempotency_key=?`, tenantID, projectID, key).Scan(&id); err != nil {
		return nil, err
	}
	return s.GetPushDeployment(tenantID, projectID, id)
}
func (s *Store) GetPushDeploymentByID(id int64) (*PushDeployment, error) {
	var tenantID, projectID int64
	if err := s.db.QueryRow(`SELECT tenant_id,project_id FROM push_deployments WHERE id=?`, id).Scan(&tenantID, &projectID); err != nil {
		return nil, err
	}
	return s.GetPushDeployment(tenantID, projectID, id)
}
func (s *Store) GetPushBinding(tenantID, id int64) (*PushServerBinding, error) {
	var b PushServerBinding
	err := s.db.QueryRow(`SELECT b.id,b.tenant_id,b.host_id,h.name,b.project_name,b.server_id,b.labels,b.content_tags,b.remote_root,b.os,b.arch,b.created_at,b.updated_at FROM push_server_bindings b JOIN push_hosts h ON h.id=b.host_id WHERE b.tenant_id=? AND b.id=?`, tenantID, id).Scan(&b.ID, &b.TenantID, &b.HostID, &b.HostName, &b.ProjectName, &b.ServerID, &b.Labels, &b.ContentTags, &b.RemoteRoot, &b.OS, &b.Arch, &b.CreatedAt, &b.UpdatedAt)
	b.TargetTag = pushTargetTag(b.ProjectName, b.ServerID)
	return &b, err
}

func pushTargetTag(projectName, serverID string) string {
	if strings.TrimSpace(projectName) == "" {
		return strings.TrimSpace(serverID)
	}
	return strings.TrimSpace(projectName) + "|" + strings.TrimSpace(serverID)
}
func (s *Store) MarkPushDeploymentRunning(id int64) error {
	_, err := s.db.Exec(`UPDATE push_deployments SET status='running',started_at=? WHERE id=? AND status='queued'`, time.Now().Format(timeLayout), id)
	return err
}
func (s *Store) MarkPushTarget(id int64, status, output string) error {
	now := time.Now().Format(timeLayout)
	if status == "running" {
		_, err := s.db.Exec(`UPDATE push_deployment_targets SET status=?,started_at=? WHERE id=?`, status, now, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE push_deployment_targets SET status=?,output=?,completed_at=? WHERE id=?`, status, output, now, id)
	return err
}
func (s *Store) CompletePushDeployment(id int64) error {
	var failures int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM push_deployment_targets WHERE deployment_id=? AND status!='success'`, id).Scan(&failures)
	if err != nil {
		return err
	}
	status := "success"
	if failures > 0 {
		status = "failed"
	}
	_, err = s.db.Exec(`UPDATE push_deployments SET status=?,completed_at=? WHERE id=?`, status, time.Now().Format(timeLayout), id)
	return err
}
func (s *Store) DeletePushDeployment(tenantID, projectID, id int64) error {
	res, err := s.db.Exec(`DELETE FROM push_deployments WHERE tenant_id=? AND project_id=? AND id=?`, tenantID, projectID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func splitLabels(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func normaliseLabels(s string) string { return strings.Join(splitLabels(s), ",") }
func validPushAuthMode(mode string) bool {
	return mode == "password" || mode == "private_key" || mode == "agent"
}

// RecordPushHostKey implements trust-on-first-use. It only fills an empty
// value and can never silently replace an operator-approved fingerprint.
func (s *Store) RecordPushHostKey(tenantID, hostID int64, publicKey, fingerprint string) error {
	_, err := s.db.Exec(`UPDATE push_hosts SET host_key=?,host_key_fingerprint=?,updated_at=? WHERE tenant_id=? AND id=? AND host_key=''`, publicKey, fingerprint, time.Now().Format(timeLayout), tenantID, hostID)
	return err
}
func labelsMatch(have, want []string, all bool) bool {
	if len(want) == 0 {
		return true
	}
	set := map[string]bool{}
	for _, v := range have {
		set[v] = true
	}
	matched := 0
	for _, v := range want {
		if set[strings.TrimSpace(v)] {
			matched++
		} else if all {
			return false
		}
	}
	return matched > 0
}
