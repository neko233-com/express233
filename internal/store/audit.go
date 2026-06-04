package store

import (
	"database/sql"
	"time"
)

// AuditEntry 操作审计记录。
type AuditEntry struct {
	ID       int64  `json:"id"`
	At       string `json:"at"`
	Username string `json:"username"`
	Action   string `json:"action"`
	Detail   string `json:"detail"`
	IP       string `json:"ip,omitempty"`
}

func (s *Store) migrateAudit() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  at TEXT NOT NULL,
  username TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_at ON audit_logs(at DESC);
`)
	return err
}

// RecordAudit 写入审计日志。
func (s *Store) RecordAudit(username, action, detail, ip string) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_logs(at, username, action, detail, ip) VALUES(?,?,?,?,?)`,
		time.Now().Format(timeLayout), username, action, detail, ip,
	)
	return err
}

// ListAuditLogs 最近审计记录（最多 limit 条）。
func (s *Store) ListAuditLogs(limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, at, username, action, detail, ip FROM audit_logs ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.At, &e.Username, &e.Action, &e.Detail, &e.IP); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UsernameForToken 根据拉取 token 查用户名（审计用）。
func (s *Store) UsernameForToken(token string) string {
	var u string
	err := s.db.QueryRow(`SELECT username FROM users WHERE token = ?`, token).Scan(&u)
	if err == sql.ErrNoRows {
		return "token:*"
	}
	if err != nil {
		return ""
	}
	return u
}
