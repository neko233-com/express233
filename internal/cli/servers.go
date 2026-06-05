package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// RunListServers 列出中央 server.yaml 中配置的 server_id。
func RunListServers(serverURL, token string) error {
	u, err := url.Parse(serverURL)
	if err != nil {
		return err
	}
	u.Path = "/api/pull/server-ids"
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("list server-ids %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		ServerIDs []string `json:"server_ids"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return err
	}
	if os.Getenv("EXPRESS233_JSON") == "1" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	for _, id := range out.ServerIDs {
		fmt.Println(id)
	}
	return nil
}
