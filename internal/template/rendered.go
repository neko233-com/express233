package template

import (
	"os"
	"path/filepath"
)

// RenderedFile 替换后的完整配置文件内容（用于预览/校验）。
type RenderedFile struct {
	Basename string `json:"basename"`
	Path     string `json:"path"`
	Before   string `json:"before"`
	After    string `json:"after"`
}

// BuildRenderedFiles 对版本内所有被 replacements 触及的配置文件生成 before/after 全文。
func BuildRenderedFiles(root string, byFile map[string]map[string]any) ([]RenderedFile, error) {
	normalized, err := NormalizeFileOverrides(byFile)
	if err != nil {
		return nil, err
	}
	matches, err := FindConfigFilesByBasename(root)
	if err != nil {
		return nil, err
	}
	var out []RenderedFile
	for base, tree := range normalized {
		paths := matches[base]
		if len(paths) == 0 {
			continue
		}
		rel := paths[0]
		full := filepath.Join(root, filepath.FromSlash(rel))
		before, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		after, err := MergeBytes(base, before, tree)
		if err != nil {
			return nil, err
		}
		out = append(out, RenderedFile{
			Basename: base,
			Path:     rel,
			Before:   string(before),
			After:    string(after),
		})
	}
	// 也包含版本内其它配置文件（无覆盖）可选 — 仅返回有覆盖的
	return out, nil
}
