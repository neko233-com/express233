package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultFileMode os.FileMode = 0o644
	defaultExecMode os.FileMode = 0o755
)

// archiveFileMode preserves the executable intent while stripping special and
// group/world-writable bits from untrusted archive metadata.
func archiveFileMode(mode os.FileMode) os.FileMode {
	if mode.Perm()&0o111 != 0 {
		return defaultExecMode
	}
	return defaultFileMode
}

func (s *Store) setVersionFileMode(tenantID int64, projectName, version, relPath string, mode os.FileMode) error {
	relPath = cleanRelPath(relPath)
	if relPath == "." || strings.HasPrefix(relPath, "../") || filepath.IsAbs(relPath) {
		return fmt.Errorf("invalid path")
	}
	_, err := s.db.Exec(
		`INSERT INTO version_file_modes(tenant_id,project_name,version,path,mode,updated_at)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(tenant_id,project_name,version,path)
		 DO UPDATE SET mode=excluded.mode,updated_at=excluded.updated_at`,
		tenantID, projectName, version, relPath, int64(archiveFileMode(mode)), time.Now().Format(timeLayout),
	)
	return err
}

// ListVersionFileModes returns explicit mode metadata. Files without a row use
// 0644, preserving compatibility with versions uploaded before this migration.
func (s *Store) ListVersionFileModes(tenantID int64, projectName, version string) (map[string]os.FileMode, error) {
	rows, err := s.db.Query(
		`SELECT path,mode FROM version_file_modes WHERE tenant_id=? AND project_name=? AND version=?`,
		tenantID, projectName, version,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]os.FileMode)
	for rows.Next() {
		var path string
		var mode int64
		if err := rows.Scan(&path, &mode); err != nil {
			return nil, err
		}
		out[filepath.ToSlash(path)] = archiveFileMode(os.FileMode(mode))
	}
	return out, rows.Err()
}

func (s *Store) deleteVersionFileMode(tenantID int64, projectName, version, relPath string) error {
	_, err := s.db.Exec(
		`DELETE FROM version_file_modes WHERE tenant_id=? AND project_name=? AND version=? AND path=?`,
		tenantID, projectName, version, cleanRelPath(relPath),
	)
	return err
}

func (s *Store) deleteVersionFileModes(tenantID int64, projectName, version string) error {
	_, err := s.db.Exec(
		`DELETE FROM version_file_modes WHERE tenant_id=? AND project_name=? AND version=?`,
		tenantID, projectName, version,
	)
	return err
}

func (s *Store) deleteProjectFileModes(tenantID int64, projectName string) error {
	_, err := s.db.Exec(
		`DELETE FROM version_file_modes WHERE tenant_id=? AND project_name=?`,
		tenantID, projectName,
	)
	return err
}
