package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigExt 返回配置文件扩展名（小写）。
func ConfigExt(basename string) string {
	return strings.ToLower(filepathExt(basename))
}

// IsScalar 是否为可写入配置的标量。
func IsScalar(v any) bool {
	switch v.(type) {
	case string, bool, int, int64, float64, float32, nil:
		return true
	default:
		return false
	}
}

// YAMLMapToStringMap 转换 yaml.v3 解析的 map。
func YAMLMapToStringMap(m map[interface{}]interface{}) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		sk := fmt.Sprint(k)
		switch t := v.(type) {
		case map[interface{}]interface{}:
			out[sk] = YAMLMapToStringMap(t)
		default:
			out[sk] = v
		}
	}
	return out
}

// SetNestedMap 在树中按 dotted 路径设置标量/子树。
func SetNestedMap(root map[string]any, dottedKey string, value any) error {
	parts := strings.Split(dottedKey, ".")
	if len(parts) == 0 {
		return fmt.Errorf("empty key")
	}
	cur := root
	for i, p := range parts[:len(parts)-1] {
		next, ok := cur[p]
		if !ok {
			child := map[string]any{}
			cur[p] = child
			cur = child
			continue
		}
		child, err := AsStringMap(next)
		if err != nil {
			return fmt.Errorf("key %q at segment %d: %w", dottedKey, i, err)
		}
		cur = child
	}
	last := parts[len(parts)-1]
	if existing, ok := cur[last]; ok {
		if _, isMap := existing.(map[string]any); isMap {
			if _, isMap := value.(map[string]any); isMap {
				return fmt.Errorf("key %q: cannot replace map with scalar or vice versa", dottedKey)
			}
		}
	}
	cur[last] = value
	return nil
}

// AsStringMap 将 any 转为 map[string]any。
func AsStringMap(v any) (map[string]any, error) {
	switch t := v.(type) {
	case map[string]any:
		return t, nil
	case map[interface{}]interface{}:
		return YAMLMapToStringMap(t), nil
	default:
		return nil, fmt.Errorf("not a map")
	}
}

// DeepCopyMap 浅拷贝嵌套 map。
func DeepCopyMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		if sub, ok := v.(map[string]any); ok {
			out[k] = DeepCopyMap(sub)
			continue
		}
		if sub, ok := v.(map[interface{}]interface{}); ok {
			out[k] = DeepCopyMap(YAMLMapToStringMap(sub))
			continue
		}
		out[k] = v
	}
	return out
}

// DeepMergeInPlace 将 src 合并进 dst，仅覆盖 src 中出现的键；类型不一致则报错。
func DeepMergeInPlace(dst, src map[string]any) error {
	for k, sv := range src {
		dv, ok := dst[k]
		if !ok {
			if sub, isMap := sv.(map[string]any); isMap {
				dst[k] = DeepCopyMap(sub)
			} else if sub, isMap := sv.(map[interface{}]interface{}); isMap {
				dst[k] = DeepCopyMap(YAMLMapToStringMap(sub))
			} else {
				dst[k] = sv
			}
			continue
		}
		srcMap, srcIsMap := sv.(map[string]any)
		if !srcIsMap {
			if sub, ok := sv.(map[interface{}]interface{}); ok {
				srcMap = YAMLMapToStringMap(sub)
				srcIsMap = true
			}
		}
		dstMap, dstIsMap := dv.(map[string]any)
		if !dstIsMap {
			if sub, ok := dv.(map[interface{}]interface{}); ok {
				dstMap = YAMLMapToStringMap(sub)
				dstIsMap = true
			}
		}
		if srcIsMap && dstIsMap {
			if err := DeepMergeInPlace(dstMap, srcMap); err != nil {
				return err
			}
			dst[k] = dstMap
			continue
		}
		if srcIsMap != dstIsMap {
			return fmt.Errorf("type mismatch at key %q", k)
		}
		dst[k] = sv
	}
	return nil
}

// FlattenScalars 将嵌套 map 展平为 dotted 键（仅 properties）。
func FlattenScalars(prefix string, m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch t := v.(type) {
		case map[string]any:
			for fk, fv := range FlattenScalars(key, t) {
				out[fk] = fv
			}
		case map[interface{}]interface{}:
			for fk, fv := range FlattenScalars(key, YAMLMapToStringMap(t)) {
				out[fk] = fv
			}
		default:
			out[key] = v
		}
	}
	return out
}

// MergeBytes 将覆盖树深度合并进配置文件内容。
func MergeBytes(basename string, data []byte, override map[string]any) ([]byte, error) {
	if len(override) == 0 {
		return data, nil
	}
	ext := ConfigExt(basename)
	switch ext {
	case ".yaml", ".yml":
		return mergeYAML(data, override)
	case ".json":
		return mergeJSON(data, override)
	case ".properties":
		return mergeProperties(string(data), override)
	default:
		return nil, fmt.Errorf("unsupported config type %q", ext)
	}
}

func mergeYAML(data []byte, override map[string]any) ([]byte, error) {
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	m, err := AsStringMap(root)
	if err != nil {
		return nil, fmt.Errorf("yaml root must be a mapping")
	}
	if err := DeepMergeInPlace(m, override); err != nil {
		return nil, err
	}
	return yaml.Marshal(m)
}

func mergeJSON(data []byte, override map[string]any) ([]byte, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	m, err := AsStringMap(root)
	if err != nil {
		return nil, fmt.Errorf("json root must be an object")
	}
	if err := DeepMergeInPlace(m, override); err != nil {
		return nil, err
	}
	return json.MarshalIndent(m, "", "  ")
}

func mergeProperties(content string, override map[string]any) ([]byte, error) {
	flatAny := FlattenScalars("", override)
	flat := make(map[string]string, len(flatAny))
	for k, v := range flatAny {
		flat[k] = fmt.Sprint(v)
	}
	return replaceProperties(content, flat)
}
