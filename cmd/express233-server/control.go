package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/express233/internal/api"
	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/store"
	"github.com/neko233-com/express233/internal/version"
)

const (
	defaultLogMaxSizeBytes = 10 * 1024 * 1024
	defaultLogMaxBackups   = 5
	defaultLogLevel        = "info"
	githubRepo             = "neko233-com/express233"
)

type runtimeConfig struct {
	Addr string `json:"addr"`
}

type runtimeState struct {
	PID          int    `json:"pid"`
	Addr         string `json:"addr"`
	DataDir      string `json:"data_dir"`
	ControlToken string `json:"control_token"`
	StartedAt    string `json:"started_at"`
	Version      string `json:"version"`
}

func serve(listen, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	ensureServerYAML(dataDir)
	if err := saveRuntimeConfig(dataDir, runtimeConfig{Addr: listen}); err != nil {
		return err
	}

	st, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	logger, logCloser, err := setupServerLogger(dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = logCloser() }()

	controlToken, err := randomHex(16)
	if err != nil {
		return err
	}
	state := runtimeState{
		PID:          os.Getpid(),
		Addr:         listen,
		DataDir:      dataDir,
		ControlToken: controlToken,
		StartedAt:    time.Now().Format(time.RFC3339),
		Version:      version.String("express233-server"),
	}
	if err := saveRuntimeState(dataDir, state); err != nil {
		return err
	}
	defer cleanupRuntimeState(dataDir, state.PID)

	srvAPI := api.New(st)
	mux := http.NewServeMux()
	mux.HandleFunc("/__admin/reload-config", func(w http.ResponseWriter, r *http.Request) {
		if !authorizeControlRequest(r, controlToken) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := srvAPI.ReloadAllServerYAML(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{Addr: listen}
	mux.HandleFunc("/__admin/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if !authorizeControlRequest(r, controlToken) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()
	})
	mux.Handle("/", srvAPI.Router())
	server.Handler = mux

	logger.Info("server starting", "version", version.String("express233-server"))
	if err := warnIfPortBlocked(listen); err != nil {
		return err
	}
	logger.Info("server listening", "addr", listen, "url", browserURL(listen), "data_dir", dataDir)
	if wd := api.DevWebDir(); wd != "" {
		logger.Info("static hot reload enabled", "dir", wd)
	}
	logger.Info("root account available; rotate the initial password with reset-root-password")
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		logger.Info("server stopped")
		return nil
	}
	logger.Error("server exited with error", "error", err)
	return err
}

func resolveDataDir(flagValue string, flagSet bool) string {
	if flagSet && strings.TrimSpace(flagValue) != "" {
		return flagValue
	}
	if d := os.Getenv("EXPRESS233_DATA"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".express233-server")
}

func resolveListenAddr(dataDir, flagValue string, flagSet bool) (string, error) {
	if flagSet && strings.TrimSpace(flagValue) != "" {
		return flagValue, nil
	}
	if a := os.Getenv("EXPRESS233_ADDR"); a != "" {
		return a, nil
	}
	cfg, err := loadRuntimeConfig(dataDir)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.Addr) != "" {
		return cfg.Addr, nil
	}
	return defaultListenAddr(), nil
}

func defaultListenAddr() string { return "127.0.0.1:23380" }

func defaultPort() string { return "23380" }

// normalizeListenAddr 将 :port 转为 127.0.0.1:port，避免 Windows 上仅监听 IPv6 而 127.0.0.1 被其它程序占用。
func normalizeListenAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func browserURL(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + listen
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func warnIfPortBlocked(listen string) error {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil
	}
	probe := net.JoinHostPort("127.0.0.1", port)
	conn, err := net.DialTimeout("tcp", probe, 400*time.Millisecond)
	if err != nil {
		return nil
	}
	_ = conn.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + probe + "/healthz")
	if err != nil {
		return fmt.Errorf(
			"127.0.0.1:%s 已被其他程序占用（常见: proxysss）；请关闭该程序或设置 EXPRESS233_ADDR=127.0.0.1:其他端口",
			port,
		)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok" {
		return fmt.Errorf("已有 express233-server 在 %s 运行", browserURL(listen))
	}
	return fmt.Errorf(
		"127.0.0.1:%s 已被其他程序占用（常见: proxysss）；请关闭该程序或设置 EXPRESS233_ADDR=127.0.0.1:其他端口",
		port,
	)
}

