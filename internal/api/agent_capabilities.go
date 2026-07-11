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
		"authentication":    []string{"JWT bearer", "HttpOnly session cookie", "pull token (pull endpoints only)"},
		"credential_policy": "SSH credentials are encrypted at rest and never returned by any API",
		"openapi":           "/api/openapi.yaml",
		"capabilities": []agentCapability{
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
			{Group: "ssh", Method: "GET", Path: "/api/push/hosts", Role: "admin", Description: "List SSH hosts without secrets"},
			{Group: "ssh", Method: "POST", Path: "/api/push/hosts", Role: "admin", Description: "Create an encrypted SSH credential"},
			{Group: "ssh", Method: "POST", Path: "/api/push/hosts/{hostID}/check", Role: "admin", Description: "Perform exactly one SSH health check"},
			{Group: "ssh", Method: "GET", Path: "/api/push/hosts/{hostID}/checks", Role: "admin", Description: "Read SSH health-check history"},
			{Group: "ssh", Method: "POST", Path: "/api/push/hosts/{hostID}/servers", Role: "admin", Description: "Bind server_id and remote root"},
			{Group: "deployment", Method: "POST", Path: "/api/projects/{id}/push-deployments", Role: "project writer", Description: "Select and deploy SSH targets serially"},
			{Group: "deployment", Method: "GET", Path: "/api/projects/{id}/push-deployments/{deploymentID}", Role: "operator", Description: "Read per-target deployment output"},
			{Group: "analytics", Method: "GET", Path: "/api/dashboard", Role: "operator", Description: "Read daily upload, publish, pull, and deploy metrics"},
		},
	})
}
