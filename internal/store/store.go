package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// Store SQLite 持久化与文件布局。
type Store struct {
	db      *sql.DB
	dataDir string
}

// Open 打开或创建数据目录。
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "express233.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, dataDir: dataDir}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.ensureDefaultAdmin(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  is_admin INTEGER NOT NULL DEFAULT 0,
  token TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS versions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL,
  version TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TEXT NOT NULL,
  published_at TEXT,
  UNIQUE(project_id, version),
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}
	if err := s.migrateAudit(); err != nil {
		return err
	}
	if err := s.migrateExtra(); err != nil {
		return err
	}
	if err := s.migrateTenant(); err != nil {
		return err
	}
	return s.migrateProjectMembers()
}

func (s *Store) ensureDefaultAdmin() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var tenantID int64
	if err := s.db.QueryRow(`SELECT id FROM tenants WHERE slug = ?`, defaultTenantSlug).Scan(&tenantID); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("root"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	token, err := newToken()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO users(username, password_hash, is_admin, role, tenant_id, token, created_at) VALUES(?,?,?,?,?,?,?)`,
		"root", string(hash), 1, RoleAdmin, tenantID, token, time.Now().Format(timeLayout),
	)
	return err
}

const timeLayout = "2006-01-02 15:04:05"

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Close 关闭数据库。
func (s *Store) Close() error {
	return s.db.Close()
}

// DataDir 数据根目录。
func (s *Store) DataDir() string { return s.dataDir }

// Authenticate 校验用户名密码，返回 user id 与是否管理员。
func (s *Store) Authenticate(username, password string) (userID int64, isAdmin bool, err error) {
	var hash string
	err = s.db.QueryRow(
		`SELECT id, password_hash, is_admin FROM users WHERE username = ?`, username,
	).Scan(&userID, &hash, &isAdmin)
	if err == sql.ErrNoRows {
		return 0, false, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return 0, false, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return 0, false, fmt.Errorf("invalid credentials")
	}
	return userID, isAdmin, nil
}

// ValidatePullToken 校验拉取 token。
func (s *Store) ValidatePullToken(token string) (bool, error) {
	_, _, err := s.LookupPullToken(token)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// LookupPullToken 解析 token 对应用户与租户。
func (s *Store) LookupPullToken(token string) (userID, tenantID int64, err error) {
	err = s.db.QueryRow(`SELECT id, tenant_id FROM users WHERE token = ?`, token).Scan(&userID, &tenantID)
	return
}
