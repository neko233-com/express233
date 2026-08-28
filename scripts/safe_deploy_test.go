package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSafeDeployDefaultsToGracefulStopOnly(t *testing.T) {
	script, err := os.ReadFile("safe-deploy.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(script)
	if !strings.Contains(text, `STOP_TIMEOUT="${STOP_TIMEOUT:-180}"`) {
		t.Fatal("default graceful stop timeout must allow MySQL flush")
	}
	if !strings.Contains(text, `FORCE_KILL_AFTER_TIMEOUT="${EXPRESS233_FORCE_KILL_AFTER_TIMEOUT:-0}"`) {
		t.Fatal("SIGKILL must be opt-in, never the deployment default")
	}
	if !strings.Contains(text, "deployment aborted without swapping files") {
		t.Fatal("graceful stop timeout must abort before file swap")
	}
	if !strings.Contains(text, `HEALTH_CHECK_TIMEOUT_SECONDS="${EXPRESS233_HEALTH_CHECK_TIMEOUT_SECONDS:-30}"`) {
		t.Fatal("new releases must allow a bounded health-check readiness window")
	}
}

func TestSafeDeployWaitsForBoundedHealthReadiness(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("safe-deploy.sh executes on Linux targets")
	}
	for _, command := range []string{"bash", "flock", "rsync", "sha256sum"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skipf("%s unavailable", command)
		}
	}

	root := t.TempDir()
	serverID := "rank-main"
	fakePull := filepath.Join(root, "express233-cli")
	fakePullBody := `#!/bin/bash
set -eu
dest=""
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--dest" ]]; then dest="$2"; shift 2; else shift; fi
done
mkdir -p "$dest/scripts" "$dest/.express233"
cat >"$dest/scripts/restart.sh" <<'SCRIPT'
#!/bin/bash
set -eu
(sleep 0.2; touch "$GAME_ROOT/health-ready") &
SCRIPT
cat >"$dest/scripts/healthcheck.sh" <<'SCRIPT'
#!/bin/bash
test -f "$GAME_ROOT/health-ready"
SCRIPT
chmod +x "$dest/scripts/restart.sh" "$dest/scripts/healthcheck.sh"
`
	if err := os.WriteFile(fakePull, []byte(fakePullBody), 0o755); err != nil {
		t.Fatal(err)
	}

	command := exec.Command("bash", "safe-deploy.sh", "--server-id", serverID)
	command.Dir = "."
	command.Env = append(os.Environ(),
		"GAME_ROOT="+root,
		"EXPRESS233_BIN="+fakePull,
		"EXPRESS233_HEALTH_CHECK_TIMEOUT_SECONDS=2",
		"EXPRESS233_HEALTH_CHECK_POLL_INTERVAL_SECONDS=0.05",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("safe deploy did not wait for readiness: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "health check passed within 2s") {
		t.Fatalf("missing bounded health success output:\n%s", output)
	}
}

