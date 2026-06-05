package store

import (
	"fmt"

	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/template"
)

// PublishValidation 发布前检查结果。
type PublishValidation struct {
	OK       bool     `json:"ok"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateBeforePublish 检查版本是否可发布。
// 不包含浏览器可视化 E2E（test/visual、visual-e2e.cmd）；发布 API 亦不会触发 Playwright。
func (s *Store) ValidateBeforePublish(tenantID int64, projectName string, projectID int64, version string, sf *config.ServerFile) (*PublishValidation, error) {
	out := &PublishValidation{OK: true}
	root, err := s.VersionDir(tenantID, projectName, version)
	if err != nil {
		return nil, err
	}
	empty, err := isDirEmpty(root)
	if err != nil {
		return nil, err
	}
	if empty {
		out.OK = false
		out.Errors = append(out.Errors, "版本目录为空，请先上传内容")
	}
	if err := ValidateUniqueConfigBasenames(root); err != nil {
		out.OK = false
		out.Errors = append(out.Errors, err.Error())
	}
	if sf != nil && len(sf.Servers) > 0 {
		entries, err := s.ListConfigFileEntries(tenantID, projectName, version)
		if err != nil {
			return nil, err
		}
		basenames := make(map[string]bool)
		for _, e := range entries {
			basenames[e.Basename] = true
		}
		for sid, entry := range sf.Servers {
			for name := range entry.Replacements {
				base := template.Basename(name)
				if !basenames[base] {
					out.Warnings = append(out.Warnings,
						fmt.Sprintf("server.yaml[%s] 引用了 %q，但版本包内无此配置文件", sid, base))
				}
			}
			_ = entry.PostHook
		}
	}
	if len(out.Errors) > 0 {
		out.OK = false
	}
	return out, nil
}
