package compare

import (
	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/template"
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
	if entry == nil {
		return report, nil
	}
	rep, err := config.PrepareReplacements(entry.Replacements)
	if err != nil {
		return nil, err
	}
	prevFrom, err := template.BuildPreview(fromRoot, project, fromVer, serverID, rep, entry.PostHook, entry.PostHookEnv)
	if err != nil {
		return nil, err
	}
	prevTo, err := template.BuildPreview(toRoot, project, toVer, serverID, rep, entry.PostHook, entry.PostHookEnv)
	if err != nil {
		return nil, err
	}
	byBaseFrom := indexPreview(prevFrom)
	byBaseTo := indexPreview(prevTo)

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
		for key := range all {
			fv, fok := fromKeys[key]
			tv, tok := toKeys[key]
			d := KeyDelta{Key: key, From: fv.Before, To: tv.After}
			if !fok && tok {
				d.Change = "added"
				d.From = ""
				d.To = tv.After
			} else if fok && !tok {
				d.Change = "removed"
				d.From = fv.Before
				d.To = ""
			} else if fv.After != tv.After || fv.Before != tv.Before {
				d.Change = "modified"
				d.From = fv.After
				d.To = tv.After
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

func indexPreview(p *template.PreviewReport) map[string]map[string]template.KeyChange {
	out := make(map[string]map[string]template.KeyChange)
	for _, f := range p.Files {
		m := make(map[string]template.KeyChange)
		for _, c := range f.Changes {
			m[c.Key] = c
		}
		out[f.Basename] = m
	}
	return out
}
