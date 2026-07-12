package api

import "net/http"

type agentCapability struct {
	Group       string `json:"group"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Role        string `json:"role"`
	Description string `json:"description"`
}

// handleAgentCapabilities is the authenticated, machine-readable inventory for
// deployment agents. The OpenAPI document remains the request/response schema.
func (s *Server) handleAgentCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"authentication":    []string{"JWT bearer", "HttpOnly session cookie", "Basic Auth", "pull token (pull and agent heartbeat endpoints)"},
		"credential_policy": "SSH credentials are encrypted at rest and never returned by any API",
		"log_retention":     "Operational and release logs are immutable and retained for a rolling 30-day window",
		"openapi":           "/api/openapi.yaml",
		"capabilities": []agentCapability{
			{Group: "guide", Method: "GET", Path: "/api/agent/guide", Role: "public", Description: "List reviewed, non-sensitive agent onboarding topics"},
			{Group: "guide", Method: "GET", Path: "/api/agent/guide/{topic}", Role: "public", Description: "Read a reviewed, non-sensitive official onboarding topic"},
			{Group: "discovery", Method: "GET", Path: "/api/agent/capabilities", Role: "operator", Description: "Discover Agent-operable HTTP endpoints"},
			{Group: "projects", Method: "GET", Path: "/api/projects", Role: "operator", Description: "List accessible projects"},
			{Group: "versions", Method: "POST", Path: "/api/projects/{id}/versions", Role: "project writer", Description: "Create a version"},
			{Group: "versions", Method: "POST", Path: "/api/projects/{id}/versions/{ver}/files", Role: "project writer", Description: "Upload binary, scripts, and nested configuration"},
			{Group: "versions", Method: "POST", Path: "/api/projects/{id}/versions/{ver}/publish", Role: "project writer", Description: "Validate and publish a version"},
			{Group: "versions", Method: "GET", Path: "/api/projects/{id}/versions/{ver}/validate", Role: "operator", Description: "Run publish validation"},
			{Group: "configuration", Method: "GET", Path: "/api/server-ids", Role: "operator", Description: "List server_id configuration targets"},
			{Group: "configuration", Method: "GET", Path: "/api/deploy/preview", Role: "operator", Description: "Preview server_id template replacement"},
			{Group: "configuration", Method: "GET", Path: "/api/deploy/diff", Role: "operator", Description: "Inspect configuration diff"},
			{Group: "pull", Method: "GET", Path: "/api/pull", Role: "pull token", Description: "Pull and safely materialize a release"},
			{Group: "pull", Method: "GET", Path: "/api/pull/preview", Role: "pull token", Description: "Preview pull contents without deployment"},
			{Group: "delivery", Method: "POST", Path: "/api/agent/nodes/heartbeat", Role: "pull token or account", Description: "Register a pull node, report applied generation, and receive desired state"},
			{Group: "delivery", Method: "GET", Path: "/api/projects/{id}/delivery-nodes", Role: "operator", Description: "List push and pull nodes with online and drift state"},
			{Group: "delivery", Method: "PUT", Path: "/api/projects/{id}/delivery-nodes/{serverID}/desired", Role: "project writer", Description: "Set pull-node desired version and auto-follow policy"},
			{Group: "ssh", Method: "GET", Path: "/api/push/hosts", Role: "admin", Description: "List SSH hosts without secrets"},
			{Group: "ssh", Method: "POST", Path: "/api/push/hosts", Role: "admin", Description: "Create an encrypted SSH credential"},
			{Group: "ssh", Method: "POST", Path: "/api/push/hosts/{hostID}/check", Role: "admin", Description: "Perform exactly one SSH health check"},
			{Group: "ssh", Method: "GET", Path: "/api/push/hosts/{hostID}/checks", Role: "admin", Description: "Read SSH health-check history"},
			{Group: "ssh", Method: "POST", Path: "/api/push/hosts/{hostID}/servers", Role: "admin", Description: "Bind server_id and remote root"},
			{Group: "deployment", Method: "POST", Path: "/api/projects/{id}/push-deployments", Role: "project writer", Description: "Select and deploy SSH targets serially"},
			{Group: "deployment", Method: "POST", Path: "/api/projects/{id}/push-tasks", Role: "project writer", Description: "Create a reusable release task"},
			{Group: "deployment", Method: "PUT", Path: "/api/projects/{id}/push-tasks/{taskID}", Role: "project writer", Description: "Update a reusable release task"},
			{Group: "deployment", Method: "POST", Path: "/api/projects/{id}/push-tasks/{taskID}/run", Role: "project writer", Description: "Repeat a task and snapshot its selector into immutable logs"},
			{Group: "deployment", Method: "GET", Path: "/api/projects/{id}/push-deployments", Role: "operator", Description: "Read immutable release logs retained for 30 days"},
			{Group: "deployment", Method: "GET", Path: "/api/projects/{id}/push-deployments/{deploymentID}", Role: "operator", Description: "Read per-target deployment output"},
			{Group: "hooks", Method: "GET", Path: "/api/projects/{id}/release-hooks", Role: "operator", Description: "List persisted version-publish hooks and pending debounce state"},
			{Group: "hooks", Method: "POST", Path: "/api/projects/{id}/release-hooks", Role: "project writer", Description: "Create an enabled or disabled release hook"},
			{Group: "hooks", Method: "PUT", Path: "/api/projects/{id}/release-hooks/{hookID}", Role: "project writer", Description: "Enable, disable, or update a release hook"},
			{Group: "hooks", Method: "POST", Path: "/api/projects/{id}/release-hooks/{hookID}/trigger", Role: "project writer", Description: "Queue a trailing-debounced hook trigger"},
			{Group: "hooks", Method: "GET", Path: "/api/projects/{id}/release-hook-events", Role: "operator", Description: "Read 30-day hook trigger and dispatch history"},
			{Group: "analytics", Method: "GET", Path: "/api/dashboard", Role: "operator", Description: "Read daily upload, publish, pull, and deploy metrics"},
		},
	})
}
