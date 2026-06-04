package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// RunDoctor 检查中央服务、token、server_id 配置是否可用。
func RunDoctor(serverURL, token, project, serverID string) error {
	opts := MergePullOptions(PullOptions{
		ServerURL: serverURL,
		Token:     token,
		Project:   project,
		ServerID:  serverID,
	})
	var fails int

	check := func(name string, err error) {
		if err != nil {
			fmt.Printf("  [FAIL] %s: %v\n", name, err)
			fails++
		} else {
			fmt.Printf("  [ OK ] %s\n", name)
		}
	}

	fmt.Println("express233 doctor")

	if opts.ServerURL == "" {
		fmt.Println("  [FAIL] server URL not set (config or --server)")
		fails++
	} else {
		check("healthz", pingHealth(opts.ServerURL))
	}

	if opts.Token == "" {
		fmt.Println("  [FAIL] token not set")
		fails++
	} else if opts.ServerURL != "" {
		ids, err := fetchServerIDs(opts.ServerURL, opts.Token)
		if err != nil {
			check("pull token", err)
		} else {
			check(fmt.Sprintf("pull token (%d server_ids)", len(ids)), nil)
			if opts.ServerID != "" {
				found := false
				for _, id := range ids {
					if id == opts.ServerID {
						found = true
						break
					}
				}
				if !found {
					check("server_id in server.yaml", fmt.Errorf("%q not found", opts.ServerID))
				} else {
					check("server_id registered", nil)
				}
			}
		}
	}

	if opts.Project != "" && opts.Token != "" && opts.ServerURL != "" {
		n, err := countPublishedVersions(opts.ServerURL, opts.Project, opts.Token)
		if err != nil {
			check("published versions", err)
		} else if n == 0 {
			check("published versions", fmt.Errorf("none published yet"))
		} else {
			check(fmt.Sprintf("published versions (%d)", n), nil)
		}
	}

	if fails > 0 {
		return fmt.Errorf("%d check(s) failed", fails)
	}
	fmt.Println("all checks passed")
	return nil
}

func pingHealth(serverURL string) error {
	u, err := url.Parse(serverURL)
	if err != nil {
		return err
	}
	u.Path = "/healthz"
	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(b) != "ok" {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func fetchServerIDs(serverURL, token string) ([]string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	u.Path = "/api/pull/server-ids"
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%d %s", resp.StatusCode, b)
	}
	var out struct {
		ServerIDs []string `json:"server_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.ServerIDs, nil
}

func countPublishedVersions(serverURL, project, token string) (int, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return 0, err
	}
	u.Path = "/api/pull/versions"
	q := u.Query()
	q.Set("token", token)
	q.Set("project", project)
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("%d %s", resp.StatusCode, b)
	}
	var out struct {
		Versions []any `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return len(out.Versions), nil
}
