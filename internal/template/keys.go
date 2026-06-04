package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadKey 读取配置文件中某个键的当前值（展示预览用）。
func ReadKey(filename string, data []byte, dottedKey string) (string, bool) {
	ext := strings.ToLower(filepathExt(filename))
	switch ext {
	case ".yaml", ".yml":
		return readYAMLKey(data, dottedKey)
	case ".json":
		return readJSONKey(data, dottedKey)
	case ".properties":
		return readPropertiesKey(string(data), dottedKey)
	default:
		return "", false
	}
}

func filepathExt(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i:]
		}
	}
	return ""
}

func readPropertiesKey(content, key string) (string, bool) {
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		sep := strings.IndexAny(line, "=:")
		if sep < 0 {
			continue
		}
		k := strings.TrimSpace(line[:sep])
		if k == key {
			return strings.TrimSpace(line[sep+1:]), true
		}
	}
	return "", false
}

func readYAMLKey(data []byte, dottedKey string) (string, bool) {
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", false
	}
	v, ok := getNested(root, dottedKey)
	if !ok {
		return "", false
	}
	return fmt.Sprint(v), true
}

func readJSONKey(data []byte, dottedKey string) (string, bool) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return "", false
	}
	v, ok := getNested(root, dottedKey)
	if !ok {
		return "", false
	}
	return fmt.Sprint(v), true
}

func getNested(root any, dottedKey string) (any, bool) {
	parts := strings.Split(dottedKey, ".")
	cur := root
	for i, p := range parts {
		m, err := asMap(cur)
		if err != nil {
			return nil, false
		}
		next, ok := m[p]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return next, true
		}
		cur = next
	}
	return nil, false
}
