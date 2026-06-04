package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/neko233-com/express233/internal/template"
)

// Validate 校验 server.yaml 语义（replacements 键必须为配置文件 basename）。
func (sf *ServerFile) Validate() error {
	if sf == nil {
		return nil
	}
	for sid, entry := range sf.Servers {
		for name, raw := range entry.Replacements {
			base := template.Basename(name)
			if base != name || strings.Contains(name, "/") || strings.Contains(name, `\`) {
				return fmt.Errorf("servers.%s.replacements: key %q must be config filename only (basename), not a path", sid, name)
			}
			if !template.IsConfigFile(base) {
				return fmt.Errorf("servers.%s.replacements: %q is not a supported config extension", sid, base)
			}
			if len(raw) == 0 {
				return fmt.Errorf("servers.%s.replacements.%s: at least one key required", sid, base)
			}
			if _, err := raw.Normalize(base); err != nil {
				return fmt.Errorf("servers.%s.replacements.%s: %w", sid, base, err)
			}
		}
	}
	return nil
}

// ServerIDs 返回所有 server_id 列表。
func (sf *ServerFile) ServerIDs() []string {
	if sf == nil || len(sf.Servers) == 0 {
		return nil
	}
	ids := make([]string, 0, len(sf.Servers))
	for id := range sf.Servers {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
