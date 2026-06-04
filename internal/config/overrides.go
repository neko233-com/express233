package config

import (
	"fmt"
	"strings"

	"github.com/neko233-com/express233/internal/template"
)

// FileOverrides 单个配置文件（basename）的覆盖树，支持嵌套 YAML 结构。
// 仍兼容扁平 dotted 键（如 mysql.url: "x"）。
type FileOverrides map[string]any

// ServerEntry 单个逻辑服配置。
type ServerEntry struct {
	Replacements map[string]FileOverrides `yaml:"replacements"`
	PostHook     string                   `yaml:"post_hook"`
	PostHookEnv  map[string]string        `yaml:"post_hook_env"`
}

// Normalize 将覆盖树转为 template 可用的结构（嵌套 map 或 properties 扁平键）。
func (o FileOverrides) Normalize(basename string) (map[string]any, error) {
	if len(o) == 0 {
		return nil, fmt.Errorf("empty overrides")
	}
	ext := strings.ToLower(template.ConfigExt(basename))
	switch ext {
	case ".yaml", ".yml", ".json":
		return normalizeStructured(o, basename)
	case ".properties":
		return normalizeProperties(o, basename)
	default:
		return nil, fmt.Errorf("unsupported config extension for %q", basename)
	}
}

func normalizeStructured(o FileOverrides, basename string) (map[string]any, error) {
	out := make(map[string]any)
	for k, v := range o {
		if strings.Contains(k, ".") {
			if _, isMap := v.(map[string]any); isMap {
				return nil, fmt.Errorf("%s: key %q mixes dotted path with nested map", basename, k)
			}
			if err := template.SetNestedMap(out, k, v); err != nil {
				return nil, fmt.Errorf("%s: %w", basename, err)
			}
			continue
		}
		if fo, ok := v.(FileOverrides); ok {
			if err := mergeSubtree(out, k, map[string]any(fo)); err != nil {
				return nil, fmt.Errorf("%s.%s: %w", basename, k, err)
			}
			continue
		}
		if sub, err := template.AsStringMap(v); err == nil {
			if err := mergeSubtree(out, k, sub); err != nil {
				return nil, fmt.Errorf("%s.%s: %w", basename, k, err)
			}
			continue
		}
		if !template.IsScalar(v) {
			return nil, fmt.Errorf("%s: key %q must be scalar or nested map", basename, k)
		}
		out[k] = v
	}
	return out, nil
}

func mergeSubtree(dst map[string]any, key string, src map[string]any) error {
	existing, ok := dst[key]
	if !ok {
		dst[key] = template.DeepCopyMap(src)
		return nil
	}
	existMap, err := template.AsStringMap(existing)
	if err != nil {
		return fmt.Errorf("override type mismatch at %q: expected map", key)
	}
	return template.DeepMergeInPlace(existMap, src)
}

func normalizeProperties(o FileOverrides, basename string) (map[string]any, error) {
	flat := make(map[string]any)
	for k, v := range o {
		switch t := v.(type) {
		case map[string]any:
			for dk, dv := range template.FlattenScalars(k, t) {
				flat[dk] = dv
			}
		case map[interface{}]interface{}:
			for dk, dv := range template.FlattenScalars(k, template.YAMLMapToStringMap(t)) {
				flat[dk] = dv
			}
		default:
			if !template.IsScalar(v) {
				return nil, fmt.Errorf("%s: properties override %q must be scalar", basename, k)
			}
			if strings.Contains(k, ".") {
				flat[k] = v
			} else {
				flat[k] = v
			}
		}
	}
	return flat, nil
}
