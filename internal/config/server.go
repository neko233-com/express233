package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerFile 中央 server.yaml 结构。
type ServerFile struct {
	Servers map[string]ServerEntry `yaml:"servers"`
}

// LoadServerFile 从路径加载 server.yaml。
func LoadServerFile(path string) (*ServerFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read server config: %w", err)
	}
	var sf ServerFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parse server config: %w", err)
	}
	if sf.Servers == nil {
		sf.Servers = make(map[string]ServerEntry)
	}
	return &sf, nil
}

// Entry 返回 server_id 对应条目，不存在则 nil。
func (sf *ServerFile) Entry(serverID string) *ServerEntry {
	if sf == nil {
		return nil
	}
	e, ok := sf.Servers[serverID]
	if !ok {
		return nil
	}
	return &e
}
