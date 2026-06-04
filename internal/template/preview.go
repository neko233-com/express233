package template

import (
	"fmt"
	"os"
	"path/filepath"
)

// KeyChange 单个键的变更预览。
type KeyChange struct {
	Key    string `json:"key"`
	Before string `json:"before"`
	After  string `json:"after"`
	Action string `json:"action"` // replace | add
}

// FilePreview 某个配置文件名（basename）的预览。
type FilePreview struct {
	Basename string      `json:"basename"`
	Paths    []string    `json:"paths"`
	Changes  []KeyChange `json:"changes"`
}

// PreviewReport server_id 在某版本下的配置变更预览。
type PreviewReport struct {
	Project       string         `json:"project"`
	Version       string         `json:"version"`
	ServerID      string         `json:"server_id"`
	Files         []FilePreview  `json:"files"`
	RenderedFiles []RenderedFile `json:"rendered_files,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
	PostHook      string         `json:"post_hook,omitempty"`
	PostHookEnv   map[string]string `json:"post_hook_env,omitempty"`
	PostHookPlan  []string       `json:"post_hook_plan,omitempty"`
}

// BuildPreview 基于版本目录与 replacements（按 basename）生成变更预览。
func BuildPreview(root string, project, version, serverID string, byFile map[string]map[string]any, postHook string, postHookEnv map[string]string) (*PreviewReport, error) {
	normalized, err := NormalizeFileOverrides(byFile)
	if err != nil {
		return nil, err
	}
	matches, err := FindConfigFilesByBasename(root)
	if err != nil {
		return nil, err
	}

	hookVars := HookTemplateVars(project, version, serverID, postHookEnv)
	report := &PreviewReport{
		Project:     project,
		Version:     version,
		ServerID:    serverID,
		PostHook:    RenderHookTemplate(postHook, hookVars),
		PostHookEnv: postHookEnv,
	}

	for base, kv := range normalized {
		paths := matches[base]
		fp := FilePreview{Basename: base, Paths: paths}
		if len(paths) == 0 {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("replacements[%s]: 版本包内不存在该配置文件，拉取时将报错", base))
			fp.Changes = previewKeysWithoutFile(treeToFlat(kv))
			report.Files = append(report.Files, fp)
			continue
		}
		if len(paths) > 1 {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("replacements[%s]: 存在 %d 个同名配置文件（违反唯一命名约束）: %v", base, len(paths), paths))
		}
		// 同名文件内容应一致；预览以第一个为准
		rel := paths[0]
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		fp.Changes = previewKeyChangesFromTree(base, data, kv)
		report.Files = append(report.Files, fp)
	}
	rendered, err := BuildRenderedFiles(root, byFile)
	if err != nil {
		return nil, err
	}
	report.RenderedFiles = rendered
	return report, nil
}

func treeToFlat(tree map[string]any) map[string]string {
	flat := FlattenScalars("", tree)
	out := make(map[string]string, len(flat))
	for k, v := range flat {
		out[k] = fmt.Sprint(v)
	}
	return out
}

func previewKeysWithoutFile(kv map[string]string) []KeyChange {
	var out []KeyChange
	for key, after := range kv {
		out = append(out, KeyChange{Key: key, Before: "", After: after, Action: "add"})
	}
	return out
}

func previewKeyChangesFromTree(basename string, data []byte, tree map[string]any) []KeyChange {
	merged, err := MergeBytes(basename, data, tree)
	if err != nil {
		return previewKeyChanges(basename, data, treeToFlat(tree))
	}
	var out []KeyChange
	for key, after := range treeToFlat(tree) {
		before, found := ReadKey(basename, data, key)
		afterMerged, _ := ReadKey(basename, merged, key)
		ch := KeyChange{Key: key, After: afterMerged}
		if !found && afterMerged == "" {
			ch.After = after
		}
		if found {
			ch.Before = before
			ch.Action = "replace"
			if before == ch.After {
				ch.Action = "unchanged"
			}
		} else {
			ch.Action = "add"
		}
		out = append(out, ch)
	}
	return out
}

func previewKeyChanges(basename string, data []byte, kv map[string]string) []KeyChange {
	var out []KeyChange
	for key, after := range kv {
		before, found := ReadKey(basename, data, key)
		ch := KeyChange{Key: key, After: after}
		if found {
			ch.Before = before
			ch.Action = "replace"
			if before == after {
				ch.Action = "unchanged"
			}
		} else {
			ch.Action = "add"
		}
		out = append(out, ch)
	}
	return out
}
