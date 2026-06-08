package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/neko233-com/express233/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runServe(args)
	}
	switch args[0] {
	case "run", "serve":
		return runServe(args[1:])
	case "start":
		return runStart(args[1:])
	case "stop":
		return runStop(args[1:])
	case "restart":
		return runRestart(args[1:])
	case "enable-autostart":
		return runEnableAutostart(args[1:])
	case "disable-autostart":
		return runDisableAutostart(args[1:])
	case "autostart-status":
		return runAutostartStatus(args[1:])
	case "status":
		return runStatus(args[1:])
	case "port":
		return runPort(args[1:])
	case "set-port":
		return runSetPort(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "reload-config":
		return runReloadConfig(args[1:])
	case "backup-config":
		return runBackupConfig(args[1:])
	case "restore-config":
		return runRestoreConfig(args[1:])
	case "reset-root-password":
		return runResetRootPassword(args[1:])
	case "version":
		fmt.Println(version.String("express233-server"))
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("express233-server serve", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	showVer := fs.Bool("version", false, "print version and exit")
	addr := &stringFlag{}
	data := &stringFlag{}
	fs.Var(addr, "addr", "listen address")
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVer {
		fmt.Println(version.String("express233-server"))
		return nil
	}

	dataDir := resolveDataDir(data.Value(), data.IsSet())
	listen, err := resolveListenAddr(dataDir, addr.Value(), addr.IsSet())
	if err != nil {
		return err
	}
	listen = normalizeListenAddr(listen)
	return serve(listen, dataDir)
}

func runStart(args []string) error {
	fs := flag.NewFlagSet("express233-server start", flag.ContinueOnError)
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
	if status, err := detectAutostartStatus(dataDir); err == nil && status.Enabled {
		if addr.IsSet() || data.IsSet() {
			return fmt.Errorf("autostart is enabled via %s; rerun enable-autostart to change addr/data", status.Backend)
		}
		if err := startManagedAutostart(dataDir); err != nil {
			return err
		}
		fmt.Printf("started express233-server via %s url=%s addr=%s data=%s\n", status.Backend, browserURL(listen), listen, dataDir)
		return nil
	}
	if st, _, ok, err := loadRuntimeState(dataDir); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("express233-server already running url=%s addr=%s data=%s", browserURL(st.Addr), st.Addr, st.DataDir)
	}
	if err := saveRuntimeConfig(dataDir, runtimeConfig{Addr: listen}); err != nil {
		return err
	}
	state, err := startDetachedServer(dataDir, listen)
	if err != nil {
		return err
	}
	fmt.Printf("started express233-server pid=%d url=%s data=%s\n", state.PID, browserURL(state.Addr), state.DataDir)
	return nil
}

func runStop(args []string) error {
	fs := flag.NewFlagSet("express233-server stop", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	if status, err := detectAutostartStatus(dataDir); err == nil && status.Enabled {
		if err := stopManagedAutostart(dataDir); err != nil {
			return err
		}
		fmt.Printf("stopped express233-server via %s\n", status.Backend)
		return nil
	}
	st, path, ok, err := loadRuntimeState(dataDir)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("express233-server is not running")
		return nil
	}
	if err := stopServer(st); err != nil {
		return err
	}
	_ = os.Remove(path)
	_ = os.Remove(runtimePIDPath(dataDir))
	fmt.Printf("stopped express233-server pid=%d\n", st.PID)
	return nil
}

func runRestart(args []string) error {
	fs := flag.NewFlagSet("express233-server restart", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	addr := &stringFlag{}
	data := &stringFlag{}
	fs.Var(addr, "addr", "listen address")
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	if status, err := detectAutostartStatus(dataDir); err == nil && status.Enabled {
		if addr.IsSet() || data.IsSet() {
			return fmt.Errorf("autostart is enabled via %s; rerun enable-autostart to change addr/data", status.Backend)
		}
		if err := restartManagedAutostart(dataDir); err != nil {
			return err
		}
		fmt.Printf("restarted express233-server via %s\n", status.Backend)
		return nil
	}
	listen, err := resolveListenAddr(dataDir, addr.Value(), addr.IsSet())
	if err != nil {
		return err
	}
	listen = normalizeListenAddr(listen)
	if st, _, ok, err := loadRuntimeState(dataDir); err != nil {
		return err
	} else if ok {
		if err := stopServer(st); err != nil {
			return err
		}
	}
	if err := saveRuntimeConfig(dataDir, runtimeConfig{Addr: listen}); err != nil {
		return err
	}
	state, err := startDetachedServer(dataDir, listen)
	if err != nil {
		return err
	}
	fmt.Printf("restarted express233-server pid=%d url=%s\n", state.PID, browserURL(state.Addr))
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("express233-server status", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	configured, err := resolveListenAddr(dataDir, "", false)
	if err != nil {
		return err
	}
	st, _, ok, err := loadRuntimeState(dataDir)
	if err != nil {
		return err
	}
	if ok {
		fmt.Printf("status=running\npid=%d\naddr=%s\nurl=%s\ndata=%s\nconfig=%s\ndefault_port=%s\n",
			st.PID, st.Addr, browserURL(st.Addr), st.DataDir, serverRuntimeConfigPath(dataDir), defaultPort())
		if auto, err := detectAutostartStatus(dataDir); err == nil {
			fmt.Printf("autostart_backend=%s\nautostart_enabled=%t\nautostart_active=%t\n", auto.Backend, auto.Enabled, auto.Active)
		}
		return nil
	}
	configured = normalizeListenAddr(configured)
	fmt.Printf("status=stopped\nconfigured_addr=%s\nurl=%s\ndata=%s\nconfig=%s\ndefault_port=%s\n",
		configured, browserURL(configured), dataDir, serverRuntimeConfigPath(dataDir), defaultPort())
	if auto, err := detectAutostartStatus(dataDir); err == nil {
		fmt.Printf("autostart_backend=%s\nautostart_enabled=%t\nautostart_active=%t\n", auto.Backend, auto.Enabled, auto.Active)
	}
	return nil
}

func runPort(args []string) error {
	fs := flag.NewFlagSet("express233-server port", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	listen, err := resolveListenAddr(dataDir, "", false)
	if err != nil {
		return err
	}
	listen = normalizeListenAddr(listen)
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return err
	}
	fmt.Printf("default_port=%s\nconfigured_addr=%s\nconfigured_port=%s\n", defaultPort(), listen, port)
	return nil
}

func runSetPort(args []string) error {
	fs := flag.NewFlagSet("express233-server set-port", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	restart := fs.Bool("restart", true, "restart running server after updating port")
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: express233-server set-port [--data DIR] [--restart=true] <port|host:port>")
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	current, err := resolveListenAddr(dataDir, "", false)
	if err != nil {
		return err
	}
	listen, err := normalizePortValue(current, fs.Arg(0))
	if err != nil {
		return err
	}
	if err := saveRuntimeConfig(dataDir, runtimeConfig{Addr: listen}); err != nil {
		return err
	}
	fmt.Printf("saved configured_addr=%s\n", listen)
	if !*restart {
		return nil
	}
	if st, _, ok, err := loadRuntimeState(dataDir); err != nil {
		return err
	} else if ok {
		if err := stopServer(st); err != nil {
			return err
		}
		state, err := startDetachedServer(dataDir, listen)
		if err != nil {
			return err
		}
		fmt.Printf("restarted express233-server pid=%d url=%s\n", state.PID, browserURL(state.Addr))
	}
	return nil
}

func runReloadConfig(args []string) error {
	fs := flag.NewFlagSet("express233-server reload-config", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	if err := validateServerConfig(dataDir); err != nil {
		return err
	}
	st, _, ok, err := loadRuntimeState(dataDir)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("config is valid; server not running, reload skipped")
		return nil
	}
	if err := postControl(st, "/__admin/reload-config"); err != nil {
		return err
	}
	fmt.Println("reloaded server.yaml from disk")
	return nil
}

func runBackupConfig(args []string) error {
	fs := flag.NewFlagSet("express233-server backup-config", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	backup, err := backupServerConfig(dataDir)
	if err != nil {
		return err
	}
	fmt.Printf("backup created at %s\n", backup)
	return nil
}

func runRestoreConfig(args []string) error {
	fs := flag.NewFlagSet("express233-server restore-config", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	fromDefault := fs.Bool("default", false, "restore default example config instead of latest backup")
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	restored, err := restoreServerConfig(dataDir, *fromDefault)
	if err != nil {
		return err
	}
	fmt.Printf("restored server config from %s\n", restored)
	if st, _, ok, err := loadRuntimeState(dataDir); err != nil {
		return err
	} else if ok {
		if err := postControl(st, "/__admin/reload-config"); err != nil {
			return err
		}
		fmt.Println("reloaded restored config")
	}
	return nil
}

func runResetRootPassword(args []string) error {
	fs := flag.NewFlagSet("express233-server reset-root-password", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	password := fs.String("password", "", "new root password")
	fs.Var(data, "data", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*password) == "" {
		return fmt.Errorf("--password is required")
	}
	dataDir := resolveDataDir(data.Value(), data.IsSet())
	if err := resetRootPassword(dataDir, *password); err != nil {
		return err
	}
	fmt.Println("root password updated")
	return nil
}

func printUsage() {
	name := filepath.Base(os.Args[0])
	fmt.Printf(`%s commands:
  serve | run            foreground server
  start                  background server
  stop                   stop running server
  restart                restart background server
	enable-autostart       install native boot autostart
	disable-autostart      remove native boot autostart
	autostart-status       show native boot autostart status
  status                 show pid/addr/data dir
  port                   show configured/default port
  set-port <port>        update configured port/address
	update                 self-update binary and restart server
  reload-config          validate and hot reload server.yaml
  backup-config          save server.yaml backup
  restore-config         restore latest backup or --default
  reset-root-password    force reset root password from CLI
  version                print version

Examples:
  %s start
	%s enable-autostart
  %s set-port 32380
	%s update
  %s reload-config
  %s reset-root-password --password 'new-secret'
`, name, name, name, name, name, name, name)
}

type stringFlag struct {
	value string
	set   bool
}

func (f *stringFlag) String() string { return f.value }

func (f *stringFlag) Set(v string) error {
	f.value = v
	f.set = true
	return nil
}

func (f *stringFlag) Value() string { return f.value }

func (f *stringFlag) IsSet() bool { return f.set }
