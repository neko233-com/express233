package api

import (
	"embed"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed guide/*.md
var agentGuideFS embed.FS

type guideTopic struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Content string `json:"content,omitempty"`
}

var guideTopics = []guideTopic{
	{ID: "overview", Title: "平台与多版本模型", Summary: "版本生命周期、固定版本、自动跟随与回滚边界。"},
	{ID: "agent", Title: "Pull Agent 接入", Summary: "安全心跳、期望状态与 Linux/Windows runner 配置。"},
	{ID: "gitea", Title: "Gitea Actions 接入", Summary: "构建、上传制品、发布与 Hook 的最小流程。"},
	{ID: "github", Title: "GitHub Actions 接入", Summary: "GitHub 工作流上传制品并触发发布的最小流程。"},
	{ID: "security", Title: "安全边界", Summary: "账号、Token、IP 封禁、SSH 凭据与日志脱敏规则。"},
}

// handleAgentGuide intentionally stays public and returns only embedded,
// reviewed product documentation. It never reads configuration, credentials,
// host addresses, audit records, or filesystem paths.
func (s *Server) handleAgentGuide(w http.ResponseWriter, r *http.Request) {
	topicID := strings.TrimSpace(chi.URLParam(r, "topic"))
	if topicID == "" {
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, map[string]any{
			"title":    "express233 官方接入指南",
			"notice":   "此接口只提供脱敏官方说明；不会返回账号、Token、SSH、服务器地址、项目或运行状态。",
			"topics":   guideTopics,
			"api_path": "/api/agent/guide/{topic}",
		})
		return
	}
	for _, topic := range guideTopics {
		if topic.ID != topicID {
			continue
		}
		content, err := agentGuideFS.ReadFile("guide/" + topic.ID + ".md")
		if err != nil {
			errJSON(w, http.StatusInternalServerError, "official guide unavailable")
			return
		}
		topic.Content = string(content)
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, topic)
		return
	}
	errJSON(w, http.StatusNotFound, "official guide topic not found")
}
