package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/express233/internal/filetags"
)

type VersionFileTag struct {
	Path string   `json:"path"`
	Tags []string `json:"tags"`
}

type FileTagBatchMode string

const (
	FileTagSet    FileTagBatchMode = "set"
	FileTagAdd    FileTagBatchMode = "add"
	FileTagRemove FileTagBatchMode = "remove"
	FileTagClear  FileTagBatchMode = "clear"
)

func (s *Store) ListVersionFileTags(tenantID int64, projectName, version string) ([]VersionFileTag, error) {
	files, err := s.ListVersionFiles(tenantID, projectName, version)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT path, tags FROM version_file_tags WHERE tenant_id = ? AND project_name = ? AND version = ?`,
		tenantID, projectName, version,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	byPath := map[string][]string{}
	for rows.Next() {
		var path, raw string
		if err := rows.Scan(&path, &raw); err != nil {
			return nil, err
		}
		byPath[filepath.ToSlash(path)] = filetags.Split(raw)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]VersionFileTag, 0, len(files))
	for _, path := range files {
		tags := byPath[path]
		if len(tags) == 0 {
			tags = []string{filetags.All}
		}
		out = append(out, VersionFileTag{Path: path, Tags: tags})
	}
	return out, nil
}

func (s *Store) TagsForVersionFile(tenantID int64, projectName, version, relPath string) ([]string, error) {
	relPath = cleanRelPath(relPath)
	var raw string
	err := s.db.QueryRow(
		`SELECT tags FROM version_file_tags WHERE tenant_id = ? AND project_name = ? AND version = ? AND path = ?`,
		tenantID, projectName, version, relPath,
	).Scan(&raw)
	if err == nil {
		return filetags.Split(raw), nil
	}
	return []string{filetags.All}, nil
}

func (s *Store) SetVersionFileTags(tenantID int64, projectName, version, relPath string, tags []string) error {
	if err := s.assertDraft(tenantID, projectName, version); err != nil {
		return err
	}
	relPath = cleanRelPath(relPath)
	if relPath == "." || strings.HasPrefix(relPath, "../") || filepath.IsAbs(relPath) {
		return fmt.Errorf("invalid path")
	}
	fullPath, err := s.versionFilePath(tenantID, projectName, version, relPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(fullPath); err != nil {
		return err
	}
	now := time.Now().Format(timeLayout)
	_, err = s.db.Exec(
		`INSERT INTO version_file_tags(tenant_id, project_name, version, path, tags, updated_at)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(tenant_id, project_name, version, path)
		 DO UPDATE SET tags = excluded.tags, updated_at = excluded.updated_at`,
		tenantID, projectName, version, relPath, filetags.Join(tags), now,
	)
	return err
}

func (s *Store) ClearVersionFileTags(tenantID int64, projectName, version, relPath string) error {
	if err := s.assertDraft(tenantID, projectName, version); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`DELETE FROM version_file_tags WHERE tenant_id = ? AND project_name = ? AND version = ? AND path = ?`,
		tenantID, projectName, version, cleanRelPath(relPath),
	)
	return err
}

func (s *Store) BatchUpdateVersionFileTags(tenantID int64, projectName, version string, paths, patterns, tags []string, mode FileTagBatchMode) ([]VersionFileTag, error) {
	if err := s.assertDraft(tenantID, projectName, version); err != nil {
		return nil, err
	}
	files, err := s.ListVersionFiles(tenantID, projectName, version)
	if err != nil {
		return nil, err
	}
	selected := selectTagPaths(files, paths, patterns)
	if len(selected) == 0 {
		return []VersionFileTag{}, nil
	}
	for _, path := range selected {
		current, err := s.TagsForVersionFile(tenantID, projectName, version, path)
		if err != nil {
			return nil, err
		}
		next := mergeTags(current, tags, mode)
		if mode == FileTagClear || len(next) == 0 {
			if err := s.ClearVersionFileTags(tenantID, projectName, version, path); err != nil {
				return nil, err
			}
			continue
		}
		if err := s.SetVersionFileTags(tenantID, projectName, version, path, next); err != nil {
			return nil, err
		}
	}
	all, err := s.ListVersionFileTags(tenantID, projectName, version)
	if err != nil {
		return nil, err
	}
	out := make([]VersionFileTag, 0, len(selected))
	selectedSet := map[string]bool{}
	for _, path := range selected {
		selectedSet[path] = true
	}
	for _, row := range all {
		if selectedSet[row.Path] {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *Store) DeleteVersionFileTags(tenantID int64, projectName, version, relPath string) error {
	_, err := s.db.Exec(
		`DELETE FROM version_file_tags WHERE tenant_id = ? AND project_name = ? AND version = ? AND path = ?`,
		tenantID, projectName, version, cleanRelPath(relPath),
	)
	return err
}

func cleanRelPath(path string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}

func selectTagPaths(files, paths, patterns []string) []string {
	selected := map[string]bool{}
	for _, path := range paths {
		path = cleanRelPath(path)
		for _, file := range files {
			if file == path {
				selected[file] = true
			}
		}
	}
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		for _, file := range files {
			if ok, _ := filepath.Match(pattern, file); ok {
				selected[file] = true
				continue
			}
			if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(file, strings.TrimSuffix(pattern, "**")) {
				selected[file] = true
			}
		}
	}
	out := make([]string, 0, len(selected))
	for _, file := range files {
		if selected[file] {
			out = append(out, file)
		}
	}
	return out
}

func mergeTags(current, input []string, mode FileTagBatchMode) []string {
	switch mode {
	case FileTagClear:
		return []string{filetags.All}
	case FileTagAdd:
		return filetags.Normalize(append(current, input...))
	case FileTagRemove:
		remove := map[string]bool{}
		for _, tag := range filetags.Normalize(input) {
			remove[tag] = true
		}
		var out []string
		for _, tag := range filetags.Normalize(current) {
			if !remove[tag] {
				out = append(out, tag)
			}
		}
		return filetags.Normalize(out)
	default:
		return filetags.Normalize(input)
	}
}