func ensureServerYAML(dataDir string) {
	path, err := serverConfigPath(dataDir)
	if err != nil {
		return
	}
	if _, err := os.Stat(path); err == nil {
		return
	}
	example := defaultServerYAML()
	_ = os.WriteFile(path, []byte(example), 0o644)
	fmt.Printf("created default server.yaml at %s\n", path)
}

func defaultServerYAML() string {
	if b, err := os.ReadFile("configs/server.yaml.example"); err == nil {
		return string(b)
	}
	return "servers: {}\n"
}

func runtimeDir(dataDir string) string {
	return filepath.Join(dataDir, "run")
}

func serverRuntimeConfigPath(dataDir string) string {
	return filepath.Join(runtimeDir(dataDir), "server-runtime.json")
}

func runtimeStatePath(dataDir string) string {
	return filepath.Join(runtimeDir(dataDir), "server-state.json")
}

func runtimePIDPath(dataDir string) string {
	return filepath.Join(runtimeDir(dataDir), "server.pid")
}

func runtimeLogPath(dataDir string) string {
	return filepath.Join(runtimeDir(dataDir), "server.log")
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info", "":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

func setupServerLogger(dataDir string) (*slog.Logger, func() error, error) {
	writer, err := newRotatingFileWriter(runtimeLogPath(dataDir), defaultLogMaxSizeBytes, defaultLogMaxBackups)
	if err != nil {
		return nil, nil, err
	}
	level := parseLogLevel(os.Getenv("EXPRESS233_LOG_LEVEL"))
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	log.SetFlags(0)
	log.SetOutput(slog.NewLogLogger(handler, level).Writer())
	return logger, writer.Close, nil
}

type rotatingFileWriter struct {
	mu         sync.Mutex
	path       string
	file       *os.File
	size       int64
	maxSize    int64
	maxBackups int
}

func newRotatingFileWriter(path string, maxSize int64, maxBackups int) (*rotatingFileWriter, error) {
	if maxSize <= 0 {
		maxSize = defaultLogMaxSizeBytes
	}
	if maxBackups <= 0 {
		maxBackups = defaultLogMaxBackups
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &rotatingFileWriter{path: path, file: file, size: info.Size(), maxSize: maxSize, maxBackups: maxBackups}, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotateIfNeeded(int64(len(p))); err != nil {
		return 0, err
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *rotatingFileWriter) rotateIfNeeded(incoming int64) error {
	if w.file == nil {
		return fmt.Errorf("log writer is closed")
	}
	if w.size+incoming <= w.maxSize {
		return nil
	}
	if err := w.file.Close(); err != nil {
		return err
	}
	for index := w.maxBackups - 1; index >= 1; index-- {
		source := fmt.Sprintf("%s.%d", w.path, index)
		target := fmt.Sprintf("%s.%d", w.path, index+1)
		if _, err := os.Stat(source); err == nil {
			_ = os.Remove(target)
			if err := os.Rename(source, target); err != nil {
				return err
			}
		}
	}
	firstBackup := w.path + ".1"
	_ = os.Remove(firstBackup)
	if _, err := os.Stat(w.path); err == nil {
		if err := os.Rename(w.path, firstBackup); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.size = 0
	return nil
}

func loadRuntimeConfig(dataDir string) (runtimeConfig, error) {
	var cfg runtimeConfig
	path := serverRuntimeConfigPath(dataDir)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(b, &cfg)
	return cfg, err
}

func saveRuntimeConfig(dataDir string, cfg runtimeConfig) error {
	if err := os.MkdirAll(runtimeDir(dataDir), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(serverRuntimeConfigPath(dataDir), append(b, '\n'), 0o644)
}

func saveRuntimeState(dataDir string, st runtimeState) error {
	if err := os.MkdirAll(runtimeDir(dataDir), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(runtimeStatePath(dataDir), append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(runtimePIDPath(dataDir), []byte(strconv.Itoa(st.PID)+"\n"), 0o644)
}

func loadRuntimeState(dataDir string) (runtimeState, string, bool, error) {
	path := runtimeStatePath(dataDir)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return runtimeState{}, path, false, nil
	}
	if err != nil {
		return runtimeState{}, path, false, err
	}
	var st runtimeState
	if err := json.Unmarshal(b, &st); err != nil {
		return runtimeState{}, path, false, err
	}
	if strings.TrimSpace(st.Addr) == "" || !healthzOK(st.Addr) {
		return st, path, false, nil
	}
	return st, path, true, nil
}

func cleanupRuntimeState(dataDir string, pid int) {
	b, err := os.ReadFile(runtimePIDPath(dataDir))
	if err == nil && strings.TrimSpace(string(b)) != strconv.Itoa(pid) {
		return
	}
	_ = os.Remove(runtimeStatePath(dataDir))
	_ = os.Remove(runtimePIDPath(dataDir))
}

func healthzOK(addr string) bool {
	client := &http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(browserURL(addr) + "/healthz")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok"
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func authorizeControlRequest(r *http.Request, token string) bool {
	if r.Method != http.MethodPost {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host != "127.0.0.1" && host != "::1" && host != "localhost" {
		return false
	}
	return r.Header.Get("X-Express233-Control-Token") == token
}

func startDetachedServer(dataDir, listen string) (*runtimeState, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(runtimeDir(dataDir), 0o755); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(runtimeLogPath(dataDir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(exe, "serve", "-data", dataDir, "-addr", listen)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st, _, ok, err := loadRuntimeState(dataDir)
		if err == nil && ok {
			return &st, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("server started but runtime state was not published in time; check %s", runtimeLogPath(dataDir))
}

func stopServer(st runtimeState) error {
	if st.ControlToken == "" {
		return fmt.Errorf("runtime control token missing")
	}
	if err := postControl(st, "/__admin/shutdown"); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !healthzOK(st.Addr) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for shutdown")
}

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("express233-server update", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	data := &stringFlag{}
	versionFlag := fs.String("version", "latest", "release version to install, for example v0.2.2 or latest")
	restart := fs.Bool("restart", true, "restart the server after updating")
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
	targetVersion, err := resolveReleaseTag(*versionFlag)
	if err != nil {
		return err
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	runningState, _, running, err := loadRuntimeState(dataDir)
	if err != nil {
		return err
	}
	if running {
		if err := stopServer(runningState); err != nil {
			return err
		}
	}
	downloaded, err := downloadReleaseBinary(targetVersion)
	if err != nil {
		return err
	}
	if err := scheduleBinarySwap(exePath, downloaded, dataDir, listen, *restart); err != nil {
		return err
	}
	fmt.Printf("scheduled update to %s from %s\n", targetVersion, downloaded)
	if *restart {
		fmt.Println("the binary will be replaced and the server will restart automatically")
	}
	return nil
}

func postControl(st runtimeState, path string) error {
	req, err := http.NewRequest(http.MethodPost, browserURL(st.Addr)+path, bytes.NewReader(nil))
	if err != nil {
		return err
	}
	req.Header.Set("X-Express233-Control-Token", st.ControlToken)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("control request failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func resolveReleaseTag(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "latest") {
		return latestReleaseTag()
	}
	if strings.HasPrefix(strings.ToLower(value), "v") {
		return value, nil
	}
	return "v" + value, nil
}

func latestReleaseTag() (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+githubRepo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "express233-server-updater")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("fetch latest release: %s", strings.TrimSpace(string(body)))
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.TagName) == "" {
		return "", fmt.Errorf("latest release tag missing")
	}
	return payload.TagName, nil
}

func releaseAssetName() string {
	name := fmt.Sprintf("express233-server-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func downloadReleaseBinary(tag string) (string, error) {
	asset := releaseAssetName()
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, tag, asset)
	tempDir, err := os.MkdirTemp("", "express233-update-")
	if err != nil {
		return "", err
	}
	path := filepath.Join(tempDir, asset)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "express233-server-updater")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", fmt.Errorf("download release binary: %s", strings.TrimSpace(string(body)))
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func scheduleBinarySwap(targetExe, downloadedPath, dataDir, listen string, restart bool) error {
	self := os.Getpid()
	scriptPath := filepath.Join(runtimeDir(dataDir), updaterScriptName())
	if err := os.MkdirAll(runtimeDir(dataDir), 0o755); err != nil {
		return err
	}
	content := updaterScriptContent(targetExe, downloadedPath, dataDir, listen, self, restart)
	mode := os.FileMode(0o700)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(scriptPath, []byte(content), mode); err != nil {
		return err
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", scriptPath)
	} else {
		cmd = exec.Command("/bin/sh", scriptPath)
	}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func updaterScriptName() string {
	if runtime.GOOS == "windows" {
		return "apply-update.cmd"
	}
	return "apply-update.sh"
}

func updaterScriptContent(targetExe, downloadedPath, dataDir, listen string, selfPID int, restart bool) string {
	if runtime.GOOS == "windows" {
		restartFlag := "0"
		if restart {
			restartFlag = "1"
		}
		return fmt.Sprintf("@echo off\r\nsetlocal\r\nset \"TARGET=%s\"\r\nset \"SOURCE=%s\"\r\nset \"DATA=%s\"\r\nset \"ADDR=%s\"\r\nset \"WAITPID=%d\"\r\nset \"RESTART=%s\"\r\nfor /L %%%%i in (1,1,30) do (\r\n  tasklist /FI \"PID eq %%WAITPID%%\" 2>NUL | find \"%%WAITPID%%\" >NUL\r\n  if errorlevel 1 goto COPY\r\n  timeout /t 1 /nobreak >NUL\r\n)\r\n:COPY\r\ncopy /Y \"%%SOURCE%%\" \"%%TARGET%%\" >NUL || exit /b 1\r\ndel /F /Q \"%%SOURCE%%\" >NUL 2>NUL\r\nif \"%%RESTART%%\"==\"1\" start \"\" \"%%TARGET%%\" start -data \"%%DATA%%\" -addr \"%%ADDR%%\"\r\ndel /F /Q \"%%~f0\" >NUL 2>NUL\r\n",
			escapeBatchValue(targetExe), escapeBatchValue(downloadedPath), escapeBatchValue(dataDir), escapeBatchValue(listen), selfPID, restartFlag)
	}
	restartCommand := ""
	if restart {
		restartCommand = fmt.Sprintf("\n\"%s\" start -data %s -addr %s >/dev/null 2>&1 &", shellEscape(targetExe), shellEscape(dataDir), shellEscape(listen))
	}
	return fmt.Sprintf("#!/bin/sh\nTARGET=%s\nSOURCE=%s\nWAITPID=%d\nfor _ in $(seq 1 30); do\n  if ! kill -0 \"$WAITPID\" 2>/dev/null; then\n    break\n  fi\n  sleep 1\ndone\ncp \"$SOURCE\" \"$TARGET\"\nchmod +x \"$TARGET\"\nrm -f \"$SOURCE\"%s\nrm -f \"$0\"\n", shellEscape(targetExe), shellEscape(downloadedPath), selfPID, restartCommand)
}

func shellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func escapeBatchValue(value string) string {
	return strings.ReplaceAll(value, "\"", "\"\"")
}

func normalizePortValue(currentAddr, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("port value is required")
	}
	if strings.Contains(value, ":") {
		return normalizeListenAddr(value), nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 || port > 65535 {
		return "", fmt.Errorf("invalid port %q", value)
	}
	host, _, err := net.SplitHostPort(normalizeListenAddr(currentAddr))
	if err != nil || strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func validateServerConfig(dataDir string) error {
	path, err := serverConfigPath(dataDir)
	if err != nil {
		return err
	}
	_, err = config.LoadServerFile(path)
	return err
}

func backupServerConfig(dataDir string) (string, error) {
	path, err := serverConfigPath(dataDir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	backupDir := filepath.Join(dataDir, "backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(backupDir, fmt.Sprintf("server-%s.yaml", time.Now().Format("20060102-150405")))
	if err := copyFile(path, target); err != nil {
		return "", err
	}
	latest := filepath.Join(backupDir, "server-latest.yaml")
	if err := copyFile(path, latest); err != nil {
		return "", err
	}
	return target, nil
}

func restoreServerConfig(dataDir string, fromDefault bool) (string, error) {
	path, err := serverConfigPath(dataDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if fromDefault {
		content := defaultServerYAML()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", err
		}
		return "default example", nil
	}
	backupDir := filepath.Join(dataDir, "backup")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return "", err
	}
	var candidates []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "server-") && strings.HasSuffix(e.Name(), ".yaml") {
			candidates = append(candidates, filepath.Join(backupDir, e.Name()))
		}
	}
	if len(candidates) == 0 {
		latest := filepath.Join(backupDir, "server-latest.yaml")
		if _, err := os.Stat(latest); err == nil {
			candidates = append(candidates, latest)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no server config backup found")
	}
	sort.Strings(candidates)
	source := candidates[len(candidates)-1]
	if err := copyFile(source, path); err != nil {
		return "", err
	}
	return source, nil
}

func resetRootPassword(dataDir, password string) error {
	st, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	users, err := st.ListUsers(1)
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Username == "root" {
			return st.UpdateUserPassword(user.ID, password)
		}
	}
	created, err := st.CreateUser(1, "root", password, store.RoleAdmin, true)
	if err != nil {
		return err
	}
	return st.UpdateUserPassword(created.ID, password)
}

func serverConfigPath(dataDir string) (string, error) {
	st, err := store.Open(dataDir)
	if err != nil {
		return "", err
	}
	defer func() { _ = st.Close() }()
	return st.ServerYAMLPath(1)
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}
