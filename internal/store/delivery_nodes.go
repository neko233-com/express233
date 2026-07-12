package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var safeDeliveryServerID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var safeDeliveryMetadata = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// DeliveryNode is a pull-mode agent registered with the control plane. Push
// nodes remain sourced from SSH bindings and are projected alongside these by
// the API, giving operators one cluster inventory without duplicating secrets.
type DeliveryNode struct {
	ID                       int64    `json:"id"`
	TenantID                 int64    `json:"tenant_id,omitempty"`
	ProjectID                int64    `json:"project_id"`
	ServerID                 string   `json:"server_id"`
	DeliveryMode             string   `json:"delivery_mode"`
	Role                     string   `json:"role,omitempty"`
	Environment              string   `json:"environment,omitempty"`
	Labels                   []string `json:"labels"`
	OS                       string   `json:"os,omitempty"`
	Arch                     string   `json:"arch,omitempty"`
	CurrentVersion           string   `json:"current_version,omitempty"`
	DesiredVersion           string   `json:"desired_version,omitempty"`
	DesiredGeneration        int64    `json:"desired_generation"`
	AppliedGeneration        int64    `json:"applied_generation"`
	AutoFollow               bool     `json:"auto_follow"`
	Status                   string   `json:"status"`
	LastError                string   `json:"last_error,omitempty"`
	HeartbeatIntervalSeconds int      `json:"heartbeat_interval_seconds"`
	LastSeenAt               string   `json:"last_seen_at,omitempty"`
	CreatedAt                string   `json:"created_at"`
	UpdatedAt                string   `json:"updated_at"`
}

type DeliveryNodeHeartbeat struct {
	TenantID                 int64
	ProjectID                int64
	ServerID                 string
	Role                     string
	Environment              string
	Labels                   []string
	OS                       string
	Arch                     string
	CurrentVersion           string
	AppliedGeneration        int64
	Status                   string
	LastError                string
	HeartbeatIntervalSeconds int
}

