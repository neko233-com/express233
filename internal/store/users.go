package store

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 账号。
// RoleAdmin / RoleOperator / RoleViewer 用户角色。
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// User 账号。
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	IsAdmin   bool   `json:"is_admin"`
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}

// ListUsers 列出用户（不含密码）。
func (s *Store) ListUsers(tenantID int64) ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, COALESCE(role,'operator'), is_admin, token, created_at FROM users WHERE tenant_id = ? ORDER BY id`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var admin int
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &admin, &u.Token, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.IsAdmin = admin == 1
		out = append(out, u)
	}
	return out, rows.Err()
}

// CreateUser 创建账号。
func (s *Store) CreateUser(tenantID int64, username, password, role string, isAdmin bool) (*User, error) {
	if role == "" {
		if isAdmin {
			role = RoleAdmin
		} else {
			role = RoleOperator
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	admin := 0
	if isAdmin || role == RoleAdmin {
		admin = 1
		role = RoleAdmin
	}
	now := time.Now().Format(timeLayout)
	res, err := s.db.Exec(
		`INSERT INTO users(username, password_hash, is_admin, role, tenant_id, token, created_at) VALUES(?,?,?,?,?,?,?)`,
		username, string(hash), admin, role, tenantID, token, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, Role: role, IsAdmin: admin == 1, Token: token, CreatedAt: now}, nil
}

// DeleteUser 删除账号。
func (s *Store) DeleteUser(id int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RefreshUserToken 刷新 token。
func (s *Store) RefreshUserToken(id int64) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	res, err := s.db.Exec(`UPDATE users SET token = ? WHERE id = ?`, token, id)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", sql.ErrNoRows
	}
	return token, nil
}

// UpdateUserPassword 修改密码。
func (s *Store) UpdateUserPassword(id int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetUserByID 查询用户。
func (s *Store) GetUserByID(id int64) (*User, error) {
	var u User
	var admin int
	err := s.db.QueryRow(
		`SELECT id, username, COALESCE(role,'operator'), is_admin, token, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Role, &admin, &u.Token, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	return &u, nil
}

// EnsureAdmin 确保操作者为管理员。
func (s *Store) EnsureAdmin(userID int64) error {
	role, err := s.UserRole(userID)
	if err != nil {
		return err
	}
	if role != RoleAdmin {
		return fmt.Errorf("admin required")
	}
	return nil
}

// UserRole 查询用户角色。
func (s *Store) UserRole(userID int64) (string, error) {
	var role string
	var admin int
	err := s.db.QueryRow(`SELECT COALESCE(role,'operator'), is_admin FROM users WHERE id = ?`, userID).Scan(&role, &admin)
	if err != nil {
		return "", err
	}
	if admin == 1 {
		return RoleAdmin, nil
	}
	if role == "" {
		return RoleOperator, nil
	}
	return role, nil
}
