package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/neko233-com/express233/internal/template"
)

// ValidateUniqueConfigBasenames 确保版本目录内配置文件 basename 全局唯一。
func ValidateUniqueConfigBasenames(versionRoot string) error {
	counts := make(map[string][]string)
	err := filepath.Walk(versionRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !template.IsConfigFile(info.Name()) {
			return nil
		}
		rel, err := filepath.Rel(versionRoot, path)
		if err != nil {
			return err
		}
		base := info.Name()
		counts[base] = append(counts[base], filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return err
	}
	for base, paths := range counts {
		if len(paths) > 1 {
			return fmt.Errorf("配置文件名必须全局唯一: %q 出现在 %v（请合并或重命名后再上传）", base, paths)
		}
	}
	return nil
}

// CheckConfigBasenameConflict 上传前检查：新文件 basename 是否与已有配置冲突。
func CheckConfigBasenameConflict(versionRoot, relPath string) error {
	base := filepath.Base(filepath.FromSlash(relPath))
	if !template.IsConfigFile(base) {
		return nil
	}
	matches, err := template.FindConfigFilesByBasename(versionRoot)
	if err != nil {
		return err
	}
	existing := matches[base]
	newRel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relPath)))
	for _, p := range existing {
		if p != newRel {
			return fmt.Errorf("配置文件名 %q 已存在于 %s，不允许第二个同名配置（无视路径树）", base, p)
		}
	}
	return nil
}
