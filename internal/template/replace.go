package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ApplyReplacements 对版本树按配置文件 basename 应用替换（无视路径）。
func ApplyReplacements(root string, files map[string]map[string]any) error {
	return ApplyByBasename(root, files)
}

func replaceYAML(data []byte, kv map[string]string) ([]byte, error) {
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	for key, val := range kv {
		if err := setNested(&root, key, val); err != nil {
			return nil, err
		}
	}
	return yaml.Marshal(root)
}

func replaceJSON(data []byte, kv map[string]string) ([]byte, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	for key, val := range kv {
		if err := setNested(&root, key, val); err != nil {
			return nil, err
		}
	}
	return json.MarshalIndent(root, "", "  ")
}

func replaceProperties(content string, kv map[string]string) ([]byte, error) {
	lines := strings.Split(content, "\n")
	keys := make(map[string]string, len(kv))
	for k, v := range kv {
		keys[k] = v
	}
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "!") {
			continue
		}
		sep := strings.IndexAny(line, "=:")
		if sep < 0 {
			continue
		}
		key := strings.TrimSpace(line[:sep])
		if v, ok := keys[key]; ok {
			lines[i] = key + "=" + v
			delete(keys, key)
		}
	}
	for k, v := range keys {
		lines = append(lines, k+"="+v)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func setNested(root *any, dottedKey, value string) error {
	parts := strings.Split(dottedKey, ".")
	if len(parts) == 0 {
		return fmt.Errorf("empty key")
	}
	cur := root
	for i, p := range parts[:len(parts)-1] {
		m, err := asMap(*cur)
		if err != nil {
			return fmt.Errorf("key %q at segment %d: %w", dottedKey, i, err)
		}
		next, ok := m[p]
		if !ok {
			child := map[string]any{}
			m[p] = child
			next = child
		}
		child, err := asMap(next)
		if err != nil {
			return fmt.Errorf("key %q at segment %d: %w", dottedKey, i, err)
		}
		*cur = m
		cur = ptr(child)
	}
	m, err := asMap(*cur)
	if err != nil {
		return err
	}
	m[parts[len(parts)-1]] = coerce(value)
	*cur = m
	return nil
}

func ptr(m map[string]any) *any {
	var a any = m
	return &a
}

func asMap(v any) (map[string]any, error) {
	switch t := v.(type) {
	case map[string]any:
		return t, nil
	case map[interface{}]interface{}:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = val
		}
		return out, nil
	default:
		return nil, fmt.Errorf("not a map")
	}
}

func coerce(s string) any {
	if s == "true" || s == "false" {
		return s == "true"
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
		return n
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}
	return s
}
