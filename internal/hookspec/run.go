package hookspec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// RunContext 执行环境变量。
type RunContext struct {
	DestDir string
	Env     map[string]string
}

// Execute 执行 post-hook.yaml 中第一个匹配当前 OS 的步骤。
func Execute(versionRoot string, ctx RunContext) error {
	sp, err := Load(versionRoot)
	if err != nil {
		return err
	}
	if sp == nil {
		return nil
	}
	goos := CurrentOS()
	for _, st := range sp.Steps {
		if st.Run == "" {
			continue
		}
		ok, err := st.matches(goos)
		if err != nil {
			return err
		}
		if ok {
			return runStep(ctx.DestDir, st, ctx.Env)
		}
	}
	return nil
}

func runStep(dest string, st Step, extraEnv map[string]string) error {
	script := filepath.Join(dest, filepath.FromSlash(st.Run))
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("post-hook script not found: %s", script)
	}
	shell := strings.ToLower(strings.TrimSpace(st.Shell))
	var cmd *exec.Cmd
	switch shell {
	case "powershell", "ps1", "pwsh":
		cmd = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	case "bash", "sh":
		cmd = exec.Command("bash", script)
	case "cmd":
		cmd = exec.Command("cmd", "/c", script)
	case "":
		if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(script), ".ps1") {
			cmd = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
		} else if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(script), ".sh") {
			cmd = exec.Command("bash", script)
		} else {
			cmd = exec.Command(script)
		}
	default:
		return fmt.Errorf("unsupported shell %q", st.Shell)
	}
	cmd.Dir = dest
	cmd.Env = os.Environ()
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runtime.GOOS != "windows" {
		_ = os.Chmod(script, 0o755)
	}
	fmt.Printf("running post-hook: %s\n", script)
	return cmd.Run()
}

// PlanLines 人类可读计划（预览用）。
func PlanLines(versionRoot string, goos string) ([]string, error) {
	sp, err := Load(versionRoot)
	if err != nil {
		return nil, err
	}
	if sp == nil {
		return nil, nil
	}
	paths, err := sp.Plan(goos)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, p := range paths {
		lines = append(lines, fmt.Sprintf("[%s] %s", goos, p))
	}
	return lines, nil
}
