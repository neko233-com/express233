package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Tenant 租户。
type Tenant struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

const defaultTenantSlug = "default"

// EnsureDefaultTenant 创建默认租户并迁移旧数据布局。
func (s *Store) EnsureDefaultTenant() error {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM tenants WHERE slug = ?`, defaultTenantSlug).Scan(&id)
	if err == sql.ErrNoRows {
		now := time.Now().Format(timeLayout)
		res, err := s.db.Exec(
			`INSERT INTO tenants(slug, name, created_at) VALUES(?,?,?)`,
			defaultTenantSlug, "Default", now,
		)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		return err
	}
	if err := s.migrateLegacyDataLayout(id, defaultTenantSlug); err != nil {
		return err
	}
	_, _ = s.db.Exec(`UPDATE users SET tenant_id = ? WHERE tenant_id IS NULL OR tenant_id = 0`, id)
	_, _ = s.db.Exec(`UPDATE projects SET tenant_id = ? WHERE tenant_id IS NULL OR tenant_id = 0`, id)
	return nil
}

func (s *Store) migrateLegacyDataLayout(tenantID int64, slug string) error {
	tenantRoot := filepath.Join(s.dataDir, "userdata", slug)
	_ = os.MkdirAll(tenantRoot, 0o755)
	newProjects := filepath.Join(tenantRoot, "projects")

	// 1. 最早期布局: {dataDir}/projects/ → {dataDir}/userdata/{slug}/projects/
	oldProjects := filepath.Join(s.dataDir, "projects")
	if _, err := os.Stat(oldProjects); err == nil {
		if _, err := os.Stat(newProjects); os.IsNotExist(err) {
			if err := os.Rename(oldProjects, newProjects); err != nil {
				return fmt.Errorf("migrate projects dir: %w", err)
			}
		}
	}

	// 2. 上一版布局: {dataDir}/tenants/{slug}/projects/{name}/ → userdata/{slug}/projects/{name}/
	oldTenantProjects := filepath.Join(s.dataDir, "tenants", slug, "projects")
	if _, err := os.Stat(oldTenantProjects); err == nil {
		if _, err := os.Stat(newProjects); os.IsNotExist(err) {
			if err := os.Rename(oldTenantProjects, newProjects); err != nil {
				return fmt.Errorf("migrate tenant projects dir: %w", err)
			}
		} else {
			// newProjects 已存在，逐项迁移
			entries, _ := os.ReadDir(oldTenantProjects)
			for _, e := range entries {
				dst := filepath.Join(newProjects, e.Name())
				if _, err := os.Stat(dst); os.IsNotExist(err) {
					if err := os.Rename(filepath.Join(oldTenantProjects, e.Name()), dst); err != nil {
						return fmt.Errorf("migrate tenant project %s: %w", e.Name(), err)
					}
				}
			}
		}
	}

	// 3. server.yaml: 两个旧位置都要检查
	for _, oldPath := range []string{
		filepath.Join(s.dataDir, "server.yaml"),
		filepath.Join(s.dataDir, "tenants", slug, "server.yaml"),
	} {
		newYAML := filepath.Join(tenantRoot, "server.yaml")
		if _, err := os.Stat(oldPath); err == nil {
			if _, err := os.Stat(newYAML); os.IsNotExist(err) {
				if err := os.Rename(oldPath, newYAML); err != nil {
					return fmt.Errorf("migrate server.yaml: %w", err)
				}
			}
		}
	}

	_ = tenantID
	return nil
}

// ListTenants 列出所有租户（系统管理）。
func (s *Store) ListTenants() ([]Tenant, error) {
	rows, err := s.db.Query(`SELECT id, slug, name, created_at FROM tenants ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CreateTenant 创建租户。
func (s *Store) CreateTenant(slug, name string) (*Tenant, error) {
	if slug == "" || name == "" {
		return nil, fmt.Errorf("slug and name required")
	}
	now := time.Now().Format(timeLayout)
	res, err := s.db.Exec(
		`INSERT INTO tenants(slug, name, created_at) VALUES(?,?,?)`,
		slug, name, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	root := filepath.Join(s.dataDir, "userdata", slug)
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		return nil, err
	}
	return &Tenant{ID: id, Slug: slug, Name: name, CreatedAt: now}, nil
}

// TenantByID 查询租户。
func (s *Store) TenantByID(id int64) (*Tenant, error) {
	var t Tenant
	err := s.db.QueryRow(
		`SELECT id, slug, name, created_at FROM tenants WHERE id = ?`, id,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UserTenantID 返回用户所属租户。
func (s *Store) UserTenantID(userID int64) (int64, error) {
	var tid int64
	err := s.db.QueryRow(`SELECT tenant_id FROM users WHERE id = ?`, userID).Scan(&tid)
	return tid, err
}

// TokenTenantID 根据 pull token 解析租户。
func (s *Store) TokenTenantID(token string) (int64, error) {
	var tid int64
	err := s.db.QueryRow(`SELECT tenant_id FROM users WHERE token = ?`, token).Scan(&tid)
	return tid, err
}
