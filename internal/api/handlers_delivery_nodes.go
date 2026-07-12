package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

type deliveryHeartbeatRequest struct {
	Project                  string   `json:"project"`
	ServerID                 string   `json:"server_id"`
	Role                     string   `json:"role"`
	Environment              string   `json:"environment"`
	Labels                   []string `json:"labels"`
	OS                       string   `json:"os"`
	Arch                     string   `json:"arch"`
	CurrentVersion           string   `json:"current_version"`
	AppliedGeneration        int64    `json:"applied_generation"`
	Status                   string   `json:"status"`
	ErrorCode                string   `json:"error_code"`
	HeartbeatIntervalSeconds int      `json:"heartbeat_interval_seconds"`
}

type deliveryDesiredResponse struct {
	Version     string `json:"version"`
	Generation  int64  `json:"generation"`
	NeedsDeploy bool   `json:"needs_deploy"`
}

type deliveryNodeView struct {
	ID                string   `json:"id"`
	DeliveryMode      string   `json:"delivery_mode"`
	ServerID          string   `json:"server_id"`
	Role              string   `json:"role,omitempty"`
	Environment       string   `json:"environment,omitempty"`
	Labels            []string `json:"labels"`
	OS                string   `json:"os,omitempty"`
	Arch              string   `json:"arch,omitempty"`
	CurrentVersion    string   `json:"current_version,omitempty"`
	DesiredVersion    string   `json:"desired_version,omitempty"`
	DesiredGeneration int64    `json:"desired_generation"`
	AppliedGeneration int64    `json:"applied_generation"`
	AutoFollow        bool     `json:"auto_follow"`
	Status            string   `json:"status"`
	LastError         string   `json:"last_error,omitempty"`
	LastSeenAt        string   `json:"last_seen_at,omitempty"`
	Online            bool     `json:"online"`
	Drift             bool     `json:"drift"`
	HostID            int64    `json:"host_id,omitempty"`
	HostName          string   `json:"host_name,omitempty"`
}

func (s *Server) handleDeliveryNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req deliveryHeartbeatRequest
	if err := readJSON(r, &req); err != nil {
		metrics.agentHeartbeatErrors.Add(1)
		errJSON(w, http.StatusBadRequest, "invalid heartbeat body")
		return
	}
	token := strings.TrimSpace(r.Header.Get("X-Express233-Token"))
	uid, tid, username, ok := s.pullIdentity(r, token)
	if !ok {
		metrics.agentHeartbeatErrors.Add(1)
		if token != "" {
			// An agent has no interactive CAPTCHA flow. Reuse the account/IP
			// throttling store so a leaked or guessed pull token cannot be tried
			// indefinitely against this otherwise public endpoint.
			banned, retry, remaining := s.recordLoginFailure(r, "agent-token")
			if banned {
				w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retry.Seconds()))))
				errJSON(w, http.StatusTooManyRequests, "too many invalid agent credential attempts; try again later")
				return
			}
			errJSON(w, http.StatusUnauthorized, "invalid agent credentials; "+strconv.Itoa(remaining)+" attempt(s) remaining before this IP is temporarily blocked")
			return
		}
		errJSON(w, http.StatusUnauthorized, "invalid agent credentials")
		return
	}
	if token != "" {
		s.clearLoginFailures(r)
	}
	if req.Project == "" || req.ServerID == "" {
		metrics.agentHeartbeatErrors.Add(1)
		errJSON(w, http.StatusBadRequest, "project and server_id required")
		return
	}
	project, err := s.Store.GetProjectByName(tid, req.Project)
	if err != nil {
		metrics.agentHeartbeatErrors.Add(1)
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	if !s.pullCanAccessProject(w, tid, uid, project.ID) {
		metrics.agentHeartbeatErrors.Add(1)
		return
	}
	if s.getServerFile(tid).Entry(req.ServerID) == nil {
		metrics.agentHeartbeatErrors.Add(1)
		errJSON(w, http.StatusBadRequest, "server_id not configured in server.yaml")
		return
	}
	if req.ErrorCode != "" && !safeAgentErrorCode(req.ErrorCode) {
		metrics.agentHeartbeatErrors.Add(1)
		errJSON(w, http.StatusBadRequest, "error_code must be a safe identifier")
		return
	}
	node, err := s.Store.HeartbeatDeliveryNode(store.DeliveryNodeHeartbeat{
		TenantID: tid, ProjectID: project.ID, ServerID: req.ServerID,
		Role: req.Role, Environment: req.Environment, Labels: req.Labels,
		OS: req.OS, Arch: req.Arch, CurrentVersion: req.CurrentVersion,
		AppliedGeneration: req.AppliedGeneration, Status: req.Status,
		LastError: req.ErrorCode, HeartbeatIntervalSeconds: req.HeartbeatIntervalSeconds,
	}, time.Now())
	if err != nil {
		metrics.agentHeartbeatErrors.Add(1)
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	metrics.agentHeartbeats.Add(1)
	// A version match alone is insufficient: a node must also acknowledge the
	// desired generation. This distinguishes a deliberate re-deploy of the same
	// version from an older successful deployment.
	needsDeploy := node.DesiredVersion != "" && (node.CurrentVersion != node.DesiredVersion || node.AppliedGeneration < node.DesiredGeneration)
	if needsDeploy {
		metrics.agentDrift.Add(1)
	}
	s.audit(r, username, "delivery.agent.heartbeat", "project="+req.Project+" server_id="+req.ServerID+" status="+node.Status)
	writeJSON(w, http.StatusOK, map[string]any{
		"server_time": time.Now().UTC().Format(time.RFC3339),
		"node":        node,
		"desired":     deliveryDesiredResponse{Version: node.DesiredVersion, Generation: node.DesiredGeneration, NeedsDeploy: needsDeploy},
	})
}

func safeAgentErrorCode(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) <= 64 && safeAPIIdentifier.MatchString(value)
}

