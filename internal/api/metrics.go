package api

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type serverMetrics struct {
	pullTotal       atomic.Uint64
	pullErrors      atomic.Uint64
	previewTotal    atomic.Uint64
	loginTotal      atomic.Uint64
	publishTotal    atomic.Uint64
}

var metrics serverMetrics

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	metricCounter(w, "express233_pull_total", "Successful pull requests", metrics.pullTotal.Load())
	metricCounter(w, "express233_pull_errors_total", "Failed pull requests", metrics.pullErrors.Load())
	metricCounter(w, "express233_preview_total", "Pull preview requests", metrics.previewTotal.Load())
	metricCounter(w, "express233_login_total", "Login attempts", metrics.loginTotal.Load())
	metricCounter(w, "express233_publish_total", "Version publish actions", metrics.publishTotal.Load())
}

func metricCounter(w http.ResponseWriter, name, help string, value uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	_, _ = fmt.Fprintf(w, "# TYPE %s counter\n", name)
	_, _ = fmt.Fprintf(w, "%s %d\n", name, value)
}
