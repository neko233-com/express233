package template

import (
	"path/filepath"
	"strings"
)

// IsConfigFile 是否为受管配置文件（上传包内 basename 必须全局唯一）。
func IsConfigFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".yaml", ".yml", ".json", ".properties":
		return true
	default:
		return false
	}
}

// Basename 规范化配置文件名（server.yaml 的 replacements 键）。
func Basename(name string) string {
	return filepath.Base(filepath.FromSlash(name))
}
