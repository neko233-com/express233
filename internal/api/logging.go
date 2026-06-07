package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

const slowRequestThreshold = 2 * time.Second

func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		duration := time.Since(start)

		logger := slog.Default().With(
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", duration.Milliseconds(),
		)
		if requestID := middleware.GetReqID(r.Context()); requestID != "" {
			logger = logger.With("request_id", requestID)
		}

		switch {
		case rec.status >= http.StatusInternalServerError:
			logger.Error("http request failed")
		case duration >= slowRequestThreshold && r.URL.Path != "/healthz" && r.URL.Path != "/readyz":
			logger.Warn("http request slow")
		case rec.status >= http.StatusBadRequest:
			logger.Debug("http request rejected")
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
