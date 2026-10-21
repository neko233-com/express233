package scripts

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func assertReleaseRollbackArchives(t *testing.T, root, serverID string, count int) {
	t.Helper()
	archives, err := filepath.Glob(filepath.Join(root, ".backup", serverID, "*.tar.gz"))
	if err != nil || len(archives) != count {
		t.Fatalf("release rollback archive count=%d want=%d err=%v", len(archives), count, err)
	}
	for _, archive := range archives {
		file, err := os.Open(archive)
		if err != nil {
			t.Fatal(err)
		}
		compressed, err := gzip.NewReader(file)
		if err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		reader := tar.NewReader(compressed)
		members := map[string]bool{}
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			name := strings.TrimSuffix(header.Name, "/")
			members[name] = true
			for _, excluded := range []string{"data", "logs", "run"} {
				prefix := serverID + "/" + excluded
				if name == prefix || strings.HasPrefix(name, prefix+"/") {
					t.Fatalf("release archive contains root runtime state: %s", name)
				}
			}
		}
		// Read through the gzip trailer too, checking CRC rather than only tar headers.
		if _, err := io.Copy(io.Discard, compressed); err != nil {
			t.Fatal(err)
		}
		if err := compressed.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{".env", "config/application.yaml", "config/local-secret.yaml", "assets/data/keep.txt", "assets/logs/keep.txt", "assets/run/keep.txt", "assets/" + serverID + "/data/keep.txt"} {
			if !members[serverID+"/"+required] {
				t.Fatalf("release archive lost configured/nested release content: %s", required)
			}
		}
	}
}

