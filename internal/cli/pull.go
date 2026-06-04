package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/neko233-com/express233/internal/hookspec"
	"github.com/neko233-com/express233/internal/pull"
)

// PullOptions 拉取参数。
type PullOptions struct {
	ServerURL string
	Project   string
	ServerID  string
	Token     string
	Version   string
	DestDir   string
	SkipHook  bool
}

// RunPull 从中央服务拉取并解压，可选执行后处理脚本。
func RunPull(opts PullOptions) error {
	opts = MergePullOptions(opts)
	if opts.DestDir == "" {
		opts.DestDir = "."
	}
	u, err := url.Parse(opts.ServerURL)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("token", opts.Token)
	q.Set("project", opts.Project)
	q.Set("server_id", opts.ServerID)
	if opts.Version != "" {
		q.Set("version", opts.Version)
	}
	u.Path = "/api/pull"
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull failed %d: %s", resp.StatusCode, string(b))
	}

	if err := os.MkdirAll(opts.DestDir, 0o755); err != nil {
		return err
	}
	manifest, err := pull.ExtractBundle(resp.Body, opts.DestDir)
	if err != nil {
		return err
	}

	fmt.Printf("pulled %s@%s for server_id=%s into %s\n", manifest.Project, manifest.Version, manifest.ServerID, opts.DestDir)

	if opts.SkipHook {
		return nil
	}
	if manifest.PostHookSpec != "" {
		env := hookEnvMap(manifest)
		return hookspec.Execute(opts.DestDir, hookspec.RunContext{DestDir: opts.DestDir, Env: env})
	}
	if manifest.PostHook == "" {
		return nil
	}
	return runPostHook(opts.DestDir, manifest)
}

func hookEnvMap(m *pull.Manifest) map[string]string {
	env := make(map[string]string, len(m.PostHookEnv)+3)
	for k, v := range m.PostHookEnv {
		env[k] = v
	}
	env["EXPRESS233_PROJECT"] = m.Project
	env["EXPRESS233_VERSION"] = m.Version
	env["EXPRESS233_SERVER_ID"] = m.ServerID
	return env
}

func runPostHook(dest string, m *pull.Manifest) error {
	script := filepath.Join(dest, filepath.FromSlash(m.PostHook))
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("post_hook not found: %s", script)
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(script), ".sh") {
		cmd = exec.Command("bash", script)
	} else {
		cmd = exec.Command(script)
	}
	cmd.Dir = dest
	cmd.Env = os.Environ()
	for k, v := range m.PostHookEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Env = append(cmd.Env,
		"EXPRESS233_PROJECT="+m.Project,
		"EXPRESS233_VERSION="+m.Version,
		"EXPRESS233_SERVER_ID="+m.ServerID,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runtime.GOOS == "windows" {
		// bash scripts need sh on Windows; user may use .bat
	} else {
		if err := os.Chmod(script, 0o755); err == nil {
			// ok
		}
	}
	fmt.Printf("running post_hook: %s\n", script)
	return cmd.Run()
}
