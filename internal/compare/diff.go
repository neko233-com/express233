package compare

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/template"
	"gopkg.in/yaml.v3"
)

// KeyDelta 键级差异。
type KeyDelta struct {
	Key    string `json:"key"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Change string `json:"change"` // added | removed | modified
}

// FileDelta 单配置文件差异。
type FileDelta struct {
	Basename string     `json:"basename"`
	Keys     []KeyDelta `json:"keys"`
}

// VersionDiffReport 两版本在指定 server_id 下的有效配置差异。
type VersionDiffReport struct {
	Project     string      `json:"project"`
	FromVersion string      `json:"from_version"`
	ToVersion   string      `json:"to_version"`
	ServerID    string      `json:"server_id"`
	Files       []FileDelta `json:"files"`
}

// DiffVersions 对比两版本经 server_id 替换后的配置键值。
func DiffVersions(fromRoot, toRoot, project, fromVer, toVer, serverID string, entry *config.ServerEntry) (*VersionDiffReport, error) {
	report := &VersionDiffReport{
		Project:     project,
		FromVersion: fromVer,
		ToVersion:   toVer,
		ServerID:    serverID,
	}
	byBaseFrom, err := renderEffectiveConfig(fromRoot, entry)
	if err != nil {
		return nil, err
	}
	byBaseTo, err := renderEffectiveConfig(toRoot, entry)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for b := range byBaseFrom {
		seen[b] = true
	}
	for b := range byBaseTo {
		seen[b] = true
	}
	for base := range seen {
		fd := FileDelta{Basename: base}
		fromKeys := byBaseFrom[base]
		toKeys := byBaseTo[base]
		all := make(map[string]bool)
		for k := range fromKeys {
			all[k] = true
		}
		for k := range toKeys {
			all[k] = true
		}
		keys := make([]string, 0, len(all))
		for key := range all {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fv, fok := fromKeys[key]
			tv, tok := toKeys[key]
			d := KeyDelta{Key: key, From: fv, To: tv}
			if !fok && tok {
				d.Change = "added"
				d.From = ""
				d.To = tv
			} else if fok && !tok {
				d.Change = "removed"
				d.From = fv
				d.To = ""
			} else if fv != tv {
				d.Change = "modified"
			} else {
				continue
			}
			fd.Keys = append(fd.Keys, d)
		}
		if len(fd.Keys) > 0 {
			report.Files = append(report.Files, fd)
		}
	}
	return report, nil
}

func renderEffectiveConfig(root string, entry *config.ServerEntry) (map[string]map[string]string, error) {
	matches, err := template.FindConfigFilesByBasename(root)
	if err != nil {
		return nil, err
	}
	replacements := map[string]map[string]any{}
	if entry != nil {
		replacements, err = config.PrepareReplacements(entry.Replacements)
		if err != nil {
			return nil, err
		}
	}
	out := make(map[string]map[string]string, len(matches))
	for base, paths := range matches {
		if len(paths) == 0 {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(paths[0]))
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		if tree := replacements[base]; len(tree) > 0 {
			data, err = template.MergeBytes(base, data, tree)
			if err != nil {
				return nil, err
			}
		}
		flat, err := flattenConfig(base, data)
		if err != nil {
			return nil, err
		}
		out[base] = flat
	}
	return out, nil
}

func flattenConfig(basename string, data []byte) (map[string]string, error) {
	switch strings.ToLower(filepath.Ext(basename)) {
	case ".properties":
		return flattenProperties(string(data)), nil
	case ".yaml", ".yml":
		var root any
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, err
		}
		return flattenStructured(root)
	case ".json":
		var root any
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, err
		}
		return flattenStructured(root)
	default:
		return nil, fmt.Errorf("unsupported config type %q", basename)
	}
}

func flattenStructured(root any) (map[string]string, error) {
	m, err := template.AsStringMap(root)
	if err != nil {
		return nil, err
	}
	flatAny := template.FlattenScalars("", m)
	out := make(map[string]string, len(flatAny))
	for k, v := range flatAny {
		out[k] = fmt.Sprint(v)
	}
	return out, nil
}

func flattenProperties(content string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		sep := strings.IndexAny(line, "=:")
		if sep < 0 {
			continue
		}
		out[strings.TrimSpace(line[:sep])] = strings.TrimSpace(line[sep+1:])
	}
	return out
}
