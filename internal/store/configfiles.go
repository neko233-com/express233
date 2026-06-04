package store

import (
	"github.com/neko233-com/express233/internal/template"
)

// ConfigFileEntry 版本内配置文件索引（basename 唯一约束）。
type ConfigFileEntry struct {
	Basename string `json:"basename"`
	Path     string `json:"path"`
}

// ListConfigFileEntries 列出所有配置文件及其路径。
func (s *Store) ListConfigFileEntries(tenantID int64, projectName, version string) ([]ConfigFileEntry, error) {
	root, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return nil, err
	}
	m, err := template.FindConfigFilesByBasename(root)
	if err != nil {
		return nil, err
	}
	var out []ConfigFileEntry
	for base, paths := range m {
		for _, p := range paths {
			out = append(out, ConfigFileEntry{Basename: base, Path: p})
		}
	}
	return out, nil
}
