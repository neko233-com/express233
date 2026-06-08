package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	systemdServiceName = "express233-server.service"
	launchdLabel       = "com.neko233.express233-server"
	windowsTaskName    = "express233-server"
)

type autostartStatus struct {
	Backend string
	Enabled bool
	Active  bool
	Detail  string
}

func runEnableAutostart(args []string) error {
	fs := flag.NewFlagSet("express233-server enable-autostart", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	addr := &stringFlag{}
	data := &stringFlag{}
	fs.Var(addr, "addr", "listen address")
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	listen, err := resolveListenAddr(dataDir, addr.Value(), addr.IsSet())
	if err != nil {
		return err
	}
	listen = normalizeListenAddr(listen)
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	if st, _, ok, err := loadRuntimeState(dataDir); err == nil && ok {
		if err := stopServer(st); err != nil {
			return err
		}
	}
	if err := saveRuntimeConfig(dataDir, runtimeConfig{Addr: listen}); err != nil {
		return err
	}
	if err := enableNativeAutostart(exePath, dataDir, listen); err != nil {
		return err
	}
	status, err := detectAutostartStatus(dataDir)
	if err != nil {
		return err
	}
	fmt.Printf("autostart enabled via %s\n", status.Backend)
	if status.Detail != "" {
		fmt.Printf("detail=%s\n", status.Detail)
	}
	return nil
}

func runDisableAutostart(args []string) error {
	fs := flag.NewFlagSet("express233-server disable-autostart", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	if err := disableNativeAutostart(dataDir); err != nil {
		return err
	}
	fmt.Println("autostart disabled")
	return nil
}

func runAutostartStatus(args []string) error {
	fs := flag.NewFlagSet("express233-server autostart-status", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	status, err := detectAutostartStatus(dataDir)
	if err != nil {
		return err
	}
	fmt.Printf("autostart_backend=%s\nautostart_enabled=%t\nautostart_active=%t\n", status.Backend, status.Enabled, status.Active)
	if status.Detail != "" {
		fmt.Printf("autostart_detail=%s\n", status.Detail)
	}
	return nil
}

func detectAutostartStatus(dataDir string) (autostartStatus, error) {
	switch runtime.GOOS {
	case "linux":
		unitPath := systemdUnitPath()
		status := autostartStatus{Backend: "systemd"}
		if _, err := os.Stat(unitPath); os.IsNotExist(err) {
			return status, nil
		} else if err != nil {
			return autostartStatus{}, err
		}
		status.Enabled = true
		status.Detail = unitPath
		status.Active = runCommand(exec.Command("systemctl", "is-active", "--quiet", systemdServiceName)) == nil
		return status, nil
	case "darwin":
		plistPath := launchdPlistPath()
		status := autostartStatus{Backend: "launchd"}
		if _, err := os.Stat(plistPath); os.IsNotExist(err) {
			return status, nil
		} else if err != nil {
			return autostartStatus{}, err
		}
		status.Enabled = true
		status.Detail = plistPath
		status.Active = runCommand(exec.Command("launchctl", "print", "system/"+launchdLabel)) == nil
		return status, nil
	case "windows":
		status := autostartStatus{Backend: "schtasks", Detail: windowsTaskName}
		if err := runCommand(exec.Command("schtasks", "/Query", "/TN", windowsTaskName)); err != nil {
			return status, nil
		}
		status.Enabled = true
		_, _, running, err := loadRuntimeState(dataDir)
		if err != nil {
			return autostartStatus{}, err
		}
		status.Active = running
		return status, nil
	default:
		return autostartStatus{Backend: runtime.GOOS}, nil
	}
}

func enableNativeAutostart(exePath, dataDir, listen string) error {
	switch runtime.GOOS {
	case "linux":
		if err := os.WriteFile(systemdUnitPath(), []byte(systemdUnitContent(exePath, dataDir, listen)), 0o644); err != nil {
			return fmt.Errorf("write systemd unit: %w", err)
		}
		if err := runCommand(exec.Command("systemctl", "daemon-reload")); err != nil {
			return err
		}
		return runCommand(exec.Command("systemctl", "enable", "--now", systemdServiceName))
	case "darwin":
		if err := os.WriteFile(launchdPlistPath(), []byte(launchdPlistContent(exePath, dataDir, listen)), 0o644); err != nil {
			return fmt.Errorf("write launchd plist: %w", err)
		}
		_ = runCommand(exec.Command("launchctl", "bootout", "system/"+launchdLabel))
		if err := runCommand(exec.Command("launchctl", "bootstrap", "system", launchdPlistPath())); err != nil {
			return err
		}
		_ = runCommand(exec.Command("launchctl", "enable", "system/"+launchdLabel))
		return runCommand(exec.Command("launchctl", "kickstart", "-k", "system/"+launchdLabel))
	case "windows":
		command := windowsTaskCommand(exePath, dataDir, listen)
		if err := runCommand(exec.Command("schtasks", "/Create", "/F", "/TN", windowsTaskName, "/SC", "ONSTART", "/RL", "HIGHEST", "/RU", "SYSTEM", "/TR", command)); err != nil {
			return err
		}
		return runCommand(exec.Command("schtasks", "/Run", "/TN", windowsTaskName))
	default:
		return fmt.Errorf("autostart unsupported on %s", runtime.GOOS)
	}
}

func disableNativeAutostart(dataDir string) error {
	status, err := detectAutostartStatus(dataDir)
	if err != nil {
		return err
	}
	if !status.Enabled {
		return nil
	}
	switch runtime.GOOS {
	case "linux":
		_ = runCommand(exec.Command("systemctl", "disable", "--now", systemdServiceName))
		if err := os.Remove(systemdUnitPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return runCommand(exec.Command("systemctl", "daemon-reload"))
	case "darwin":
		_ = runCommand(exec.Command("launchctl", "bootout", "system/"+launchdLabel))
		if err := os.Remove(launchdPlistPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	case "windows":
		_ = runCommand(exec.Command("schtasks", "/End", "/TN", windowsTaskName))
		_ = stopManagedAutostart(dataDir)
		return runCommand(exec.Command("schtasks", "/Delete", "/F", "/TN", windowsTaskName))
	default:
		return fmt.Errorf("autostart unsupported on %s", runtime.GOOS)
	}
}

func startManagedAutostart(dataDir string) error {
	status, err := detectAutostartStatus(dataDir)
	if err != nil {
		return err
	}
	if !status.Enabled {
		return errors.New("autostart is not enabled")
	}
	switch runtime.GOOS {
	case "linux":
		return runCommand(exec.Command("systemctl", "start", systemdServiceName))
	case "darwin":
		if err := runCommand(exec.Command("launchctl", "bootstrap", "system", launchdPlistPath())); err != nil && !strings.Contains(err.Error(), "already bootstrapped") {
			return err
		}
		return runCommand(exec.Command("launchctl", "kickstart", "-k", "system/"+launchdLabel))
	case "windows":
		return runCommand(exec.Command("schtasks", "/Run", "/TN", windowsTaskName))
	default:
		return fmt.Errorf("autostart unsupported on %s", runtime.GOOS)
	}
}

func stopManagedAutostart(dataDir string) error {
	status, err := detectAutostartStatus(dataDir)
	if err != nil {
		return err
	}
	if !status.Enabled {
		return errors.New("autostart is not enabled")
	}
	switch runtime.GOOS {
	case "linux":
		return runCommand(exec.Command("systemctl", "stop", systemdServiceName))
	case "darwin":
		return runCommand(exec.Command("launchctl", "bootout", "system/"+launchdLabel))
	case "windows":
		if err := runCommand(exec.Command("schtasks", "/End", "/TN", windowsTaskName)); err == nil {
			return nil
		}
		st, path, ok, err := loadRuntimeState(dataDir)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := stopServer(st); err != nil {
			return err
		}
		_ = os.Remove(path)
		_ = os.Remove(runtimePIDPath(dataDir))
		return nil
	default:
		return fmt.Errorf("autostart unsupported on %s", runtime.GOOS)
	}
}

func restartManagedAutostart(dataDir string) error {
	status, err := detectAutostartStatus(dataDir)
	if err != nil {
		return err
	}
	if !status.Enabled {
		return errors.New("autostart is not enabled")
	}
	switch runtime.GOOS {
	case "linux":
		return runCommand(exec.Command("systemctl", "restart", systemdServiceName))
	case "darwin":
		_ = runCommand(exec.Command("launchctl", "bootout", "system/"+launchdLabel))
		if err := runCommand(exec.Command("launchctl", "bootstrap", "system", launchdPlistPath())); err != nil && !strings.Contains(err.Error(), "already bootstrapped") {
			return err
		}
		return runCommand(exec.Command("launchctl", "kickstart", "-k", "system/"+launchdLabel))
	case "windows":
		if err := stopManagedAutostart(dataDir); err != nil {
			return err
		}
		return startManagedAutostart(dataDir)
	default:
		return fmt.Errorf("autostart unsupported on %s", runtime.GOOS)
	}
}

func restartStrategyCommand(dataDir, listen string) string {
	status, err := detectAutostartStatus(dataDir)
	if err == nil && status.Enabled {
		switch runtime.GOOS {
		case "linux":
			return "systemctl start " + systemdServiceName
		case "darwin":
			return "launchctl kickstart -k system/" + launchdLabel
		case "windows":
			return "schtasks /Run /TN " + windowsTaskName
		}
	}
	if runtime.GOOS == "windows" {
		return `start "" "%TARGET%" start -data "%DATA%" -addr "%ADDR%"`
	}
	return fmt.Sprintf("\"%s\" start -data %s -addr %s >/dev/null 2>&1 &", shellEscape(resolveExecutableForScript()), shellEscape(dataDir), shellEscape(listen))
}

func resolveExecutableForScript() string {
	exe, err := os.Executable()
	if err != nil {
		return "express233-server"
	}
	return exe
}

func systemdUnitPath() string {
	return filepath.Join(string(os.PathSeparator), "etc", "systemd", "system", systemdServiceName)
}

func launchdPlistPath() string {
	return filepath.Join(string(os.PathSeparator), "Library", "LaunchDaemons", launchdLabel+".plist")
}

func systemdUnitContent(exePath, dataDir, listen string) string {
	return fmt.Sprintf("[Unit]\nDescription=express233-server\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=simple\nExecStart=%s serve -data %s -addr %s\nRestart=on-failure\nRestartSec=5\nWorkingDirectory=%s\n\n[Install]\nWantedBy=multi-user.target\n", systemdEscapeArg(exePath), systemdEscapeArg(dataDir), systemdEscapeArg(listen), systemdEscapeArg(filepath.Dir(exePath)))
}

func launchdPlistContent(exePath, dataDir, listen string) string {
	return fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\">\n<dict>\n  <key>Label</key>\n  <string>%s</string>\n  <key>ProgramArguments</key>\n  <array>\n    <string>%s</string>\n    <string>serve</string>\n    <string>-data</string>\n    <string>%s</string>\n    <string>-addr</string>\n    <string>%s</string>\n  </array>\n  <key>RunAtLoad</key>\n  <true/>\n  <key>KeepAlive</key>\n  <true/>\n  <key>WorkingDirectory</key>\n  <string>%s</string>\n</dict>\n</plist>\n", launchdLabel, xmlEscape(exePath), xmlEscape(dataDir), xmlEscape(listen), xmlEscape(filepath.Dir(exePath)))
}

func windowsTaskCommand(exePath, dataDir, listen string) string {
	return fmt.Sprintf("\"%s\" serve -data \"%s\" -addr \"%s\"", exePath, dataDir, listen)
}

func systemdEscapeArg(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, " ", "\\x20")
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return replacer.Replace(value)
}

func runCommand(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s: %s", strings.Join(cmd.Args, " "), msg)
	}
	return nil
}
