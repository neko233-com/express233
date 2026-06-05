package store

import (
	"database/sql"
	"fmt"
	"time"
)

// ProjectInvite 项目邀请。
type ProjectInvite struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	Token     string `json:"token"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

// ProjectInviteInfo 接受邀请前展示的公开信息。
type ProjectInviteInfo struct {
	ProjectName string `json:"project_name"`
	Role        string `json:"role"`
	ExpiresAt   string `json:"expires_at"`
	Expired     bool   `json:"expired"`
}

// CreateProjectInvite 创建邀请链接 token。
func (s *Store) CreateProjectInvite(projectID, createdBy int64, role string, validHours int) (*ProjectInvite, error) {
	if role != ProjectRoleAdmin && role != ProjectRoleViewer {
		return nil, fmt.Errorf("invalid role")
	}
	if validHours <= 0 {
		validHours = 168
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	exp := now.Add(time.Duration(validHours) * time.Hour)
	created := now.Format(timeLayout)
	res, err := s.db.Exec(
		`INSERT INTO project_invites(project_id, token, role, created_by, expires_at, created_at) VALUES(?,?,?,?,?,?)`,
		projectID, token, role, createdBy, exp.Format(timeLayout), created,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &ProjectInvite{
		ID:        id,
		ProjectID: projectID,
		Token:     token,
		Role:      role,
		ExpiresAt: exp.Format(timeLayout),
		CreatedAt: created,
	}, nil
}

// GetProjectInviteByToken 查询邀请。
func (s *Store) GetProjectInviteByToken(token string) (*ProjectInvite, error) {
	var inv ProjectInvite
	err := s.db.QueryRow(
		`SELECT id, project_id, token, role, expires_at, created_at FROM project_invites WHERE token = ?`,
		token,
	).Scan(&inv.ID, &inv.ProjectID, &inv.Token, &inv.Role, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// ProjectInvitePreview 邀请预览（含项目名）。
func (s *Store) ProjectInvitePreview(token string) (*ProjectInviteInfo, error) {
	inv, err := s.GetProjectInviteByToken(token)
	if err != nil {
		return nil, err
	}
	var pname string
	if err := s.db.QueryRow(`SELECT name FROM projects WHERE id = ?`, inv.ProjectID).Scan(&pname); err != nil {
		return nil, err
	}
	exp, _ := time.Parse(timeLayout, inv.ExpiresAt)
	return &ProjectInviteInfo{
		ProjectName: pname,
		Role:        inv.Role,
		ExpiresAt:   inv.ExpiresAt,
		Expired:     time.Now().After(exp),
	}, nil
}

// AcceptProjectInvite 登录用户接受邀请。
func (s *Store) AcceptProjectInvite(token string, userID int64) (*Project, error) {
	inv, err := s.GetProjectInviteByToken(token)
	if err != nil {
		return nil, err
	}
	exp, err := time.Parse(timeLayout, inv.ExpiresAt)
	if err != nil || time.Now().After(exp) {
		return nil, fmt.Errorf("invite expired")
	}
	var tenantID int64
	var pname string
	if err := s.db.QueryRow(`SELECT tenant_id, name FROM projects WHERE id = ?`, inv.ProjectID).Scan(&tenantID, &pname); err != nil {
		return nil, err
	}
	var userTenant int64
	if err := s.db.QueryRow(`SELECT tenant_id FROM users WHERE id = ?`, userID).Scan(&userTenant); err != nil {
		return nil, err
	}
	if userTenant != tenantID {
		return nil, fmt.Errorf("invite belongs to another tenant")
	}
	if err := s.AddProjectMember(inv.ProjectID, userID, inv.Role); err != nil {
		return nil, err
	}
	return &Project{ID: inv.ProjectID, Name: pname}, nil
}

// DeleteProjectInvite 删除邀请（可选，接受后也可保留链接复用 — 设计为可复用直到过期）。
func (s *Store) DeleteProjectInvite(projectID int64, inviteID int64) error {
	res, err := s.db.Exec(`DELETE FROM project_invites WHERE id = ? AND project_id = ?`, inviteID, projectID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListProjectInvites 列出项目未过期邀请。
func (s *Store) ListProjectInvites(projectID int64) ([]ProjectInvite, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, token, role, expires_at, created_at FROM project_invites
		 WHERE project_id = ? AND expires_at > ? ORDER BY created_at DESC`,
		projectID, time.Now().Format(timeLayout),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ProjectInvite
	for rows.Next() {
		var inv ProjectInvite
		if err := rows.Scan(&inv.ID, &inv.ProjectID, &inv.Token, &inv.Role, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}