// Run a real executable from the instance directory so the deployment script can
// verify /proc/PID/exe without weakening its unrelated-process protection.
func TestSafeDeployFixtureProcess(t *testing.T) {
	mode := os.Getenv("EXPRESS233_SAFE_DEPLOY_TEST_PROCESS")
	if mode == "" {
		return
	}
	if mode != "ignore-term" && mode != "graceful" {
		t.Fatalf("unknown fixture process mode %q", mode)
	}
	termination := make(chan os.Signal, 1)
	signal.Notify(termination, syscall.Signal(15))
	defer signal.Stop(termination)
	if err := os.WriteFile(os.Getenv("EXPRESS233_SAFE_DEPLOY_TEST_READY"), []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range termination {
		if mode == "ignore-term" {
			continue
		}
		if err := os.WriteFile(os.Getenv("EXPRESS233_SAFE_DEPLOY_TEST_STOPPED"), []byte("stopped"), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
}

func startSafeDeployFixture(t *testing.T, finalDir, mode, stoppedMarker string) *exec.Cmd {
	t.Helper()
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(testExecutable)
	if err != nil {
		t.Fatal(err)
	}
	serverExecutable := filepath.Join(finalDir, "game-server-fixture")
	if err := os.WriteFile(serverExecutable, contents, 0o755); err != nil {
		t.Fatal(err)
	}
	readyMarker := filepath.Join(finalDir, "run", "fixture-ready")
	process := exec.Command(serverExecutable, "-test.run=^TestSafeDeployFixtureProcess$")
	process.Env = append(os.Environ(),
		"EXPRESS233_SAFE_DEPLOY_TEST_PROCESS="+mode,
		"EXPRESS233_SAFE_DEPLOY_TEST_READY="+readyMarker,
		"EXPRESS233_SAFE_DEPLOY_TEST_STOPPED="+stoppedMarker,
	)
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = process.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		_ = process.Process.Kill()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(readyMarker); err == nil {
			return process
		}
		select {
		case <-done:
			t.Fatal("fixture process exited before installing its signal handler")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture process did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

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
printf '%s' "${EXPRESS233_SKIP_BACKUP:-missing}" >"$GAME_ROOT/downstream-skip-backup"
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
	downstreamSkipBackup, err := os.ReadFile(filepath.Join(root, "downstream-skip-backup"))
	if err != nil || string(downstreamSkipBackup) != "true" {
		t.Fatalf("skip-backup policy was not propagated to restart hook: %q err=%v", downstreamSkipBackup, err)
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

	oldProcess := startSafeDeployFixture(t, finalDir, "ignore-term", "")
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
	if err := os.WriteFile(filepath.Join(finalDir, ".env"), []byte("FIXTURE_MODE=local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"assets/data", "assets/logs", "assets/run", "assets/" + serverID + "/data"} {
		if err := os.MkdirAll(filepath.Join(finalDir, relative), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(finalDir, relative, "keep.txt"), []byte("old-release-asset"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(finalDir, "legacy-wrapper", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(finalDir, "legacy-wrapper", "data", "obsolete.db"), []byte("obsolete-release-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	gracefulMarker := filepath.Join(root, "graceful-stop")
	oldProcess := startSafeDeployFixture(t, finalDir, "graceful", gracefulMarker)
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
for relative in assets/data assets/logs assets/run "assets/$EXPRESS233_SERVER_ID/data"; do
  mkdir -p "$dest/$relative"
  printf current-release-asset >"$dest/$relative/keep.txt"
done
cp "$EXPRESS233_TEST_SERVER_BINARY" "$dest/game-server-fixture"
printf artifact >"$dest/game.bin"
printf manifest >"$dest/.express233/manifest.json"
printf 'schema: new\nnew_field: default\n' >"$dest/config/application.yaml"
cat >"$dest/scripts/restart.sh" <<'SCRIPT'
#!/bin/bash
set -eu
printf '%s' "${EXPRESS233_SKIP_BACKUP:-missing}" >"$GAME_ROOT/downstream-skip-backup"
mkdir -p "$GAME_ROOT/$SERVER_ID/run"
EXPRESS233_SAFE_DEPLOY_TEST_PROCESS=graceful \
EXPRESS233_SAFE_DEPLOY_TEST_READY="$GAME_ROOT/new-process-ready" \
EXPRESS233_SAFE_DEPLOY_TEST_STOPPED="$GAME_ROOT/new-process-stopped" \
  "$GAME_ROOT/$SERVER_ID/game-server-fixture" '-test.run=^TestSafeDeployFixtureProcess$' </dev/null >/dev/null 2>&1 &
printf '%s' "$!" >"$GAME_ROOT/$SERVER_ID/run/server.pid"
SCRIPT
cat >"$dest/scripts/healthcheck.sh" <<'SCRIPT'
#!/bin/bash
pid=$(cat "$GAME_ROOT/$SERVER_ID/run/server.pid")
kill -0 "$pid"
test -f "$GAME_ROOT/new-process-ready"
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
			"EXPRESS233_TEST_SERVER_BINARY="+oldProcess.Path,
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
	assertReleaseRollbackArchives(t, root, serverID, 1)
	if !strings.Contains(firstOutput, "waiting for graceful exit") {
		t.Fatalf("missing graceful stop phase:\n%s", firstOutput)
	}
	downstreamSkipBackup, err := os.ReadFile(filepath.Join(root, "downstream-skip-backup"))
	if err != nil || string(downstreamSkipBackup) != "false" {
		t.Fatalf("backup-enabled policy was not propagated to restart hook: %q err=%v", downstreamSkipBackup, err)
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
	// A failed release must restore code/config from the release archive while
	// retaining the latest persistent data instead of restoring an online copy.
	if err := os.WriteFile(filepath.Join(finalDir, "data", "players.db"), []byte("latest-live-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	failedPullBody := strings.Replace(fakePullBody, "printf artifact >", "printf broken-artifact >", 1)
	failedPullBody = strings.Replace(failedPullBody, "schema: new", "schema: broken", 1)
	failedPullBody = strings.Replace(failedPullBody, `test -f "$GAME_ROOT/new-process-ready"`, "exit 1", 1)
	if err := os.WriteFile(fakePull, []byte(failedPullBody), 0o755); err != nil {
		t.Fatal(err)
	}
	failed := exec.Command("bash", "safe-deploy.sh", "--backup", "--server-id", serverID, "--stop-timeout", "5")
	failed.Env = append(os.Environ(), "GAME_ROOT="+root, "EXPRESS233_BIN="+fakePull,
		"EXPRESS233_TEST_SERVER_BINARY="+oldProcess.Path, "EXPRESS233_SERVER_ID="+serverID,
		"EXPRESS233_HEALTH_CHECK_TIMEOUT_SECONDS=1", "EXPRESS233_HEALTH_CHECK_POLL_INTERVAL_SECONDS=0.05")
	output, failure := failed.CombinedOutput()
	t.Cleanup(func() {
		current, err := os.ReadFile(filepath.Join(runDir, "server.pid"))
		if err != nil {
			return
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(current)))
		if err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				_ = process.Kill()
			}
		}
	})
	if failure == nil || !strings.Contains(string(output), "ROLLBACK complete") {
		t.Fatalf("failed health check must fail deployment and roll back release: err=%v\n%s", failure, output)
	}
	assertReleaseRollbackArchives(t, root, serverID, 2)
	data, err = os.ReadFile(filepath.Join(finalDir, "data", "players.db"))
	if err != nil || string(data) != "latest-live-data" {
		t.Fatalf("rollback reverted persistent data: %q err=%v", data, err)
	}
	artifact, err := os.ReadFile(filepath.Join(finalDir, "game.bin"))
	if err != nil || string(artifact) != "artifact" {
		t.Fatalf("rollback did not restore previous code: %q err=%v", artifact, err)
	}
	application, err = os.ReadFile(filepath.Join(finalDir, "config", "application.yaml"))
	if err != nil || !strings.Contains(string(application), "schema: new") || strings.Contains(string(application), "schema: broken") {
		t.Fatalf("rollback did not restore previous release config: %q err=%v", application, err)
	}
	localConfig, err = os.ReadFile(filepath.Join(finalDir, "config", "local-secret.yaml"))
	if err != nil || string(localConfig) != "local: retained\n" {
		t.Fatalf("rollback lost local release config: %q err=%v", localConfig, err)
	}
}
