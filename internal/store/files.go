package store

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/neko233-com/express233/internal/template"
)

// WriteVersionFile 写入版本内文件（仅 draft）。
func (s *Store) WriteVersionFile(tenantID int64, projectName, version, relPath string, r io.Reader) error {
	if err := s.assertDraft(tenantID, projectName, version); err != nil {
		return err
	}
	root, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return err
	}
	if err := CheckConfigBasenameConflict(root, relPath); err != nil {
		return err
	}
	return s.writeVersionBlob(tenantID, projectName, version, relPath, r)
}

// DeleteVersionFile 删除版本内文件（仅 draft）。
func (s *Store) DeleteVersionFile(tenantID int64, projectName, version, relPath string) error {
	if err := s.assertDraft(tenantID, projectName, version); err != nil {
		return err
	}
	path, err := s.versionFilePath(tenantID, projectName, version, relPath)
	if err != nil {
		return err
	}
	return s.releaseBlobLink(path)
}

// ExtractZipToVersion 解压 zip 到版本目录（仅 draft）。
func (s *Store) ExtractZipToVersion(tenantID int64, projectName, version string, r io.ReaderAt, size int64) error {
	if err := s.assertDraft(tenantID, projectName, version); err != nil {
		return err
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	if err := validateZipConfigBasenames(zr); err != nil {
		return err
	}
	root, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Clean(f.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("invalid zip entry: %s", f.Name)
		}
		dest := filepath.Join(root, name)
		if !strings.HasPrefix(dest, filepath.Clean(root)+string(os.PathSeparator)) && dest != filepath.Clean(root) {
			if !strings.HasPrefix(dest, root) {
				return fmt.Errorf("zip path escape: %s", f.Name)
			}
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(dest); err == nil {
			if err := s.releaseBlobLink(dest); err != nil {
				return err
			}
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		hash, ingestErr := s.ingestBlobFromReader(rc)
		_ = rc.Close()
		if ingestErr != nil {
			return ingestErr
		}
		if err := s.linkBlobToVersion(hash, dest); err != nil {
			return err
		}
	}
	return ValidateUniqueConfigBasenames(root)
}

// ExtractTarToVersion 解压 tar/tar.gz 到版本目录（仅 draft）。
func (s *Store) ExtractTarToVersion(tenantID int64, projectName, version string, r io.Reader, gzipped bool) error {
	if err := s.assertDraft(tenantID, projectName, version); err != nil {
		return err
	}
	if gzipped {
		gr, err := gzip.NewReader(r)
		if err != nil {
			return err
		}
		defer func() { _ = gr.Close() }()
		r = gr
	}
	root, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return err
	}
	tr := tar.NewReader(r)
	seen := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." {
			continue
		}
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("invalid tar entry: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return fmt.Errorf("unsupported tar entry: %s", hdr.Name)
		}
		base := filepath.Base(name)
		if template.IsConfigFile(base) {
			if prev, ok := seen[base]; ok {
				return fmt.Errorf("tar contains duplicate config filename %q in %s and %s", base, prev, hdr.Name)
			}
			seen[base] = hdr.Name
		}
		dest := filepath.Join(root, name)
		if !strings.HasPrefix(dest, filepath.Clean(root)+string(os.PathSeparator)) && dest != filepath.Clean(root) {
			if !strings.HasPrefix(dest, root) {
				return fmt.Errorf("tar path escape: %s", hdr.Name)
			}
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(dest); err == nil {
			if err := s.releaseBlobLink(dest); err != nil {
				return err
			}
		}
		hash, err := s.ingestBlobFromReader(tr)
		if err != nil {
			return err
		}
		if err := s.linkBlobToVersion(hash, dest); err != nil {
			return err
		}
	}
	return ValidateUniqueConfigBasenames(root)
}

func (s *Store) assertDraft(tenantID int64, projectName, version string) error {
	return s.assertMutable(tenantID, projectName, version)
}

// SafeJoinVersion 安全拼接版本路径。
func SafeJoinVersion(root, relPath string) (string, error) {
	root = filepath.Clean(root)
	rel := filepath.Clean(filepath.FromSlash(relPath))
	dest := filepath.Join(root, rel)
	if !strings.HasPrefix(dest, root+string(os.PathSeparator)) && dest != root {
		return "", fmt.Errorf("invalid path")
	}
	return dest, nil
}

func (s *Store) versionFilePath(tenantID int64, projectName, version, relPath string) (string, error) {
	root, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return "", err
	}
	return SafeJoinVersion(root, relPath)
}

func validateZipConfigBasenames(zr *zip.Reader) error {
	seen := make(map[string]string)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if !template.IsConfigFile(base) {
			continue
		}
		if prev, ok := seen[base]; ok {
			return fmt.Errorf("zip contains duplicate config filename %q in %s and %s", base, prev, f.Name)
		}
		seen[base] = f.Name
	}
	return nil
}
