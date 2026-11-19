package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) migrateBlobs() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS blobs (
  hash TEXT PRIMARY KEY,
  size INTEGER NOT NULL,
  ref_count INTEGER NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS store_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS version_file_tags (
  tenant_id INTEGER NOT NULL,
  project_name TEXT NOT NULL,
  version TEXT NOT NULL,
  path TEXT NOT NULL,
  tags TEXT NOT NULL DEFAULT '*',
  updated_at TEXT NOT NULL,
  PRIMARY KEY(tenant_id, project_name, version, path)
);
CREATE TABLE IF NOT EXISTS version_file_modes (
  tenant_id INTEGER NOT NULL,
  project_name TEXT NOT NULL,
  version TEXT NOT NULL,
  path TEXT NOT NULL,
  mode INTEGER NOT NULL DEFAULT 420,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(tenant_id, project_name, version, path)
)`)
	if err != nil {
		return err
	}
	if err := s.checkBlobStorage(); err != nil {
		return err
	}
	if local, ok := s.blobs.(localBlobStorage); ok {
		if err := os.MkdirAll(local.root, 0o755); err != nil {
			return err
		}
	}
	return s.migrateExistingFilesToBlobs()
}

func (s *Store) checkBlobStorage() error {
	var name string
	err := s.db.QueryRow(`SELECT value FROM store_meta WHERE key = 'blob_storage'`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		name = s.BlobStorageName()
		var count int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			name = "local"
		}
	} else if err != nil {
		return err
	}
	if name != s.BlobStorageName() {
		return fmt.Errorf("artifact storage mismatch: data uses %q, configured %q", name, s.BlobStorageName())
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO store_meta(key, value) VALUES('blob_storage', ?)`, name)
	return err
}

func (s *Store) migrateExistingFilesToBlobs() error {
	var done string
	err := s.db.QueryRow(`SELECT value FROM store_meta WHERE key = 'blobs_migrated'`).Scan(&done)
	if err == nil && done == "1" {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	userdata := filepath.Join(s.dataDir, "userdata")
	if _, err := os.Stat(userdata); os.IsNotExist(err) {
		return s.markBlobsMigrated()
	}

	err = filepath.Walk(userdata, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !isVersionDataFile(userdata, path) {
			return nil
		}
		return s.adoptLegacyArtifact(path, info)
	})
	if err != nil {
		return err
	}
	return s.markBlobsMigrated()
}

func (s *Store) adoptLegacyArtifact(path string, info os.FileInfo) error {
	hash, err := hashFile(path)
	if err != nil {
		return err
	}
	if local, ok := s.blobs.(localBlobStorage); ok {
		if original, err := os.Stat(local.path(hash)); err == nil && os.SameFile(original, info) {
			return nil
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	putErr := s.putBlob(hash, info.Size(), f)
	if err := errors.Join(putErr, f.Close()); err != nil {
		return err
	}
	if err := s.materializeBlob(hash, path); err != nil {
		return errors.Join(err, s.decrementBlobRef(hash))
	}
	return nil
}

func (s *Store) markBlobsMigrated() error {
	_, err := s.db.Exec(
		`INSERT INTO store_meta(key, value) VALUES('blobs_migrated', '1')
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
	)
	return err
}
