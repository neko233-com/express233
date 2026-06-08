package api

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/neko233-com/express233/internal/version"
)

type systemUpdateStatus struct {
	Running        bool   `json:"running"`
	StartedAt      string `json:"started_at,omitempty"`
	FinishedAt     string `json:"finished_at,omitempty"`
	RequestedBy    string `json:"requested_by,omitempty"`
	TargetVersion  string `json:"target_version,omitempty"`
	CurrentVersion string `json:"current_version"`
	CurrentCommit  string `json:"current_commit"`
	OK             bool   `json:"ok"`
	Error          string `json:"error,omitempty"`
	Output         string `json:"output,omitempty"`
}

func (s *Server) handleSystemUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireSystemRoot(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.currentUpdateStatus())
}

func (s *Server) handleSystemUpdate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.currentSession(r)
	if !ok || sess.Username != "root" {
		errJSON(w, http.StatusForbidden, "system root required")
		return
	}
	target := r.URL.Query().Get("version")
	if target == "" {
		target = "latest"
	}

	s.updateMu.Lock()
	if s.updateStatus.Running {
		s.updateMu.Unlock()
		errJSON(w, http.StatusConflict, "system update already running")
		return
	}
	st := systemUpdateStatus{
		Running:        true,
		StartedAt:      time.Now().Format(time.RFC3339),
		RequestedBy:    sess.Username,
		TargetVersion:  target,
		CurrentVersion: version.Version,
		CurrentCommit:  version.Commit,
	}
	s.updateStatus = st
	s.updateMu.Unlock()

	dataDir := s.Store.DataDir()
	s.auditSession(r, "system.update.start", "version="+target)
	go s.runSystemUpdate(target, dataDir, sess.Username)

	writeJSON(w, http.StatusAccepted, s.currentUpdateStatus())
}

func (s *Server) runSystemUpdate(target, dataDir, requestedBy string) {
	if s.updateRunner != nil {
		output, err := s.updateRunner(target, dataDir)
		s.finishSystemUpdate(err == nil, requestedBy, target, output, err)
		return
	}
	output, err := runSelfUpdate(target, dataDir)
	s.finishSystemUpdate(err == nil, requestedBy, target, output, err)
}

func runSelfUpdate(target, dataDir string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	args := []string{"update", "--version", target, "--data", dataDir}
	cmd := exec.Command(exe, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	return out.String(), err
}

func (s *Server) finishSystemUpdate(ok bool, requestedBy, target, output string, err error) {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	st := s.updateStatus
	st.Running = false
	st.FinishedAt = time.Now().Format(time.RFC3339)
	st.RequestedBy = requestedBy
	st.TargetVersion = target
	st.CurrentVersion = version.Version
	st.CurrentCommit = version.Commit
	st.OK = ok
	st.Output = strings.TrimSpace(output)
	if err != nil {
		st.Error = err.Error()
	}
	s.updateStatus = st
}

func (s *Server) currentUpdateStatus() systemUpdateStatus {
	s.updateMu.RLock()
	st := s.updateStatus
	s.updateMu.RUnlock()
	st.CurrentVersion = version.Version
	st.CurrentCommit = version.Commit
	return st
}

func (s *Server) requireSystemRoot(w http.ResponseWriter, r *http.Request) bool {
	sess, ok := s.currentSession(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "login required")
		return false
	}
	if sess.Username != "root" {
		errJSON(w, http.StatusForbidden, "system root required")
		return false
	}
	return true
}
