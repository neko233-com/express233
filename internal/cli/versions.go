package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// RunListVersions 列出项目已发布版本。
func RunListVersions(serverURL, project, token string) error {
	u, err := url.Parse(serverURL)
	if err != nil {
		return err
	}
	u.Path = "/api/pull/versions"
	q := u.Query()
	q.Set("token", token)
	q.Set("project", project)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("versions %d: %s", resp.StatusCode, string(body))
	}
	if os.Getenv("EXPRESS233_JSON") == "1" {
		var raw json.RawMessage = body
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(raw)
	}
	var out struct {
		Project  string `json:"project"`
		Versions []struct {
			Version     string `json:"version"`
			PublishedAt string `json:"published_at"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return err
	}
	fmt.Printf("project %s published versions:\n", out.Project)
	for _, v := range out.Versions {
		fmt.Printf("  %s  (%s)\n", v.Version, v.PublishedAt)
	}
	return nil
}
