package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileOverrideSet 按 basename 的覆盖树（已 Normalize）。
type FileOverrideSet map[string]map[string]any

// TransformBytes 在内存中应用覆盖（深度合并或 properties 键替换）。
func TransformBytes(filename string, data []byte, override map[string]any) ([]byte, error) {
	return MergeBytes(filename, data, override)
}

// TransformBytesLegacy 扁平 string 键（向后兼容）。
func TransformBytesLegacy(filename string, data []byte, kv map[string]string) ([]byte, error) {
	if len(kv) == 0 {
		return data, nil
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".yaml", ".yml":
		return replaceYAML(data, kv)
	case ".json":
		return replaceJSON(data, kv)
	case ".properties":
		return replaceProperties(string(data), kv)
	default:
		return nil, fmt.Errorf("unsupported config type %q", ext)
	}
}

// ApplyByBasename 无视目录树：对 root 下所有 basename 匹配的文件应用同一套替换。
func ApplyByBasename(root string, byFile map[string]map[string]any) error {
	normalized, err := NormalizeFileOverrides(byFile)
	if err != nil {
		return err
	}
	matches, err := FindConfigFilesByBasename(root)
	if err != nil {
		return err
	}
	for base, kv := range normalized {
		if len(kv) == 0 {
			continue
		}
		paths, ok := matches[base]
		if !ok || len(paths) == 0 {
			return fmt.Errorf("replacement configured for %q but no such file in version", base)
		}
		for _, rel := range paths {
			path := filepath.Join(root, filepath.FromSlash(rel))
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			out, err := MergeBytes(base, data, kv)
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			if err := os.WriteFile(path, out, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// NormalizeFileOverrides 将 server.yaml 键规范为 basename 并 Normalize 覆盖树。
func NormalizeFileOverrides(byFile map[string]map[string]any) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any)
	for name, tree := range byFile {
		base := Basename(name)
		if out[base] == nil {
			out[base] = make(map[string]any)
		}
		if err := DeepMergeInPlace(out[base], tree); err != nil {
			return nil, fmt.Errorf("%s: %w", base, err)
		}
	}
	return out, nil
}

// OverridesFromConfig 将 config.FileOverrides 转为可合并的树。
func OverridesFromConfig(byFile map[string]map[string]any) (map[string]map[string]any, error) {
	return NormalizeFileOverrides(byFile)
}

// OverridesFromLegacy 旧版 map[string]map[string]string。
func OverridesFromLegacy(byFile map[string]map[string]string) map[string]map[string]any {
	out := make(map[string]map[string]any)
	for name, kv := range byFile {
		base := Basename(name)
		if out[base] == nil {
			out[base] = make(map[string]any)
		}
		for k, v := range kv {
			_ = SetNestedMap(out[base], k, v)
		}
	}
	return out
}

// FindConfigFilesByBasename 返回 basename -> 版本内所有相对路径（仅配置文件）。
func FindConfigFilesByBasename(root string) (map[string][]string, error) {
	out := make(map[string][]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := info.Name()
		if !IsConfigFile(base) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		out[base] = append(out[base], rel)
		return nil
	})
	return out, err
}
