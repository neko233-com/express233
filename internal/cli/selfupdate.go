package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/neko233-com/express233/internal/version"
)

const (
	repo       = "neko233-com/express233"
	binaryName = "express233-cli"
)

// InstallOrSwitch 从 GitHub Release 安装或切换 CLI 版本。
func InstallOrSwitch(targetVersion string) error {
	ver := targetVersion
	if ver == "" || ver == "latest" {
		v, err := latestRelease()
		if err != nil {
			return err
		}
		ver = v
	}
	ver = strings.TrimPrefix(strings.TrimPrefix(ver, "v"), "V")

	osName := runtime.GOOS
	arch := runtime.GOARCH
	asset := fmt.Sprintf("%s-%s-%s", binaryName, osName, arch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repo, ver, asset)
	if osName == "windows" {
		url += ".exe"
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	dest, err := installPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".new"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	_ = f.Close()
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(dest)
		return os.Rename(tmp, dest)
	}
	fmt.Printf("express233-cli %s installed to %s\n", ver, dest)
	return nil
}

func installPath() (string, error) {
	if p := os.Getenv("EXPRESS233_INSTALL_DIR"); p != "" {
		name := binaryName
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		return filepath.Join(p, name), nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return exe, nil
}

func latestRelease() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return strings.TrimPrefix(body.TagName, "v"), nil
}

// PrintVersion 打印当前版本。
func PrintVersion() {
	fmt.Printf("%s (%s/%s)\n", version.String("express233-cli"), runtime.GOOS, runtime.GOARCH)
}
