package hookspec

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultPath = ".express233/post-hook.yaml"

// Spec 版本包内后处理声明（拉取方按本机 OS 执行）。
type Spec struct {
	Version int    `yaml:"version"`
	Steps   []Step `yaml:"steps"`
}

// Step 单步执行；第一个匹配的 when 生效。无 when 的步骤在轮到时常驻匹配（else）。
type Step struct {
	When  string `yaml:"when,omitempty"`
	Else  bool   `yaml:"else,omitempty"`
	Run   string `yaml:"run"`
	Shell string `yaml:"shell,omitempty"`
}

// Load 从版本目录读取 post-hook 声明。
func Load(versionRoot string) (*Spec, error) {
	path := filepath.Join(versionRoot, filepath.FromSlash(DefaultPath))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sp Spec
	if err := yaml.Unmarshal(data, &sp); err != nil {
		return nil, fmt.Errorf("parse post-hook.yaml: %w", err)
	}
	if sp.Version == 0 {
		sp.Version = 1
	}
	return &sp, nil
}

// Plan 返回将执行的脚本相对路径（第一个匹配的 step）。
func (sp *Spec) Plan(goos string) ([]string, error) {
	if sp == nil {
		return nil, nil
	}
	for _, st := range sp.Steps {
		if st.Run == "" {
			continue
		}
		ok, err := st.matches(goos)
		if err != nil {
			return nil, err
		}
		if ok {
			return []string{st.Run}, nil
		}
	}
	return nil, nil
}

func (st *Step) matches(goos string) (bool, error) {
	if st.Else {
		return true, nil
	}
	if st.When == "" {
		return true, nil
	}
	w := strings.TrimSpace(st.When)
	// os == "windows" | os == 'linux'
	if strings.Contains(w, "==") {
		parts := strings.SplitN(w, "==", 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid when: %q", st.When)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key != "os" {
			return false, fmt.Errorf("unsupported when key %q (only os supported)", key)
		}
		return goos == val, nil
	}
	return false, fmt.Errorf("unsupported when expression: %q", st.When)
}

// CurrentOS 用于测试注入。
func CurrentOS() string {
	return runtime.GOOS
}
