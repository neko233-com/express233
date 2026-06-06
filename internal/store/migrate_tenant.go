package store

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) migrateTenant() error {
	_, _ = s.db.Exec(`
CREATE TABLE IF NOT EXISTS tenants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL
)`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN tenant_id INTEGER`)
	_, _ = s.db.Exec(`ALTER TABLE projects ADD COLUMN tenant_id INTEGER`)

	// 重建 projects 表以支持 (tenant_id, name) 唯一
	var hasNew int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='projects_v2'`).Scan(&hasNew)
	if hasNew == 0 {
		_, err := s.db.Exec(`
CREATE TABLE projects_v2 (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(tenant_id, name),
  FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
)`)
		if err != nil {
			return err
		}
		_, _ = s.db.Exec(`
INSERT INTO projects_v2(id, tenant_id, name, created_at)
SELECT id, COALESCE(tenant_id, 0), name, created_at FROM projects`)
		_, _ = s.db.Exec(`DROP TABLE projects`)
		_, _ = s.db.Exec(`ALTER TABLE projects_v2 RENAME TO projects`)
	}

	return s.EnsureDefaultTenant()
}

// ServerYAMLPath 租户级 server.yaml。
// 路径布局: {dataDir}/userdata/{slug}/server.yaml
func (s *Store) ServerYAMLPath(tenantID int64) (string, error) {
	t, err := s.TenantByID(tenantID)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.dataDir, "userdata", t.Slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "server.yaml"), nil
}

// VersionDir 租户项目版本目录。
// 路径布局: {dataDir}/userdata/{slug}/projects/{projectName}/{version}/
func (s *Store) VersionDir(tenantID int64, projectName, version string) (string, error) {
	t, err := s.TenantByID(tenantID)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.dataDir, "userdata", t.Slug, "projects", projectName, version), nil
}

// ProjectsRoot 租户项目根目录。
// 路径布局: {dataDir}/userdata/{slug}/projects/
func (s *Store) ProjectsRoot(tenantID int64) (string, error) {
	t, err := s.TenantByID(tenantID)
	if err != nil {
		return "", err
	}
	root := filepath.Join(s.dataDir, "userdata", t.Slug, "projects")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	return root, nil
}

// ProjectBelongsToTenant 校验项目归属。
func (s *Store) ProjectBelongsToTenant(projectID, tenantID int64) bool {
	var tid int64
	if err := s.db.QueryRow(`SELECT tenant_id FROM projects WHERE id = ?`, projectID).Scan(&tid); err != nil {
		return false
	}
	return tid == tenantID
}

// ErrWrongTenant 租户不匹配。
var ErrWrongTenant = fmt.Errorf("resource not in tenant scope")
