package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunAgentOnceConvergesAndKeepsTokenOutOfBody(t *testing.T) {
	const fictionalToken = "fictional-agent-token"
	var heartbeats int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		heartbeats++
		if got := r.Header.Get("X-Express233-Token"); got != fictionalToken {
			t.Errorf("token header = %q", got)
		}
		var heartbeat agentHeartbeat
		if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(heartbeat)
		if strings.Contains(string(encoded), fictionalToken) {
			t.Fatal("credential leaked into heartbeat JSON")
		}
		needsDeploy := heartbeat.AppliedGeneration < 7
		_ = json.NewEncoder(w).Encode(map[string]any{
			"desired": map[string]any{"version": "7.4.2", "generation": 7, "needs_deploy": needsDeploy},
		})
	}))
	defer server.Close()

	originalRunner := agentRunDeployment
	defer func() { agentRunDeployment = originalRunner }()
	runs := 0
	agentRunDeployment = func(_ context.Context, opts AgentOptions, version string) error {
		runs++
		if opts.ServerID != "logic-fictional-21" || version != "7.4.2" {
			t.Fatalf("unexpected deployment %s@%s", opts.ServerID, version)
		}
		return nil
	}

	root := t.TempDir()
	err := RunAgent(context.Background(), AgentOptions{
		ServerURL: server.URL, Project: "fictional-game", ServerID: "logic-fictional-21",
		Token: fictionalToken, RootDir: root, Interval: 10 * time.Second, Once: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if runs != 1 || heartbeats != 3 {
		t.Fatalf("runs=%d heartbeats=%d, want 1 and 3", runs, heartbeats)
	}
	state, err := loadAgentState(agentStatePath(root, "logic-fictional-21"))
	if err != nil {
		t.Fatal(err)
	}
	if state.CurrentVersion != "7.4.2" || state.AppliedGeneration != 7 {
		t.Fatalf("state = %+v", state)
	}
}

func TestRunAgentOnceFailureDoesNotAcknowledgeGeneration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"desired": map[string]any{"version": "8.0.0", "generation": 2, "needs_deploy": true},
		})
	}))
	defer server.Close()
	originalRunner := agentRunDeployment
	defer func() { agentRunDeployment = originalRunner }()
	agentRunDeployment = func(context.Context, AgentOptions, string) error { return os.ErrPermission }
	root := t.TempDir()
	err := RunAgent(context.Background(), AgentOptions{
		ServerURL: server.URL, Project: "fictional-data", ServerID: "worker-fictional-04",
		Token: "fictional-token", RootDir: root, Interval: 10 * time.Second, Once: true,
	})
	if err == nil || strings.Contains(err.Error(), "fictional-token") {
		t.Fatalf("unexpected safe error: %v", err)
	}
	state, stateErr := loadAgentState(agentStatePath(root, "worker-fictional-04"))
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	if state.AppliedGeneration != 0 || state.CurrentVersion != "" {
		t.Fatalf("failed deployment was acknowledged: %+v", state)
	}
}

func TestAgentJitterIsStableAndBounded(t *testing.T) {
	base := time.Minute
	a := agentJitteredInterval(base, "gateway-fictional")
	b := agentJitteredInterval(base, "gateway-fictional")
	if a != b || a < 54*time.Second || a > 66*time.Second {
		t.Fatalf("jitter %s is unstable or out of bounds", a)
	}
}

func TestAgentStatePersistenceFailureIsNotAcknowledged(t *testing.T) {
	var maxApplied int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var heartbeat agentHeartbeat
		_ = json.NewDecoder(r.Body).Decode(&heartbeat)
		if heartbeat.AppliedGeneration > maxApplied {
			maxApplied = heartbeat.AppliedGeneration
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"desired": map[string]any{"version": "9.1.0", "generation": 3, "needs_deploy": true}})
	}))
	defer server.Close()
	originalRunner := agentRunDeployment
	defer func() { agentRunDeployment = originalRunner }()
	agentRunDeployment = func(context.Context, AgentOptions, string) error { return nil }
	rootFile := t.TempDir() + string(os.PathSeparator) + "not-a-directory"
	if err := os.WriteFile(rootFile, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := RunAgent(context.Background(), AgentOptions{ServerURL: server.URL, Project: "fictional", ServerID: "logic-fictional-09", Token: "fictional-token", RootDir: rootFile, Interval: 10 * time.Second, Once: true})
	if err == nil {
		t.Fatal("expected persistence failure")
	}
	if maxApplied != 0 {
		t.Fatalf("unpersisted generation was acknowledged: %d", maxApplied)
	}
}
