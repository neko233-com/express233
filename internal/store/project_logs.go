package store

import (
	"database/sql"
	"time"
)

const projectLogRetention = 30 * 24 * time.Hour

type ProjectLog struct {
	ID        int64  `json:"id"`
	At        string `json:"at"`
	TenantID  int64  `json:"tenant_id,omitempty"`
	ProjectID int64  `json:"project_id"`
	Project   string `json:"project,omitempty"`
	Username  string `json:"username,omitempty"`
	Action    string `json:"action"`
	ServerID  string `json:"server_id,omitempty"`
	Version   string `json:"version,omitempty"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Tags      string `json:"tags,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	IP        string `json:"ip,omitempty"`
}

type ProjectLogFilter struct {
	TenantID  int64
	ProjectID int64
	ServerID  string
	Version   string
	Limit     int
}

func (s *Store) migrateProjectLogs() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS project_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  at TEXT NOT NULL,
  tenant_id INTEGER NOT NULL DEFAULT 1,
  project_id INTEGER NOT NULL,
  username TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  server_id TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL DEFAULT '',
  os TEXT NOT NULL DEFAULT '',
  arch TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_project_logs_project_at ON project_logs(project_id, at DESC);
CREATE INDEX IF NOT EXISTS idx_project_logs_server_version ON project_logs(project_id, server_id, version, at DESC);
`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`DELETE FROM project_logs WHERE at < ?`, time.Now().Add(-projectLogRetention).Format(timeLayout))
	return nil
}

func (s *Store) RecordProjectLog(log ProjectLog) error {
	if log.At == "" {
		log.At = time.Now().Format(timeLayout)
	}
	if log.Status == "" {
		log.Status = "ok"
	}
	if _, err := s.db.Exec(`DELETE FROM project_logs WHERE at < ?`, time.Now().Add(-projectLogRetention).Format(timeLayout)); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO project_logs(at, tenant_id, project_id, username, action, server_id, version, os, arch, tags, status, error, ip)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		log.At, log.TenantID, log.ProjectID, log.Username, log.Action, log.ServerID, log.Version, log.OS, log.Arch, log.Tags, log.Status, log.Error, log.IP,
	)
	return err
}

func (s *Store) ListProjectLogs(filter ProjectLogFilter) ([]ProjectLog, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT l.id, l.at, l.tenant_id, l.project_id, p.name, l.username, l.action, l.server_id, l.version, l.os, l.arch, l.tags, l.status, l.error, l.ip
FROM project_logs l
JOIN projects p ON p.id = l.project_id
WHERE l.project_id = ? AND p.tenant_id = ?`
	args := []any{filter.ProjectID, filter.TenantID}
	if filter.ServerID != "" {
		query += ` AND l.server_id = ?`
		args = append(args, filter.ServerID)
	}
	if filter.Version != "" {
		query += ` AND l.version = ?`
		args = append(args, filter.Version)
	}
	query += ` ORDER BY l.id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ProjectLog
	for rows.Next() {
		var log ProjectLog
		if err := rows.Scan(&log.ID, &log.At, &log.TenantID, &log.ProjectID, &log.Project, &log.Username, &log.Action, &log.ServerID, &log.Version, &log.OS, &log.Arch, &log.Tags, &log.Status, &log.Error, &log.IP); err != nil {
			return nil, err
		}
		out = append(out, log)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return out, nil
}
