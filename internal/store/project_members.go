package store

import (
	"database/sql"
	"fmt"
	"time"
)

// 项目内角色（与租户角色独立）。
const (
	ProjectRoleAdmin  = "admin"  // 读写
	ProjectRoleViewer = "viewer" // 只读
)

// ProjectMember 项目成员。
type ProjectMember struct {
	ProjectID int64  `json:"project_id"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username,omitempty"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
}

func (s *Store) migrateProjectMembers() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS project_members (
  project_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  role TEXT NOT NULL,
  joined_at TEXT NOT NULL,
  PRIMARY KEY(project_id, user_id),
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS project_invites (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL,
  token TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL,
  created_by INTEGER NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
)`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`
INSERT OR IGNORE INTO project_members(project_id, user_id, role, joined_at)
SELECT p.id, u.id, 'admin', p.created_at
FROM projects p
JOIN users u ON u.tenant_id = p.tenant_id
WHERE u.is_admin = 1
  AND NOT EXISTS (SELECT 1 FROM project_members m WHERE m.project_id = p.id)`)
	_, _ = s.db.Exec(`
INSERT OR IGNORE INTO project_members(project_id, user_id, role, joined_at)
SELECT p.id,
  (SELECT id FROM users WHERE tenant_id = p.tenant_id ORDER BY id LIMIT 1),
  'admin', p.created_at
FROM projects p
WHERE NOT EXISTS (SELECT 1 FROM project_members m WHERE m.project_id = p.id)`)
	return nil
}

// AddProjectMember 添加成员。
func (s *Store) AddProjectMember(projectID, userID int64, role string) error {
	if role != ProjectRoleAdmin && role != ProjectRoleViewer {
		return fmt.Errorf("invalid project role %q", role)
	}
	now := time.Now().Format(timeLayout)
	_, err := s.db.Exec(
		`INSERT INTO project_members(project_id, user_id, role, joined_at) VALUES(?,?,?,?)
		 ON CONFLICT(project_id, user_id) DO UPDATE SET role = excluded.role`,
		projectID, userID, role, now,
	)
	return err
}

// ProjectMemberRole 查询用户在项目中的角色；无成员记录返回 "", nil。
func (s *Store) ProjectMemberRole(projectID, userID int64) (string, error) {
	var role string
	err := s.db.QueryRow(
		`SELECT role FROM project_members WHERE project_id = ? AND user_id = ?`,
		projectID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

// ProjectAccess 解析用户对项目的有效角色（租户 admin 视为项目 admin）。
func (s *Store) ProjectAccess(projectID, userID int64, tenantRole string) (string, error) {
	if tenantRole == RoleAdmin {
		return ProjectRoleAdmin, nil
	}
	role, err := s.ProjectMemberRole(projectID, userID)
	if err != nil {
		return "", err
	}
	return role, nil
}

// CanWriteProject 是否可对项目做写操作。
func CanWriteProject(projectRole string) bool {
	return projectRole == ProjectRoleAdmin
}

// ListProjectMembers 列出成员。
func (s *Store) ListProjectMembers(projectID int64) ([]ProjectMember, error) {
	rows, err := s.db.Query(`
SELECT m.project_id, m.user_id, u.username, m.role, m.joined_at
FROM project_members m
JOIN users u ON u.id = m.user_id
WHERE m.project_id = ?
ORDER BY m.joined_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectMember
	for rows.Next() {
		var m ProjectMember
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RemoveProjectMember 移除成员（不能移除最后一个 admin）。
func (s *Store) RemoveProjectMember(projectID, userID int64) error {
	role, err := s.ProjectMemberRole(projectID, userID)
	if err != nil {
		return err
	}
	if role == "" {
		return sql.ErrNoRows
	}
	if role == ProjectRoleAdmin {
		var n int
		_ = s.db.QueryRow(
			`SELECT COUNT(*) FROM project_members WHERE project_id = ? AND role = ?`,
			projectID, ProjectRoleAdmin,
		).Scan(&n)
		if n <= 1 {
			return fmt.Errorf("cannot remove the last project admin")
		}
	}
	_, err = s.db.Exec(`DELETE FROM project_members WHERE project_id = ? AND user_id = ?`, projectID, userID)
	return err
}
