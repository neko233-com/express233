package store

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

// StreamVersionArchive 将版本目录原样打包为 tar.gz（不做 server_id 替换）。
func (s *Store) StreamVersionArchive(tenantID int64, projectName, version string, w io.Writer) error {
	root, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); err != nil {
		return err
	}
	gzw := gzip.NewWriter(w)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name:    filepath.ToSlash(rel),
			Mode:    0o644,
			Size:    int64(len(data)),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
}
