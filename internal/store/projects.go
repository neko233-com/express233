package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Project 项目。
type Project struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	MyRole    string `json:"my_role,omitempty"`
}

// Version 版本。
type Version struct {
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"project_id"`
	ProjectName string `json:"project_name,omitempty"`
	Version     string `json:"version"`
	Status      string `json:"status"` // draft | published
	CreatedAt   string `json:"created_at"`
	PublishedAt string `json:"published_at,omitempty"`
}

// ListProjects 列出用户可见项目（成员或租户 admin）。
func (s *Store) ListProjects(tenantID, userID int64, tenantRole string) ([]Project, error) {
	var rows *sql.Rows
	var err error
	if tenantRole == RoleAdmin {
		rows, err = s.db.Query(`SELECT id, name, created_at, 'admin' FROM projects WHERE tenant_id = ? ORDER BY name`, tenantID)
	} else {
		rows, err = s.db.Query(`
SELECT p.id, p.name, p.created_at, m.role FROM projects p
INNER JOIN project_members m ON m.project_id = p.id AND m.user_id = ?
WHERE p.tenant_id = ?
ORDER BY p.name`, userID, tenantID)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.MyRole); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateProject 创建项目并将创建者设为项目管理员。
func (s *Store) CreateProject(tenantID, ownerUserID int64, name string) (*Project, error) {
	now := time.Now().Format(timeLayout)
	res, err := s.db.Exec(`INSERT INTO projects(tenant_id, name, created_at) VALUES(?,?,?)`, tenantID, name, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	root, err := s.ProjectsRoot(tenantID)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := s.AddProjectMember(id, ownerUserID, ProjectRoleAdmin); err != nil {
		return nil, err
	}
	return &Project{ID: id, Name: name, CreatedAt: now}, nil
}

// DeleteProject 删除项目及所有版本文件。
func (s *Store) DeleteProject(tenantID, id int64) error {
	var name string
	if err := s.db.QueryRow(`SELECT name FROM projects WHERE id = ? AND tenant_id = ?`, id, tenantID).Scan(&name); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM projects WHERE id = ? AND tenant_id = ?`, id, tenantID); err != nil {
		return err
	}
	root, err := s.ProjectsRoot(tenantID)
	if err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(root, name))
	return nil
}

// GetProjectByName 按名称查项目（租户内）；不校验成员，调用方需检查权限。
func (s *Store) GetProjectByName(tenantID int64, name string) (*Project, error) {
	var p Project
	err := s.db.QueryRow(`SELECT id, name, created_at FROM projects WHERE tenant_id = ? AND name = ?`, tenantID, name).Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ProjectNameInTenant 按 ID 查项目名（租户内）。
func (s *Store) ProjectNameInTenant(tenantID, projectID int64) (string, error) {
	var name string
	err := s.db.QueryRow(`SELECT name FROM projects WHERE id = ? AND tenant_id = ?`, projectID, tenantID).Scan(&name)
	return name, err
}

// ListVersions 列出项目下版本。
func (s *Store) ListVersions(projectID int64) ([]Version, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, version, status, created_at, COALESCE(published_at,'') FROM versions WHERE project_id = ? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Version, &v.Status, &v.CreatedAt, &v.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// CreateVersion 创建草稿版本。
func (s *Store) CreateVersion(tenantID, projectID int64, projectName, version string) (*Version, error) {
	if !s.ProjectBelongsToTenant(projectID, tenantID) {
		return nil, ErrWrongTenant
	}
	now := time.Now().Format(timeLayout)
	res, err := s.db.Exec(
		`INSERT INTO versions(project_id, version, status, created_at) VALUES(?,?,?,?)`,
		projectID, version, "draft", now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	dir, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Version{ID: id, ProjectID: projectID, Version: version, Status: "draft", CreatedAt: now}, nil
}

// GetVersion 获取版本元数据。
func (s *Store) GetVersion(projectID int64, version string) (*Version, error) {
	var v Version
	var pub sql.NullString
	err := s.db.QueryRow(
		`SELECT id, project_id, version, status, created_at, published_at FROM versions WHERE project_id = ? AND version = ?`,
		projectID, version,
	).Scan(&v.ID, &v.ProjectID, &v.Version, &v.Status, &v.CreatedAt, &pub)
	if err != nil {
		return nil, err
	}
	if pub.Valid {
		v.PublishedAt = pub.String
	}
	return &v, nil
}

// PublishVersion 发布版本（不可再改）。
func (s *Store) PublishVersion(tenantID, projectID int64, version string) error {
	if !s.ProjectBelongsToTenant(projectID, tenantID) {
		return ErrWrongTenant
	}
	v, err := s.GetVersion(projectID, version)
	if err != nil {
		return err
	}
	if v.Status == "published" {
		return fmt.Errorf("already published")
	}
	if v.Status != "draft" && v.Status != "pending_review" {
		return fmt.Errorf("version status %q cannot be published", v.Status)
	}
	var p Project
	if err := s.db.QueryRow(`SELECT id, name, created_at FROM projects WHERE id = ?`, projectID).Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
		return err
	}
	dir, err := s.VersionDir(tenantID, p.Name, version)
	if err != nil {
		return err
	}
	empty, err := isDirEmpty(dir)
	if err != nil {
		return err
	}
	if empty {
		return fmt.Errorf("version directory is empty; upload content before publish")
	}
	if err := ValidateUniqueConfigBasenames(dir); err != nil {
		return err
	}
	now := time.Now().Format(timeLayout)
	_, err = s.db.Exec(
		`UPDATE versions SET status = 'published', published_at = ? WHERE project_id = ? AND version = ?`,
		now, projectID, version,
	)
	return err
}

func isDirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

// DeleteVersion 删除版本（需已确认）。
func (s *Store) DeleteVersion(tenantID, projectID int64, projectName, version string) error {
	if !s.ProjectBelongsToTenant(projectID, tenantID) {
		return ErrWrongTenant
	}
	res, err := s.db.Exec(`DELETE FROM versions WHERE project_id = ? AND version = ?`, projectID, version)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	dir, err := s.VersionDir(tenantID, projectName, version)
	if err == nil {
		_ = os.RemoveAll(dir)
	}
	return nil
}

// ListPublishedVersions 返回项目下所有已发布版本（新→旧）。
func (s *Store) ListPublishedVersions(projectID int64) ([]Version, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, version, status, created_at, COALESCE(published_at,'') FROM versions
		 WHERE project_id = ? AND status = 'published' ORDER BY published_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Version, &v.Status, &v.CreatedAt, &v.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LatestPublishedVersion 最近发布的版本号。
func (s *Store) LatestPublishedVersion(projectID int64) (string, error) {
	var ver string
	err := s.db.QueryRow(
		`SELECT version FROM versions WHERE project_id = ? AND status = 'published' ORDER BY published_at DESC LIMIT 1`,
		projectID,
	).Scan(&ver)
	return ver, err
}

// ListVersionFiles 列出版本目录内相对路径。
func (s *Store) ListVersionFiles(tenantID int64, projectName, version string) ([]string, error) {
	root, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return nil, err
	}
	var files []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}
