package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// RunPreview 预览 server_id 在某版本下的配置变更（使用拉取 token）。
func RunPreview(serverURL, project, version, serverID, token string) error {
	cfg, _ := LoadUserConfig()
	if serverURL == "" {
		serverURL = cfg.Server
	}
	if project == "" {
		project = cfg.Project
	}
	if token == "" {
		token = cfg.Token
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return err
	}
	u.Path = "/api/pull/preview"
	q := u.Query()
	q.Set("token", token)
	q.Set("project", project)
	q.Set("server_id", serverID)
	if version != "" {
		q.Set("version", version)
	}
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("preview %d: %s", resp.StatusCode, string(body))
	}
	return printPreviewJSON(body)
}

func printPreviewJSON(body []byte) error {
	var report struct {
		Project  string `json:"project"`
		Version  string `json:"version"`
		ServerID string `json:"server_id"`
		Files    []struct {
			Basename string `json:"basename"`
			Paths    []string `json:"paths"`
			Changes  []struct {
				Key    string `json:"key"`
				Before string `json:"before"`
				After  string `json:"after"`
				Action string `json:"action"`
			} `json:"changes"`
		} `json:"files"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		return err
	}

	fmt.Printf("预览 %s / %s / server_id=%s\n", report.Project, report.Version, report.ServerID)
	for _, w := range report.Warnings {
		fmt.Printf("  ⚠ %s\n", w)
	}
	for _, f := range report.Files {
		fmt.Printf("\n[%s]", f.Basename)
		if len(f.Paths) > 0 {
			fmt.Printf(" @ %s", f.Paths[0])
		}
		fmt.Println()
		for _, c := range f.Changes {
			switch c.Action {
			case "add":
				fmt.Printf("  + %s = %s\n", c.Key, c.After)
			case "unchanged":
				fmt.Printf("  = %s = %s (无变化)\n", c.Key, c.Before)
			default:
				fmt.Printf("  ~ %s: %q -> %q\n", c.Key, c.Before, c.After)
			}
		}
	}

	if os.Getenv("EXPRESS233_PREVIEW_JSON") == "1" {
		var raw json.RawMessage = body
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(raw)
	}
	return nil
}
