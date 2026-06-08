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
	OS        string
	Arch      string
	Tags      []string
	Retries   int
	SkipHook  bool
}

// RunPull 从中央服务拉取并解压，可选执行后处理脚本。
func RunPull(opts PullOptions) error {
	opts = MergePullOptions(opts)
	if opts.DestDir == "" {
		opts.DestDir = "."
	}
	if opts.OS == "" {
		opts.OS = runtime.GOOS
	}
	if opts.Arch == "" {
		opts.Arch = runtime.GOARCH
	}
	if opts.Retries <= 0 {
		opts.Retries = 3
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
	if opts.OS != "" {
		q.Set("os", opts.OS)
	}
	if opts.Arch != "" {
		q.Set("arch", opts.Arch)
	}
	for _, tag := range opts.Tags {
		q.Add("tags", tag)
	}
	u.Path = "/api/pull"
	u.RawQuery = q.Encode()

	if err := os.MkdirAll(opts.DestDir, 0o755); err != nil {
		return err
	}
	tmp, err := downloadBundleWithRetry(u.String(), opts.Retries)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()
	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	manifest, err := pull.ExtractBundle(f, opts.DestDir)
	if err != nil {
		return err
	}

	fmt.Printf("pulled %s@%s for server_id=%s os=%s arch=%s into %s\n", manifest.Project, manifest.Version, manifest.ServerID, manifest.OS, manifest.Arch, opts.DestDir)

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

func downloadBundleWithRetry(rawURL string, retries int) (string, error) {
	var last error
	for attempt := 1; attempt <= retries; attempt++ {
		tmp, err := downloadBundle(rawURL)
		if err == nil {
			if attempt > 1 {
				fmt.Printf("download succeeded after retry %d/%d\n", attempt, retries)
			}
			return tmp, nil
		}
		last = err
		if attempt < retries {
			fmt.Printf("download failed (%v), retrying %d/%d...\n", err, attempt+1, retries)
		}
	}
	return "", last
}

func downloadBundle(rawURL string) (string, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("pull failed %d: %s", resp.StatusCode, string(b))
	}
	tmp, err := os.CreateTemp("", "express233-pull-*.tar.gz")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
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
	if runtime.GOOS != "windows" {
		_ = os.Chmod(script, 0o755)
	}
	fmt.Printf("running post_hook: %s\n", script)
	return cmd.Run()
}
