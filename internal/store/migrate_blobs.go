package store

import (
	"database/sql"
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
)`)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.blobsRoot(), 0o755); err != nil {
		return err
	}
	return s.migrateExistingFilesToBlobs()
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
		return s.adoptFileToBlob(path)
	})
	if err != nil {
		return err
	}
	return s.markBlobsMigrated()
}

func (s *Store) markBlobsMigrated() error {
	_, err := s.db.Exec(
		`INSERT INTO store_meta(key, value) VALUES('blobs_migrated', '1')
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
	)
	return err
}
