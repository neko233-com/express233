package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Store) BlobStorageName() string { return s.blobs.Name() }

func blobKey(hash string) string { return "blobs/" + hash[:2] + "/" + hash }

func (s *Store) putBlob(hash string, size int64, data io.Reader) error {
	if err := s.blobs.Put(context.Background(), hash, size, data); err != nil {
		return fmt.Errorf("store artifact: %w", err)
	}
	_, err := s.db.Exec(`
		INSERT INTO blobs(hash, size, ref_count, created_at) VALUES(?,?,1,?)
		ON CONFLICT(hash) DO UPDATE SET ref_count = ref_count + 1
	`, hash, size, time.Now().Format(timeLayout))
	return err
}

func (s *Store) materializeBlob(hash, dest string) error {
	if local, ok := s.blobs.(localBlobStorage); ok {
		staging, err := os.MkdirTemp(filepath.Dir(dest), ".artifact-link-*")
		if err != nil {
			return err
		}
		tmp := filepath.Join(staging, "file")
		defer func() {
			_ = os.Remove(tmp)
			_ = os.Remove(staging)
		}()
		if err := os.Link(local.path(hash), tmp); err == nil {
			return os.Rename(tmp, dest)
		}
	}
	in, err := s.blobs.Open(context.Background(), hash)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	return writeArtifactAtomically(dest, in)
}

func (s *Store) replaceArtifact(path string, r io.Reader) error {
	// Spool once to compute the content key without sizing memory by upload size.
	f, err := os.CreateTemp("", "express233-artifact-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), r)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	hash := hex.EncodeToString(h.Sum(nil))

	s.artifactMu.Lock()
	defer s.artifactMu.Unlock()
	oldHash, err := hashFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := s.putBlob(hash, size, f); err != nil {
		return err
	}
	if err := s.materializeBlob(hash, path); err != nil {
		return errors.Join(err, s.decrementBlobRef(hash))
	}
	if oldHash != "" {
		return s.decrementBlobRef(oldHash)
	}
	return nil
}

func (s *Store) releaseBlobLink(path string) error {
	s.artifactMu.Lock()
	defer s.artifactMu.Unlock()
	hash, err := hashFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return s.decrementBlobRef(hash)
}

func (s *Store) releaseVersionDir(root string) error {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
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

func (s *Store) decrementBlobRef(hash string) error {
	var refs int
	err := s.db.QueryRow(`UPDATE blobs SET ref_count = MAX(0, ref_count - 1) WHERE hash = ? RETURNING ref_count`, hash).Scan(&refs)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && refs > 0) {
		return nil
	}
	if err != nil {
		return err
	}
	// A physical cleanup failure leaves the zero-reference row available
	// through the storage index, without failing the committed version change.
	_ = s.deleteUnreferencedBlob(hash)
	return nil
}

// The caller holds artifactMu. A failed delete leaves a zero-reference row
// so a later upload can reuse the object or an operator can retry cleanup.
func (s *Store) deleteUnreferencedBlob(hash string) error {
	var refs int
	err := s.db.QueryRow(`SELECT ref_count FROM blobs WHERE hash = ?`, hash).Scan(&refs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if refs > 0 {
		return fmt.Errorf("blob still has %d references", refs)
	}
	if err := s.blobs.Delete(context.Background(), hash); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM blobs WHERE hash = ? AND ref_count = 0`, hash)
	return err
}

func hashFile(path string) (string, error) {
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

// BlobStats 返回 blob 去重存储统计。
type BlobStats struct {
	BlobCount  int   `json:"blob_count"`
	TotalBytes int64 `json:"total_bytes"`
	TotalRefs  int   `json:"total_refs"`
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
	return len(parts) >= 5 && parts[1] == "projects"
}
