package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// blobs 表按内容 SHA-256 去重；版本目录内文件通过硬链指向 blob，ref_count 归零时删除实体。
func (s *Store) blobsRoot() string {
	return filepath.Join(s.dataDir, "blobs")
}

func (s *Store) blobPath(hash string) string {
	if len(hash) < 2 {
		return filepath.Join(s.blobsRoot(), hash)
	}
	return filepath.Join(s.blobsRoot(), hash[:2], hash)
}

func (s *Store) ingestBlob(data []byte) (hash string, err error) {
	sum := sha256.Sum256(data)
	hash = hex.EncodeToString(sum[:])
	blobPath := s.blobPath(hash)
	now := time.Now().Format(timeLayout)

	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	var ref int
	err = tx.QueryRow(`SELECT ref_count FROM blobs WHERE hash = ?`, hash).Scan(&ref)
	switch {
	case err == sql.ErrNoRows:
		if err := s.writeBlobFile(blobPath, data); err != nil {
			return "", err
		}
		if _, err := tx.Exec(
			`INSERT INTO blobs(hash, size, ref_count, created_at) VALUES(?,?,1,?)`,
			hash, len(data), now,
		); err != nil {
			_ = os.Remove(blobPath)
			return "", err
		}
	case err != nil:
		return "", err
	default:
		if _, err := tx.Exec(`UPDATE blobs SET ref_count = ref_count + 1 WHERE hash = ?`, hash); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return hash, nil
}

func (s *Store) ingestBlobFromReader(r io.Reader) (hash string, err error) {
	h := sha256.New()
	data, err := io.ReadAll(io.TeeReader(r, h))
	if err != nil {
		return "", err
	}
	hash = hex.EncodeToString(h.Sum(nil))
	blobPath := s.blobPath(hash)
	now := time.Now().Format(timeLayout)

	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	var ref int
	err = tx.QueryRow(`SELECT ref_count FROM blobs WHERE hash = ?`, hash).Scan(&ref)
	switch {
	case err == sql.ErrNoRows:
		if err := s.writeBlobFile(blobPath, data); err != nil {
			return "", err
		}
		if _, err := tx.Exec(
			`INSERT INTO blobs(hash, size, ref_count, created_at) VALUES(?,?,1,?)`,
			hash, len(data), now,
		); err != nil {
			_ = os.Remove(blobPath)
			return "", err
		}
	case err != nil:
		return "", err
	default:
		if _, err := tx.Exec(`UPDATE blobs SET ref_count = ref_count + 1 WHERE hash = ?`, hash); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return hash, nil
}

func (s *Store) writeBlobFile(blobPath string, data []byte) error {
	if _, err := os.Stat(blobPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(blobPath), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(blobPath), ".blob-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	cleanup = false
	if err := os.Rename(tmpPath, blobPath); err != nil {
		if _, statErr := os.Stat(blobPath); statErr == nil {
			return nil
		}
		return err
	}
	return nil
}

func (s *Store) linkBlobToVersion(hash, versionPath string) error {
	blobPath := s.blobPath(hash)
	if err := os.Remove(versionPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Link(blobPath, versionPath); err == nil {
		return nil
	}
	return copyBlobFile(blobPath, versionPath)
}

func (s *Store) releaseBlobLink(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	hash, err := s.hashFile(path)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return s.decrementBlobRef(hash)
}

func (s *Store) releaseVersionDir(root string) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return s.releaseBlobLink(path)
	})
}

func (s *Store) releaseProjectDir(root string) error {
	return s.releaseVersionDir(root)
}

func (s *Store) decrementBlobRef(hash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var ref int
	err = tx.QueryRow(`SELECT ref_count FROM blobs WHERE hash = ?`, hash).Scan(&ref)
	if err == sql.ErrNoRows {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	ref--
	if ref <= 0 {
		if _, err := tx.Exec(`DELETE FROM blobs WHERE hash = ?`, hash); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		_ = os.Remove(s.blobPath(hash))
		return nil
	}
	if _, err := tx.Exec(`UPDATE blobs SET ref_count = ? WHERE hash = ?`, ref, hash); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Store) isLinkedToBlob(path string) (bool, error) {
	hash, err := s.hashFile(path)
	if err != nil {
		return false, err
	}
	bp := s.blobPath(hash)
	st1, err1 := os.Stat(path)
	st2, err2 := os.Stat(bp)
	if err1 != nil || err2 != nil {
		return false, nil
	}
	return os.SameFile(st1, st2), nil
}

func (s *Store) adoptFileToBlob(path string) error {
	linked, err := s.isLinkedToBlob(path)
	if err != nil {
		return err
	}
	if linked {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	hash, err := s.ingestBlob(data)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return s.linkBlobToVersion(hash, path)
}

func copyBlobFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// BlobStats 返回 blob 去重存储统计（调试/运维）。
type BlobStats struct {
	BlobCount   int   `json:"blob_count"`
	TotalBytes  int64 `json:"total_bytes"`
	TotalRefs   int   `json:"total_refs"`
}

func (s *Store) BlobStats() (BlobStats, error) {
	var st BlobStats
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size),0), COALESCE(SUM(ref_count),0) FROM blobs`).
		Scan(&st.BlobCount, &st.TotalBytes, &st.TotalRefs)
	return st, err
}

func isVersionDataFile(userdataRoot, path string) bool {
	rel, err := filepath.Rel(userdataRoot, path)
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	// {slug}/projects/{project}/{version}/...
	if len(parts) < 5 || parts[1] != "projects" {
		return false
	}
	return true
}

func (s *Store) writeVersionBlob(tenantID int64, projectName, version, relPath string, r io.Reader) error {
	path, err := s.versionFilePath(tenantID, projectName, version, relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		if err := s.releaseBlobLink(path); err != nil {
			return err
		}
	}
	hash, err := s.ingestBlobFromReader(r)
	if err != nil {
		return fmt.Errorf("blob ingest: %w", err)
	}
	return s.linkBlobToVersion(hash, path)
}
