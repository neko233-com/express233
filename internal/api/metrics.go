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
	fmt.Fprintf(w, "# HELP express233_pull_total Successful pull requests\n")
	fmt.Fprintf(w, "# TYPE express233_pull_total counter\n")
	fmt.Fprintf(w, "express233_pull_total %d\n", metrics.pullTotal.Load())
	fmt.Fprintf(w, "# HELP express233_pull_errors_total Failed pull requests\n")
	fmt.Fprintf(w, "# TYPE express233_pull_errors_total counter\n")
	fmt.Fprintf(w, "express233_pull_errors_total %d\n", metrics.pullErrors.Load())
	fmt.Fprintf(w, "# HELP express233_preview_total Pull preview requests\n")
	fmt.Fprintf(w, "# TYPE express233_preview_total counter\n")
	fmt.Fprintf(w, "express233_preview_total %d\n", metrics.previewTotal.Load())
	fmt.Fprintf(w, "# HELP express233_login_total Login attempts\n")
	fmt.Fprintf(w, "# TYPE express233_login_total counter\n")
	fmt.Fprintf(w, "express233_login_total %d\n", metrics.loginTotal.Load())
	fmt.Fprintf(w, "# HELP express233_publish_total Version publish actions\n")
	fmt.Fprintf(w, "# TYPE express233_publish_total counter\n")
	fmt.Fprintf(w, "express233_publish_total %d\n", metrics.publishTotal.Load())
}
