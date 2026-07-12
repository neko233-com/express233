package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// AgentOptions configures a pull-mode node that reconciles desired state from
// the express233 control plane. Credentials are sent only in headers.
type AgentOptions struct {
	ServerURL   string
	Project     string
	ServerID    string
	Token       string
	Username    string
	Password    string
	RootDir     string
	Runner      string
	Role        string
	Environment string
	Labels      []string
	OS          string
	Arch        string
	Interval    time.Duration
	Once        bool
}

type agentState struct {
	CurrentVersion    string `json:"current_version"`
	AppliedGeneration int64  `json:"applied_generation"`
}

type agentHeartbeat struct {
	Project                  string   `json:"project"`
	ServerID                 string   `json:"server_id"`
	Role                     string   `json:"role,omitempty"`
	Environment              string   `json:"environment,omitempty"`
	Labels                   []string `json:"labels,omitempty"`
	OS                       string   `json:"os"`
	Arch                     string   `json:"arch"`
	CurrentVersion           string   `json:"current_version,omitempty"`
	AppliedGeneration        int64    `json:"applied_generation"`
	Status                   string   `json:"status"`
	ErrorCode                string   `json:"error_code,omitempty"`
	HeartbeatIntervalSeconds int      `json:"heartbeat_interval_seconds"`
}

type agentHeartbeatResponse struct {
	Desired struct {
		Version     string `json:"version"`
		Generation  int64  `json:"generation"`
		NeedsDeploy bool   `json:"needs_deploy"`
	} `json:"desired"`
}

var agentHTTPClient = &http.Client{Timeout: 20 * time.Second}
var agentRunDeployment = runAgentDeployment

