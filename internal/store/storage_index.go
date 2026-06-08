package store

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Store) migrateStorageIndex() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS storage_index (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL DEFAULT 0,
  rel_path TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  size_bytes INTEGER NOT NULL DEFAULT 0,
  project_name TEXT,
  version TEXT,
  file_rel TEXT,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_storage_index_name ON storage_index(name);
CREATE INDEX IF NOT EXISTS idx_storage_index_path ON storage_index(rel_path);
CREATE INDEX IF NOT EXISTS idx_storage_index_tenant ON storage_index(tenant_id);
`)
	return err
}

// StorageIndexMeta 索引元信息。
type StorageIndexMeta struct {
	EntryCount int    `json:"entry_count"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// StorageSearchHit 本地文件搜索命中。
type StorageSearchHit struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	SizeBytes   int64  `json:"size_bytes"`
	ProjectName string `json:"project_name,omitempty"`
	Version     string `json:"version,omitempty"`
	FileRel     string `json:"file_rel,omitempty"`
}

func (s *Store) StorageIndexMeta() (StorageIndexMeta, error) {
	var m StorageIndexMeta
	if err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(updated_at),'') FROM storage_index`).Scan(&m.EntryCount, &m.UpdatedAt); err != nil {
		return m, err
	}
	return m, nil
}

// RebuildStorageIndex 全量重建租户可见的本地存储索引。
func (s *Store) RebuildStorageIndex(tenantID int64) (int, error) {
	if _, err := s.db.Exec(`DELETE FROM storage_index WHERE tenant_id = ?`, tenantID); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`DELETE FROM storage_index WHERE tenant_id = 0 AND kind LIKE 'blob%'`); err != nil {
		return 0, err
	}

	now := time.Now().Format(timeLayout)
	count, err := s.indexTenantTree(tenantID, now)
	if err != nil {
		return 0, err
	}
	blobCount, err := s.indexBlobs(now)
	if err != nil {
		return 0, err
	}
	return count + blobCount, nil
}

func (s *Store) indexTenantTree(tenantID int64, now string) (int, error) {
	t, err := s.TenantByID(tenantID)
	if err != nil {
		return 0, err
	}
	userRoot := filepath.Join(s.dataDir, "userdata", t.Slug)
	count := 0
	insert := func(relPath, name, kind string, size int64, project, version, fileRel string) error {
		_, err := s.db.Exec(
			`INSERT INTO storage_index(tenant_id, rel_path, name, kind, size_bytes, project_name, version, file_rel, updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
			tenantID, relPath, name, kind, size, nullIfEmpty(project), nullIfEmpty(version), nullIfEmpty(fileRel), now,
		)
		if err == nil {
			count++
		}
		return err
	}

	relUser := filepath.ToSlash(filepath.Join("userdata", t.Slug))
	if err := insert(relUser, t.Slug, "tenant", dirSize(userRoot), "", "", ""); err != nil {
		return 0, err
	}

	projectsRoot := filepath.Join(userRoot, "projects")
	relProjects := relUser + "/projects"
	if st, err := os.Stat(projectsRoot); err == nil && st.IsDir() {
		if err := insert(relProjects, "projects", "folder", dirSize(projectsRoot), "", "", ""); err != nil {
			return 0, err
		}
	}

	projects, err := s.ListProjects(tenantID, 0, RoleAdmin)
	if err != nil {
		return 0, err
	}
	for _, p := range projects {
		projDir := filepath.Join(projectsRoot, p.Name)
		relProj := relProjects + "/" + p.Name
		if err := insert(relProj, p.Name, "project", dirSize(projDir), p.Name, "", ""); err != nil {
			return 0, err
		}
		versions, err := s.ListVersions(p.ID)
		if err != nil {
			return 0, err
		}
		for _, v := range versions {
			verDir := filepath.Join(projDir, v.Version)
			relVer := relProj + "/" + v.Version
			if err := insert(relVer, v.Version, "version", dirSize(verDir), p.Name, v.Version, ""); err != nil {
				return 0, err
			}
			walkErr := filepath.Walk(verDir, func(path string, info os.FileInfo, walkErr error) error {
				if walkErr != nil || info.IsDir() {
					return walkErr
				}
				rel, err := filepath.Rel(s.dataDir, path)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				fileRel, err := filepath.Rel(verDir, path)
				if err != nil {
					return err
				}
				return insert(rel, info.Name(), "file", info.Size(), p.Name, v.Version, filepath.ToSlash(fileRel))
			})
			if walkErr != nil {
				return 0, walkErr
			}
		}
	}
	return count, nil
}

func (s *Store) indexBlobs(now string) (int, error) {
	count := 0
	blobsRoot := s.blobsRoot()
	relRoot := "blobs"
	if st, err := os.Stat(blobsRoot); err == nil && st.IsDir() {
		_, err := s.db.Exec(
			`INSERT INTO storage_index(tenant_id, rel_path, name, kind, size_bytes, updated_at) VALUES(0,?,?,?,?,?)`,
			relRoot, "blobs", "folder", dirSize(blobsRoot), now,
		)
		if err != nil {
			return 0, err
		}
		count++
	}
	rows, err := s.db.Query(`SELECT hash, size, ref_count FROM blobs ORDER BY hash`)
	if err != nil {
		return 0, err
	}
	type blobRow struct {
		hash string
		size int64
		refs int
	}
	var blobRows []blobRow
	for rows.Next() {
		var r blobRow
		if err := rows.Scan(&r.hash, &r.size, &r.refs); err != nil {
			_ = rows.Close()
			return 0, err
		}
		blobRows = append(blobRows, r)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	for _, r := range blobRows {
		relFromRoot, _ := filepath.Rel(s.dataDir, s.blobPath(r.hash))
		relFromRoot = filepath.ToSlash(relFromRoot)
		kind := "blob"
		if r.refs == 0 {
			kind = "orphan_blob"
		}
		_, err := s.db.Exec(
			`INSERT INTO storage_index(tenant_id, rel_path, name, kind, size_bytes, updated_at) VALUES(0,?,?,?,?,?)`,
			relFromRoot, r.hash[:min(12, len(r.hash))]+"…", kind, r.size, now,
		)
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// SearchStorageIndex 在索引中搜索路径/文件名。
func (s *Store) SearchStorageIndex(tenantID int64, query string, limit int) ([]StorageSearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	like := "%" + escapeLike(query) + "%"
	rows, err := s.db.Query(`
SELECT rel_path, name, kind, size_bytes, COALESCE(project_name,''), COALESCE(version,''), COALESCE(file_rel,'')
FROM storage_index
WHERE (tenant_id = ? OR (tenant_id = 0 AND kind LIKE 'blob%'))
  AND (name LIKE ? ESCAPE '\' OR rel_path LIKE ? ESCAPE '\' OR COALESCE(file_rel,'') LIKE ? ESCAPE '\')
ORDER BY kind, rel_path
LIMIT ?`, tenantID, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []StorageSearchHit
	for rows.Next() {
		var h StorageSearchHit
		if err := rows.Scan(&h.Path, &h.Name, &h.Kind, &h.SizeBytes, &h.ProjectName, &h.Version, &h.FileRel); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