func TestSafeDeployTimeoutAbortsBeforeSwap(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("safe-deploy.sh executes on Linux targets")
	}
	for _, command := range []string{"bash", "flock", "sha256sum"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skipf("%s unavailable", command)
		}
	}

	root := t.TempDir()
	serverID := "111"
	finalDir := filepath.Join(root, serverID)
	runDir := filepath.Join(finalDir, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(finalDir, "old.bin"), []byte("running-release"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldProcess := exec.Command("bash", "-c", `trap '' TERM; while :; do sleep 0.1; done`)
	if err := oldProcess.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = oldProcess.Process.Kill()
		_ = oldProcess.Wait()
	})
	if err := os.WriteFile(filepath.Join(runDir, "server.pid"), []byte(strconv.Itoa(oldProcess.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}

	fakePull := filepath.Join(root, "express233-cli")
	fakePullBody := `#!/bin/bash
set -eu
dest=""
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--dest" ]]; then dest="$2"; shift 2; else shift; fi
done
mkdir -p "$dest/scripts"
printf new-release >"$dest/new.bin"
cat >"$dest/scripts/restart.sh" <<'SCRIPT'
#!/bin/bash
printf started >"$GAME_ROOT/restart-called"
SCRIPT
chmod +x "$dest/scripts/restart.sh"
`
	if err := os.WriteFile(fakePull, []byte(fakePullBody), 0o755); err != nil {
		t.Fatal(err)
	}

	command := exec.Command("bash", "safe-deploy.sh", "--server-id", serverID, "--stop-timeout", "1")
	command.Dir = "."
	command.Env = append(os.Environ(),
		"GAME_ROOT="+root,
		"EXPRESS233_BIN="+fakePull,
		"EXPRESS233_FORCE_KILL_AFTER_TIMEOUT=0",
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected graceful timeout failure:\n%s", output)
	}
	if !strings.Contains(string(output), "deployment aborted without swapping files") {
		t.Fatalf("missing safe abort message:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(finalDir, "old.bin")); err != nil {
		t.Fatalf("old release was changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(finalDir, "new.bin")); !os.IsNotExist(err) {
		t.Fatalf("new release was swapped despite timeout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "restart-called")); !os.IsNotExist(err) {
		t.Fatalf("restart was called despite timeout: %v", err)
	}
	if err := oldProcess.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("old process was killed: %v", err)
	}
}

func TestSafeDeployGracefulStopAndIdempotentReplay(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("safe-deploy.sh executes on Linux targets")
	}
	for _, command := range []string{"bash", "flock", "rsync", "sha256sum"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skipf("%s unavailable", command)
		}
	}
	root := t.TempDir()
	serverID := "111"
	finalDir := filepath.Join(root, serverID)
	runDir := filepath.Join(finalDir, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(finalDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(finalDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(finalDir, "data", "players.db"), []byte("persistent-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(finalDir, "config", "application.yaml"), []byte("schema: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(finalDir, "config", "local-secret.yaml"), []byte("local: retained\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(finalDir, "legacy-wrapper", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(finalDir, "legacy-wrapper", "data", "obsolete.db"), []byte("obsolete-release-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	gracefulMarker := filepath.Join(root, "graceful-stop")
	oldProcess := exec.Command("bash", "-c", `trap 'printf stopped >"$1"; exit 0' TERM; while :; do sleep 0.1; done`, "bash", gracefulMarker)
	if err := oldProcess.Start(); err != nil {
		t.Fatal(err)
	}
	oldDone := make(chan struct{})
	go func() {
		_ = oldProcess.Wait()
		close(oldDone)
	}()
	t.Cleanup(func() {
		_ = oldProcess.Process.Kill()
		select {
		case <-oldDone:
		case <-time.After(time.Second):
		}
	})
	if err := os.WriteFile(filepath.Join(runDir, "server.pid"), []byte(strconv.Itoa(oldProcess.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}

	fakePull := filepath.Join(root, "express233-cli")
	fakePullBody := `#!/bin/bash
set -eu
dest=""
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--dest" ]]; then dest="$2"; shift 2; else shift; fi
done
mkdir -p "$dest/scripts" "$dest/.express233" "$dest/config"
printf artifact >"$dest/game.bin"
printf manifest >"$dest/.express233/manifest.json"
printf 'schema: new\nnew_field: default\n' >"$dest/config/application.yaml"
cat >"$dest/scripts/restart.sh" <<'SCRIPT'
#!/bin/bash
set -eu
mkdir -p "$GAME_ROOT/$SERVER_ID/run"
bash -c 'trap "exit 0" TERM; while :; do sleep 0.1; done' </dev/null >/dev/null 2>&1 &
printf '%s' "$!" >"$GAME_ROOT/$SERVER_ID/run/server.pid"
SCRIPT
cat >"$dest/scripts/healthcheck.sh" <<'SCRIPT'
#!/bin/bash
pid=$(cat "$GAME_ROOT/$SERVER_ID/run/server.pid")
kill -0 "$pid"
SCRIPT
chmod +x "$dest/scripts/restart.sh" "$dest/scripts/healthcheck.sh"
`
	if err := os.WriteFile(fakePull, []byte(fakePullBody), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func() string {
		t.Helper()
		command := exec.Command("bash", "safe-deploy.sh", "--backup", "--server-id", serverID, "--stop-timeout", "5")
		command.Dir = "."
		command.Env = append(os.Environ(),
			"GAME_ROOT="+root,
			"EXPRESS233_BIN="+fakePull,
			"EXPRESS233_PROJECT=game-server-sf",
			"EXPRESS233_SERVER_ID="+serverID,
			"EXPRESS233_DEPLOYMENT_ID=42",
			"EXPRESS233_IDEMPOTENCY_KEY=release-request-001",
			"VERSION=0.0.1",
		)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("safe deploy failed: %v\n%s", err, output)
		}
		return string(output)
	}
	firstOutput := run()
	if !strings.Contains(firstOutput, "waiting for graceful exit") {
		t.Fatalf("missing graceful stop phase:\n%s", firstOutput)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(gracefulMarker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("old process did not receive SIGTERM")
		}
		time.Sleep(20 * time.Millisecond)
	}
	pidBytes, err := os.ReadFile(filepath.Join(runDir, "server.pid"))
	if err != nil {
		t.Fatal(err)
	}
	newPID, err := strconv.Atoi(string(pidBytes))
	if err != nil || newPID == oldProcess.Process.Pid {
		t.Fatalf("new pid=%q old=%d err=%v", pidBytes, oldProcess.Process.Pid, err)
	}
	data, err := os.ReadFile(filepath.Join(finalDir, "data", "players.db"))
	if err != nil || string(data) != "persistent-data" {
		t.Fatalf("persistent data lost: %q err=%v", data, err)
	}
	application, err := os.ReadFile(filepath.Join(finalDir, "config", "application.yaml"))
	if err != nil || !strings.Contains(string(application), "schema: new") || !strings.Contains(string(application), "new_field: default") {
		t.Fatalf("new config schema not activated: %q err=%v", application, err)
	}
	localConfig, err := os.ReadFile(filepath.Join(finalDir, "config", "local-secret.yaml"))
	if err != nil || string(localConfig) != "local: retained\n" {
		t.Fatalf("local config lost: %q err=%v", localConfig, err)
	}
	if _, err := os.Stat(filepath.Join(finalDir, "legacy-wrapper")); !os.IsNotExist(err) {
		t.Fatalf("legacy wrapper was retained by rsync excludes: %v", err)
	}
	stateBytes, err := os.ReadFile(filepath.Join(finalDir, ".express233", "deployment-state"))
	if err != nil || !strings.Contains(string(stateBytes), "release-request-001") || !strings.Contains(string(stateBytes), "\n42\n") {
		t.Fatalf("deployment context was not committed: %q err=%v", stateBytes, err)
	}
	t.Cleanup(func() {
		if process, findErr := os.FindProcess(newPID); findErr == nil {
			_ = process.Kill()
		}
	})
	secondOutput := run()
	if !strings.Contains(secondOutput, "idempotent no-op") {
		t.Fatalf("second run was not idempotent:\n%s", secondOutput)
	}
	secondPID, err := os.ReadFile(filepath.Join(runDir, "server.pid"))
	if err != nil || string(secondPID) != string(pidBytes) {
		t.Fatalf("idempotent run restarted process: first=%q second=%q err=%v", pidBytes, secondPID, err)
	}
}
