package template

import "strings"

// RenderHookTemplate 替换 post_hook 等字段中的 {{KEY}} / ${KEY} 占位符。
func RenderHookTemplate(s string, vars map[string]string) string {
	if s == "" || len(vars) == 0 {
		return s
	}
	out := s
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
		out = strings.ReplaceAll(out, "${"+k+"}", v)
	}
	return out
}

// HookTemplateVars 构建拉取/后处理常用变量。
func HookTemplateVars(project, version, serverID string, extra map[string]string) map[string]string {
	vars := map[string]string{
		"PROJECT":            project,
		"VERSION":            version,
		"SERVER_ID":          serverID,
		"EXPRESS233_PROJECT": project,
		"EXPRESS233_VERSION": version,
		"EXPRESS233_SERVER_ID": serverID,
	}
	for k, v := range extra {
		vars[k] = v
	}
	return vars
}
