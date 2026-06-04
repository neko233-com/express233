package cli

import (
	"fmt"
)

// RunRollback 部署上一已发布版本（或 --to 指定版本）。
func RunRollback(opts PullOptions, toVersion string, stepsBack int) error {
	opts = MergePullOptions(opts)
	if opts.ServerURL == "" || opts.Project == "" || opts.ServerID == "" || opts.Token == "" {
		return fmt.Errorf("rollback requires server, project, server-id, token")
	}
	target := toVersion
	if target == "" {
		if stepsBack <= 0 {
			stepsBack = 1
		}
		v, err := publishedVersionByOffset(opts.ServerURL, opts.Project, opts.Token, stepsBack)
		if err != nil {
			return fmt.Errorf("resolve rollback version: %w", err)
		}
		target = v
	}
	opts.Version = target
	fmt.Printf("rollback deploy -> version %s\n", target)
	return RunPull(opts)
}

func publishedVersionByOffset(serverURL, project, token string, offset int) (string, error) {
	vers, err := fetchPublishedVersions(serverURL, project, token)
	if err != nil {
		return "", err
	}
	if offset >= len(vers) {
		return "", fmt.Errorf("only %d published version(s), cannot rollback offset %d", len(vers), offset)
	}
	return vers[offset].Version, nil
}

type versionRow struct {
	Version string `json:"version"`
}

func fetchPublishedVersions(serverURL, project, token string) ([]versionRow, error) {
	u, err := buildAPIURL(serverURL, "/api/pull/versions", map[string]string{
		"token":   token,
		"project": project,
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		Versions []versionRow `json:"versions"`
	}
	if err := getJSON(u, &out); err != nil {
		return nil, err
	}
	return out.Versions, nil
}
