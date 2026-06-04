package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// RunDiff 对比两版本在 server_id 下的配置差异。
func RunDiff(serverURL, project, from, to, serverID, token string) error {
	opts := MergePullOptions(PullOptions{ServerURL: serverURL, Token: token, Project: project})
	rawURL, err := buildAPIURL(opts.ServerURL, "/api/pull/diff", map[string]string{
		"token":     opts.Token,
		"project":   project,
		"from":      from,
		"to":        to,
		"server_id": serverID,
	})
	if err != nil {
		return err
	}

	var report json.RawMessage
	if err := getJSON(rawURL, &report); err != nil {
		return err
	}
	if os.Getenv("EXPRESS233_JSON") == "1" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	return printDiffHuman(report)
}

func printDiffHuman(raw json.RawMessage) error {
	var report struct {
		FromVersion string `json:"from_version"`
		ToVersion   string `json:"to_version"`
		ServerID    string `json:"server_id"`
		Files       []struct {
			Basename string `json:"basename"`
			Keys     []struct {
				Key    string `json:"key"`
				From   string `json:"from"`
				To     string `json:"to"`
				Change string `json:"change"`
			} `json:"keys"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return err
	}
	fmt.Printf("diff %s -> %s  server_id=%s\n", report.FromVersion, report.ToVersion, report.ServerID)
	for _, f := range report.Files {
		fmt.Printf("\n[%s]\n", f.Basename)
		for _, k := range f.Keys {
			fmt.Printf("  %s %s: %q -> %q\n", k.Change, k.Key, k.From, k.To)
		}
	}
	return nil
}
