package api

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type serverMetrics struct {
	pullTotal            atomic.Uint64
	pullErrors           atomic.Uint64
	previewTotal         atomic.Uint64
	loginTotal           atomic.Uint64
	publishTotal         atomic.Uint64
	sshCheckTotal        atomic.Uint64
	sshCheckErrors       atomic.Uint64
	hookTriggers         atomic.Uint64
	hookMerges           atomic.Uint64
	hookDispatches       atomic.Uint64
	hookFailures         atomic.Uint64
	agentHeartbeats      atomic.Uint64
	agentHeartbeatErrors atomic.Uint64
	agentDrift           atomic.Uint64
	desiredStateChanges  atomic.Uint64
}

var metrics serverMetrics

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	metricCounter(w, "express233_pull_total", "Successful pull requests", metrics.pullTotal.Load())
	metricCounter(w, "express233_pull_errors_total", "Failed pull requests", metrics.pullErrors.Load())
	metricCounter(w, "express233_preview_total", "Pull preview requests", metrics.previewTotal.Load())
	metricCounter(w, "express233_login_total", "Login attempts", metrics.loginTotal.Load())
	metricCounter(w, "express233_publish_total", "Version publish actions", metrics.publishTotal.Load())
	metricCounter(w, "express233_ssh_check_total", "Single-attempt SSH health checks", metrics.sshCheckTotal.Load())
	metricCounter(w, "express233_ssh_check_errors_total", "Failed single-attempt SSH health checks", metrics.sshCheckErrors.Load())
	metricCounter(w, "express233_release_hook_triggers_total", "Accepted release hook triggers", metrics.hookTriggers.Load())
	metricCounter(w, "express233_release_hook_merges_total", "Release hook triggers merged into a pending debounce window", metrics.hookMerges.Load())
	metricCounter(w, "express233_release_hook_dispatches_total", "Release hook deployment tasks dispatched", metrics.hookDispatches.Load())
	metricCounter(w, "express233_release_hook_failures_total", "Release hook dispatch failures", metrics.hookFailures.Load())
	metricCounter(w, "express233_agent_heartbeats_total", "Accepted pull-agent heartbeats", metrics.agentHeartbeats.Load())
	metricCounter(w, "express233_agent_heartbeat_errors_total", "Rejected or failed pull-agent heartbeats", metrics.agentHeartbeatErrors.Load())
	metricCounter(w, "express233_agent_drift_observations_total", "Heartbeats that observed desired-state drift", metrics.agentDrift.Load())
	metricCounter(w, "express233_desired_state_changes_total", "Operator or publish-driven desired-state changes", metrics.desiredStateChanges.Load())
}

func metricCounter(w http.ResponseWriter, name, help string, value uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	_, _ = fmt.Fprintf(w, "# TYPE %s counter\n", name)
	_, _ = fmt.Fprintf(w, "%s %d\n", name, value)
}