func (s *Store) migrateDeliveryNodes() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS delivery_nodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  project_id INTEGER NOT NULL,
  server_id TEXT NOT NULL,
  delivery_mode TEXT NOT NULL DEFAULT 'pull',
  role TEXT NOT NULL DEFAULT '',
  environment TEXT NOT NULL DEFAULT '',
  labels TEXT NOT NULL DEFAULT '',
  os TEXT NOT NULL DEFAULT '',
  arch TEXT NOT NULL DEFAULT '',
  current_version TEXT NOT NULL DEFAULT '',
  desired_version TEXT NOT NULL DEFAULT '',
  desired_generation INTEGER NOT NULL DEFAULT 0,
  applied_generation INTEGER NOT NULL DEFAULT 0,
  auto_follow INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'unknown',
  last_error TEXT NOT NULL DEFAULT '',
  heartbeat_interval_seconds INTEGER NOT NULL DEFAULT 60,
  last_seen_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(tenant_id, project_id, server_id, delivery_mode),
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_delivery_nodes_project ON delivery_nodes(project_id, delivery_mode, server_id);
CREATE INDEX IF NOT EXISTS idx_delivery_nodes_last_seen ON delivery_nodes(last_seen_at);
`)
	return err
}

const deliveryNodeColumns = `id,tenant_id,project_id,server_id,delivery_mode,role,environment,labels,os,arch,current_version,desired_version,desired_generation,applied_generation,auto_follow,status,last_error,heartbeat_interval_seconds,last_seen_at,created_at,updated_at`

func scanDeliveryNode(scanner pushHostScanner, node *DeliveryNode) error {
	var labels string
	if err := scanner.Scan(&node.ID, &node.TenantID, &node.ProjectID, &node.ServerID, &node.DeliveryMode, &node.Role, &node.Environment, &labels, &node.OS, &node.Arch, &node.CurrentVersion, &node.DesiredVersion, &node.DesiredGeneration, &node.AppliedGeneration, &node.AutoFollow, &node.Status, &node.LastError, &node.HeartbeatIntervalSeconds, &node.LastSeenAt, &node.CreatedAt, &node.UpdatedAt); err != nil {
		return err
	}
	node.Labels = splitLabels(labels)
	return nil
}

func normalizeDeliveryHeartbeat(heartbeat *DeliveryNodeHeartbeat) error {
	heartbeat.ServerID = strings.TrimSpace(heartbeat.ServerID)
	heartbeat.Role = strings.TrimSpace(heartbeat.Role)
	heartbeat.Environment = strings.TrimSpace(heartbeat.Environment)
	heartbeat.OS = strings.TrimSpace(heartbeat.OS)
	heartbeat.Arch = strings.TrimSpace(heartbeat.Arch)
	heartbeat.CurrentVersion = strings.TrimSpace(heartbeat.CurrentVersion)
	heartbeat.Status = strings.TrimSpace(heartbeat.Status)
	heartbeat.LastError = strings.TrimSpace(heartbeat.LastError)
	if !safeDeliveryServerID.MatchString(heartbeat.ServerID) {
		return fmt.Errorf("invalid server_id")
	}
	for _, value := range []string{heartbeat.Role, heartbeat.Environment, heartbeat.OS, heartbeat.Arch} {
		if value != "" && !safeDeliveryMetadata.MatchString(value) {
			return fmt.Errorf("role, environment, os, and arch must use safe identifiers")
		}
	}
	if heartbeat.CurrentVersion != "" && !safeDeliveryMetadata.MatchString(heartbeat.CurrentVersion) {
		return fmt.Errorf("current_version must use a safe identifier")
	}
	if heartbeat.Status == "" {
		heartbeat.Status = "ok"
	}
	if heartbeat.Status != "ok" && heartbeat.Status != "error" && heartbeat.Status != "deploying" {
		return fmt.Errorf("status must be ok, deploying, or error")
	}
	if len(heartbeat.LastError) > 500 {
		heartbeat.LastError = heartbeat.LastError[:500]
	}
	if heartbeat.HeartbeatIntervalSeconds == 0 {
		heartbeat.HeartbeatIntervalSeconds = 60
	}
	if heartbeat.HeartbeatIntervalSeconds < 10 || heartbeat.HeartbeatIntervalSeconds > 3600 {
		return fmt.Errorf("heartbeat_interval_seconds must be between 10 and 3600")
	}
	heartbeat.Labels = normalizeTaskValues(heartbeat.Labels)
	if len(heartbeat.Labels) > 32 {
		return fmt.Errorf("labels must not contain more than 32 values")
	}
	for _, label := range heartbeat.Labels {
		if !safeDeliveryMetadata.MatchString(label) {
			return fmt.Errorf("labels must use safe identifiers")
		}
	}
	return nil
}

func (s *Store) HeartbeatDeliveryNode(heartbeat DeliveryNodeHeartbeat, now time.Time) (*DeliveryNode, error) {
	if err := normalizeDeliveryHeartbeat(&heartbeat); err != nil {
		return nil, err
	}
	stamp := now.Format(timeLayout)
	labels := normaliseLabels(strings.Join(heartbeat.Labels, ","))
	// The generation never moves backwards. Older agents can report a stale
	// state after a restart; accepting it would make the console believe a newer
	// rollout regressed. A matching desired version is allowed to acknowledge
	// the generation even when an older agent did not persist it locally yet.
	_, err := s.db.Exec(`INSERT INTO delivery_nodes(tenant_id,project_id,server_id,delivery_mode,role,environment,labels,os,arch,current_version,applied_generation,status,last_error,heartbeat_interval_seconds,last_seen_at,created_at,updated_at) VALUES(?,?,?,'pull',?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,project_id,server_id,delivery_mode) DO UPDATE SET role=excluded.role,environment=excluded.environment,labels=excluded.labels,os=excluded.os,arch=excluded.arch,current_version=excluded.current_version,applied_generation=CASE WHEN excluded.applied_generation>delivery_nodes.applied_generation THEN excluded.applied_generation WHEN excluded.current_version=delivery_nodes.desired_version AND delivery_nodes.desired_generation>delivery_nodes.applied_generation THEN delivery_nodes.desired_generation ELSE delivery_nodes.applied_generation END,status=excluded.status,last_error=excluded.last_error,heartbeat_interval_seconds=excluded.heartbeat_interval_seconds,last_seen_at=excluded.last_seen_at,updated_at=excluded.updated_at`, heartbeat.TenantID, heartbeat.ProjectID, heartbeat.ServerID, heartbeat.Role, heartbeat.Environment, labels, heartbeat.OS, heartbeat.Arch, heartbeat.CurrentVersion, heartbeat.AppliedGeneration, heartbeat.Status, heartbeat.LastError, heartbeat.HeartbeatIntervalSeconds, stamp, stamp, stamp)
	if err != nil {
		return nil, err
	}
	return s.GetDeliveryNode(heartbeat.TenantID, heartbeat.ProjectID, heartbeat.ServerID)
}

func (s *Store) GetDeliveryNode(tenantID, projectID int64, serverID string) (*DeliveryNode, error) {
	var node DeliveryNode
	err := scanDeliveryNode(s.db.QueryRow(`SELECT `+deliveryNodeColumns+` FROM delivery_nodes WHERE tenant_id=? AND project_id=? AND server_id=? AND delivery_mode='pull'`, tenantID, projectID, serverID), &node)
	return &node, err
}

func (s *Store) ListDeliveryNodes(tenantID, projectID int64) ([]DeliveryNode, error) {
	rows, err := s.db.Query(`SELECT `+deliveryNodeColumns+` FROM delivery_nodes WHERE tenant_id=? AND project_id=? ORDER BY server_id`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	nodes := make([]DeliveryNode, 0)
	for rows.Next() {
		var node DeliveryNode
		if err := scanDeliveryNode(rows, &node); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

func (s *Store) SetDeliveryNodeDesired(tenantID, projectID int64, serverID, version string, autoFollow *bool, now time.Time) (*DeliveryNode, error) {
	serverID, version = strings.TrimSpace(serverID), strings.TrimSpace(version)
	if !safeDeliveryServerID.MatchString(serverID) {
		return nil, fmt.Errorf("invalid server_id")
	}
	stamp := now.Format(timeLayout)
	if _, err := s.db.Exec(`INSERT INTO delivery_nodes(tenant_id,project_id,server_id,delivery_mode,created_at,updated_at) VALUES(?,?,?,'pull',?,?) ON CONFLICT(tenant_id,project_id,server_id,delivery_mode) DO NOTHING`, tenantID, projectID, serverID, stamp, stamp); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if version != "" {
		if _, err := tx.Exec(`UPDATE delivery_nodes SET desired_generation=desired_generation+CASE WHEN desired_version<>? THEN 1 ELSE 0 END,status=CASE WHEN desired_version<>? THEN 'pending' ELSE status END,desired_version=?,last_error='',updated_at=? WHERE tenant_id=? AND project_id=? AND server_id=? AND delivery_mode='pull'`, version, version, version, stamp, tenantID, projectID, serverID); err != nil {
			return nil, err
		}
	}
	if autoFollow != nil {
		if _, err := tx.Exec(`UPDATE delivery_nodes SET auto_follow=?,updated_at=? WHERE tenant_id=? AND project_id=? AND server_id=? AND delivery_mode='pull'`, *autoFollow, stamp, tenantID, projectID, serverID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetDeliveryNode(tenantID, projectID, serverID)
}

func (s *Store) AdvanceAutoFollowDeliveryNodes(tenantID, projectID int64, version string, now time.Time) (int64, error) {
	// Do not advance an already matching node. Besides reducing needless work,
	// this keeps a duplicate publish request from manufacturing false drift.
	result, err := s.db.Exec(`UPDATE delivery_nodes SET desired_generation=desired_generation+1,status='pending',desired_version=?,last_error='',updated_at=? WHERE tenant_id=? AND project_id=? AND delivery_mode='pull' AND auto_follow=1 AND desired_version<>?`, version, now.Format(timeLayout), tenantID, projectID, version)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return count, nil
}

func (s *Store) DeleteDeliveryNode(tenantID, projectID int64, serverID string) error {
	result, err := s.db.Exec(`DELETE FROM delivery_nodes WHERE tenant_id=? AND project_id=? AND server_id=? AND delivery_mode='pull'`, tenantID, projectID, serverID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) LatestSuccessfulPushVersion(projectID, bindingID int64) (string, error) {
	var version string
	err := s.db.QueryRow(`SELECT d.version FROM push_deployment_targets t JOIN push_deployments d ON d.id=t.deployment_id WHERE d.project_id=? AND t.binding_id=? AND t.status='success' ORDER BY t.id DESC LIMIT 1`, projectID, bindingID).Scan(&version)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return version, err
}