func (s *Server) handleListDeliveryNodes(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pc, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	pullNodes, err := s.Store.ListDeliveryNodes(pc.TenantID, projectID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	bindings, err := s.Store.ListPushBindingsForSelector(pc.TenantID, nil, nil, true)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	hosts, err := s.Store.ListPushHosts(pc.TenantID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	hostByID := make(map[int64]store.PushHost, len(hosts))
	for _, host := range hosts {
		hostByID[host.ID] = host
	}
	views := make([]deliveryNodeView, 0, len(pullNodes)+len(bindings))
	now := time.Now()
	for _, node := range pullNodes {
		lastSeen, _ := time.ParseInLocation("2006-01-02 15:04:05", node.LastSeenAt, time.Local)
		grace := time.Duration(max(30, node.HeartbeatIntervalSeconds*2)) * time.Second
		online := !lastSeen.IsZero() && now.Sub(lastSeen) <= grace
		drift := node.DesiredVersion != "" && (node.CurrentVersion != node.DesiredVersion || node.AppliedGeneration < node.DesiredGeneration)
		views = append(views, deliveryNodeView{
			ID: "pull:" + strconv.FormatInt(node.ID, 10), DeliveryMode: "pull", ServerID: node.ServerID,
			Role: node.Role, Environment: node.Environment, Labels: node.Labels, OS: node.OS, Arch: node.Arch,
			CurrentVersion: node.CurrentVersion, DesiredVersion: node.DesiredVersion,
			DesiredGeneration: node.DesiredGeneration, AppliedGeneration: node.AppliedGeneration,
			AutoFollow: node.AutoFollow, Status: node.Status, LastError: node.LastError,
			LastSeenAt: node.LastSeenAt, Online: online, Drift: drift,
		})
	}
	for _, binding := range bindings {
		host := hostByID[binding.HostID]
		labels := splitAPILabels(binding.Labels)
		role, environment := deliveryDimensions(labels)
		current, versionErr := s.Store.LatestSuccessfulPushVersion(projectID, binding.ID)
		if versionErr != nil {
			errJSON(w, http.StatusInternalServerError, versionErr.Error())
			return
		}
		status := host.LastCheckStatus
		if status == "" {
			status = "unknown"
		}
		views = append(views, deliveryNodeView{
			ID: "push:" + strconv.FormatInt(binding.ID, 10), DeliveryMode: "push", ServerID: binding.ServerID,
			Role: role, Environment: environment, Labels: labels, OS: binding.OS, Arch: binding.Arch,
			CurrentVersion: current, Status: status, LastError: host.LastCheckError,
			LastSeenAt: host.LastCheckAt, Online: status == "ok", HostID: host.ID, HostName: host.Name,
		})
	}
	writeJSON(w, http.StatusOK, views)
}

type setDeliveryDesiredRequest struct {
	Version    string `json:"version"`
	AutoFollow *bool  `json:"auto_follow"`
}

func (s *Server) handleSetDeliveryNodeDesired(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pc, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	serverID := chi.URLParam(r, "serverID")
	if s.getServerFile(pc.TenantID).Entry(serverID) == nil {
		errJSON(w, http.StatusBadRequest, "server_id not configured in server.yaml")
		return
	}
	var req setDeliveryDesiredRequest
	if err := readJSON(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.Version = strings.TrimSpace(req.Version)
	if req.Version == "" && req.AutoFollow != nil && *req.AutoFollow {
		req.Version, err = s.Store.LatestPublishedVersion(projectID)
		if err != nil && err != sql.ErrNoRows {
			errJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if req.Version != "" {
		version, getErr := s.Store.GetVersion(projectID, req.Version)
		if getErr != nil || version.Status != "published" {
			errJSON(w, http.StatusBadRequest, "desired version must be published")
			return
		}
	}
	node, err := s.Store.SetDeliveryNodeDesired(pc.TenantID, projectID, serverID, req.Version, req.AutoFollow, time.Now())
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	metrics.desiredStateChanges.Add(1)
	s.auditSession(r, "delivery.desired.update", "project_id="+strconv.FormatInt(projectID, 10)+" server_id="+serverID+" version="+req.Version)
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleDeleteDeliveryNode(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pc, err := s.projectByID(r, projectID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "project not found")
		return
	}
	serverID := chi.URLParam(r, "serverID")
	if err := s.Store.DeleteDeliveryNode(pc.TenantID, projectID, serverID); err != nil {
		if err == sql.ErrNoRows {
			errJSON(w, http.StatusNotFound, "pull node not found")
			return
		}
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditSession(r, "delivery.node.delete", "project_id="+strconv.FormatInt(projectID, 10)+" server_id="+serverID)
	w.WriteHeader(http.StatusNoContent)
}

func splitAPILabels(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func deliveryDimensions(labels []string) (role, environment string) {
	for _, label := range labels {
		if strings.HasPrefix(label, "role:") {
			role = strings.TrimPrefix(label, "role:")
		}
		if strings.HasPrefix(label, "env:") {
			environment = strings.TrimPrefix(label, "env:")
		}
	}
	return role, environment
}