// RunAgent heartbeats, reconciles at most one desired generation per cycle,
// and waits until the next interval after any failure (no hidden tight retry).
func RunAgent(ctx context.Context, opts AgentOptions) error {
	opts = mergeAgentOptions(opts)
	if err := validateAgentOptions(opts); err != nil {
		return err
	}
	for {
		err := runAgentCycle(ctx, opts)
		if opts.Once {
			return err
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "agent cycle failed; waiting until next heartbeat")
		}
		if ctx.Err() != nil {
			return nil
		}
		timer := time.NewTimer(agentJitteredInterval(opts.Interval, opts.ServerID))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func mergeAgentOptions(opts AgentOptions) AgentOptions {
	pullOpts := MergePullOptions(PullOptions{
		ServerURL: opts.ServerURL, Project: opts.Project, ServerID: opts.ServerID,
		Token: opts.Token, Username: opts.Username, Password: opts.Password,
	})
	opts.ServerURL, opts.Project, opts.ServerID = pullOpts.ServerURL, pullOpts.Project, pullOpts.ServerID
	opts.Token, opts.Username, opts.Password = pullOpts.Token, pullOpts.Username, pullOpts.Password
	if opts.OS == "" {
		opts.OS = runtime.GOOS
	}
	if opts.Arch == "" {
		opts.Arch = runtime.GOARCH
	}
	if opts.Interval == 0 {
		opts.Interval = time.Minute
	}
	if opts.Runner == "" {
		if runtime.GOOS == "windows" {
			opts.Runner = "safe-deploy.ps1"
		} else {
			opts.Runner = "safe-deploy.sh"
		}
	}
	if opts.RootDir == "" {
		opts.RootDir = "/opt/game-servers"
		if runtime.GOOS == "windows" {
			opts.RootDir = `C:\express233\game-servers`
		}
	}
	return opts
}

func validateAgentOptions(opts AgentOptions) error {
	if opts.ServerURL == "" || opts.Project == "" || opts.ServerID == "" {
		return fmt.Errorf("agent requires --server, --project, and --server-id")
	}
	if opts.Token == "" && (opts.Username == "" || opts.Password == "") {
		return fmt.Errorf("agent requires --token or --username/--password")
	}
	if opts.Interval < 10*time.Second || opts.Interval > time.Hour {
		return fmt.Errorf("agent interval must be between 10s and 1h")
	}
	parsed, err := url.ParseRequestURI(opts.ServerURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("invalid server URL")
	}
	return nil
}

func runAgentCycle(ctx context.Context, opts AgentOptions) error {
	statePath := agentStatePath(opts.RootDir, opts.ServerID)
	state, err := loadAgentState(statePath)
	if err != nil {
		return fmt.Errorf("load agent state: %w", err)
	}
	response, err := sendAgentHeartbeat(ctx, opts, state, "ok", "")
	if err != nil {
		return err
	}
	if !response.Desired.NeedsDeploy || response.Desired.Version == "" {
		return nil
	}
	// Mark the transition before executing the runner. Operators can therefore
	// tell an in-progress safe swap from an offline node without exposing any
	// deployment credentials in the heartbeat payload.
	_, _ = sendAgentHeartbeat(ctx, opts, state, "deploying", "")
	if err := agentRunDeployment(ctx, opts, response.Desired.Version); err != nil {
		_, _ = sendAgentHeartbeat(ctx, opts, state, "error", "deployment_failed")
		return fmt.Errorf("deployment runner failed")
	}
	nextState := state
	nextState.CurrentVersion = response.Desired.Version
	nextState.AppliedGeneration = response.Desired.Generation
	if err := saveAgentState(statePath, nextState); err != nil {
		_, _ = sendAgentHeartbeat(ctx, opts, state, "error", "state_persist_failed")
		return fmt.Errorf("persist applied generation: %w", err)
	}
	if _, err := sendAgentHeartbeat(ctx, opts, nextState, "ok", ""); err != nil {
		return fmt.Errorf("deployment succeeded but acknowledgement failed: %w", err)
	}
	fmt.Printf("agent converged server_id=%s version=%s generation=%d\n", opts.ServerID, nextState.CurrentVersion, nextState.AppliedGeneration)
	return nil
}

func sendAgentHeartbeat(ctx context.Context, opts AgentOptions, state agentState, status, errorCode string) (*agentHeartbeatResponse, error) {
	body, err := json.Marshal(agentHeartbeat{
		Project: opts.Project, ServerID: opts.ServerID, Role: opts.Role, Environment: opts.Environment,
		Labels: opts.Labels, OS: opts.OS, Arch: opts.Arch, CurrentVersion: state.CurrentVersion,
		AppliedGeneration: state.AppliedGeneration, Status: status, ErrorCode: errorCode,
		HeartbeatIntervalSeconds: int(opts.Interval.Seconds()),
	})
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(strings.TrimRight(opts.ServerURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid server URL")
	}
	base.Path = "/api/agent/nodes/heartbeat"
	base.RawQuery = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if opts.Token != "" {
		req.Header.Set("X-Express233-Token", opts.Token)
	} else {
		req.SetBasicAuth(opts.Username, opts.Password)
	}
	resp, err := agentHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("heartbeat request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.CopyN(io.Discard, resp.Body, 4096)
		return nil, fmt.Errorf("heartbeat rejected with HTTP %d", resp.StatusCode)
	}
	var result agentHeartbeatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid heartbeat response")
	}
	return &result, nil
}

func runAgentDeployment(ctx context.Context, opts AgentOptions, version string) error {
	args := []string{"--backup", "--server-id", opts.ServerID, "--version", version, "--root", opts.RootDir}
	var command *exec.Cmd
	if strings.EqualFold(filepath.Ext(opts.Runner), ".ps1") {
		shell := "pwsh"
		if _, err := exec.LookPath(shell); err != nil {
			shell = "powershell"
		}
		command = exec.CommandContext(ctx, shell, append([]string{"-NoProfile", "-NonInteractive", "-File", opts.Runner}, args...)...)
	} else {
		command = exec.CommandContext(ctx, opts.Runner, args...)
	}
	// The runner consumes secrets from its process environment rather than
	// command-line flags. Command lines are commonly visible to other local
	// users and tend to be copied into diagnostics; the runner must never print
	// these values.
	command.Env = append(os.Environ(),
		"EXPRESS233_SERVER="+opts.ServerURL,
		"EXPRESS233_PROJECT="+opts.Project,
		"EXPRESS233_SERVER_ID="+opts.ServerID,
		"EXPRESS233_TOKEN="+opts.Token,
		"EXPRESS233_USERNAME="+opts.Username,
		"EXPRESS233_PASSWORD="+opts.Password,
		"EXPRESS233_TAGS="+strings.Join(opts.Labels, ","),
	)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}

func agentStatePath(root, serverID string) string {
	safeID := strings.NewReplacer("/", "_", `\`, "_", ":", "_").Replace(serverID)
	return filepath.Join(root, ".express233-agent", safeID+".json")
}

func loadAgentState(path string) (agentState, error) {
	var state agentState
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func saveAgentState(path string, state agentState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func agentJitteredInterval(interval time.Duration, serverID string) time.Duration {
	var hash uint32 = 2166136261
	for _, b := range []byte(serverID) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	// Stable -10%..+10% jitter prevents synchronized cluster heartbeats.
	percent := int(hash%21) - 10
	return interval + time.Duration(percent)*interval/100
}
