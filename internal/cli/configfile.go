package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UserConfig ~/.express233/config.yaml
type UserConfig struct {
	Server  string `yaml:"server"`
	Token   string `yaml:"token"`
	Project string `yaml:"project"`
	Dest    string `yaml:"default_dest"`
}

// ConfigPath 返回配置文件路径。
func ConfigPath() (string, error) {
	return configPath()
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".express233", "config.yaml"), nil
}

// LoadUserConfig 读取用户配置（不存在则空配置）。
func LoadUserConfig() (UserConfig, error) {
	var c UserConfig
	path, err := configPath()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return c, nil
}

// SaveUserConfig 写入用户配置。
func SaveUserConfig(c UserConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// MergePullOptions 用配置文件填充 CLI 未指定的字段。
func MergePullOptions(opts PullOptions) PullOptions {
	cfg, err := LoadUserConfig()
	if err != nil {
		return opts
	}
	if opts.ServerURL == "" {
		opts.ServerURL = cfg.Server
	}
	if opts.Token == "" {
		opts.Token = cfg.Token
	}
	if opts.Project == "" {
		opts.Project = cfg.Project
	}
	if opts.DestDir == "" || opts.DestDir == "." {
		if cfg.Dest != "" {
			opts.DestDir = cfg.Dest
		}
	}
	return opts
}

// PrintConfig 打印当前配置。
func PrintConfig() error {
	cfg, err := LoadUserConfig()
	if err != nil {
		return err
	}
	path, _ := configPath()
	fmt.Printf("# %s\n", path)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}
