package config

import "github.com/neko233-com/express233/internal/template"

// PrepareReplacements 将 server.yaml 中的覆盖转为按 basename 的合并树。
func PrepareReplacements(byFile map[string]FileOverrides) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any, len(byFile))
	for name, raw := range byFile {
		base := template.Basename(name)
		norm, err := raw.Normalize(base)
		if err != nil {
			return nil, err
		}
		out[base] = norm
	}
	return out, nil
}
