package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/neko233-com/express233/internal/store"
)

type pushHostRequest struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Username   string `json:"username"`
	HostKey    string `json:"host_key"`
	PrivateKey string `json:"private_key"`
	Credential string `json:"credential"`
	AuthMode   string `json:"auth_mode"`
	Port       int    `json:"port"`
}
type pushBindingRequest struct {
	ServerID    string `json:"server_id"`
	Labels      string `json:"labels"`
	ContentTags string `json:"content_tags"`
	RemoteRoot  string `json:"remote_root"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
}
type createPushDeploymentRequest struct {
	Version   string   `json:"version"`
	ServerIDs []string `json:"server_ids"`
	Tags      []string `json:"tags"`
	TagMatch  string   `json:"tag_match"`
	DryRun    bool     `json:"dry_run"`
}

func (s *Server) pushCipher() (*pushCredentialCipher, error) { return newPushCredentialCipher() }

func (s *Server) handleListPushHosts(w http.ResponseWriter, r *http.Request) {
	tid, _ := s.tenantFromSession(r)
	hosts, err := s.Store.ListPushHosts(tid)
	if err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, hosts)
}
func (s *Server) handleCreatePushHost(w http.ResponseWriter, r *http.Request) {
	var req pushHostRequest
	if err := readJSON(r, &req); err != nil {
		errJSON(w, 400, "invalid body")
		return
	}
	if err := validatePushHostRequest(req, true); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	secretValue := req.Credential
	if secretValue == "" {
		secretValue = req.PrivateKey
	}
	secret := ""
	if req.AuthMode != "agent" {
		cipher, err := s.pushCipher()
		if err != nil {
			errJSON(w, 503, err.Error())
			return
		}
		secret, err = cipher.encrypt(secretValue)
		if err != nil {
			errJSON(w, 400, err.Error())
			return
		}
	}
	tid, _ := s.tenantFromSession(r)
	host, err := s.Store.CreatePushHost(store.PushHost{TenantID: tid, Name: req.Name, Address: req.Address, Port: req.Port, Username: req.Username, AuthMode: req.AuthMode, HostKey: req.HostKey}, secret)
	if err != nil {
		errJSON(w, 400, err.Error())
		return
	}
	s.auditSession(r, "push.host.create", "name="+host.Name)
	writeJSON(w, 201, host)
}
func (s *Server) handleGetPushHost(w http.ResponseWriter, r *http.Request) {
	tid, _ := s.tenantFromSession(r)
	h, _, err := s.Store.GetPushHost(tid, pushHostID(r))
	if err != nil {
		errJSON(w, 404, "host not found")
		return
	}
	writeJSON(w, 200, h)
}
func (s *Server) handleUpdatePushHost(w http.ResponseWriter, r *http.Request) {
	var req pushHostRequest
	if err := readJSON(r, &req); err != nil {
		errJSON(w, 400, "invalid body")
		return
	}
	tid, _ := s.tenantFromSession(r)
	current, _, err := s.Store.GetPushHost(tid, pushHostID(r))
	if err != nil {
		errJSON(w, http.StatusNotFound, "host not found")
		return
	}
	if req.HostKey == "" {
		req.HostKey = current.HostKey
	}
	if req.AuthMode == "" {
		req.AuthMode = current.AuthMode
	}
	if err := validatePushHostRequest(req, false); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	var key string
	secretValue := req.Credential
	if secretValue == "" {
		secretValue = req.PrivateKey
	}
	if secretValue != "" {
		c, err := s.pushCipher()
		if err != nil {
			errJSON(w, 503, err.Error())
			return
		}
		key, err = c.encrypt(secretValue)
		if err != nil {
			errJSON(w, 400, err.Error())
			return
		}
	}
	err = s.Store.UpdatePushHost(store.PushHost{ID: pushHostID(r), TenantID: tid, Name: req.Name, Address: req.Address, Port: req.Port, Username: req.Username, AuthMode: req.AuthMode, HostKey: req.HostKey}, key)
	if err != nil {
		errJSON(w, 400, err.Error())
		return
	}
	w.WriteHeader(204)
}
func (s *Server) handleDeletePushHost(w http.ResponseWriter, r *http.Request) {
	tid, _ := s.tenantFromSession(r)
	if err := s.Store.DeletePushHost(tid, pushHostID(r)); err != nil {
		errJSON(w, 404, "host not found")
		return
	}
	w.WriteHeader(204)
}
func pushHostID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "hostID"), 10, 64)
	return id
}

func (s *Server) handleListPushBindings(w http.ResponseWriter, r *http.Request) {
	tid, _ := s.tenantFromSession(r)
	items, err := s.Store.ListPushServerBindings(tid, pushHostID(r))
	if err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) handleCreatePushBinding(w http.ResponseWriter, r *http.Request) {
	var req pushBindingRequest
	if err := readJSON(r, &req); err != nil {
		errJSON(w, 400, "invalid body")
		return
	}
	if err := validatePushBindingRequest(req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	tid, _ := s.tenantFromSession(r)
	item, err := s.Store.CreatePushServerBinding(store.PushServerBinding{TenantID: tid, HostID: pushHostID(r), ServerID: req.ServerID, Labels: req.Labels, ContentTags: req.ContentTags, RemoteRoot: req.RemoteRoot, OS: req.OS, Arch: req.Arch})
	if err != nil {
		errJSON(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) handleUpdatePushBinding(w http.ResponseWriter, r *http.Request) {
	var req pushBindingRequest
	if err := readJSON(r, &req); err != nil {
		errJSON(w, 400, "invalid body")
		return
	}
	if err := validatePushBindingRequest(req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	tid, _ := s.tenantFromSession(r)
	id, _ := strconv.ParseInt(chi.URLParam(r, "bindingID"), 10, 64)
	if err := s.Store.UpdatePushServerBinding(store.PushServerBinding{ID: id, TenantID: tid, HostID: pushHostID(r), ServerID: req.ServerID, Labels: req.Labels, ContentTags: req.ContentTags, RemoteRoot: req.RemoteRoot, OS: req.OS, Arch: req.Arch}); err != nil {
		errJSON(w, 400, err.Error())
		return
	}
	w.WriteHeader(204)
}
func (s *Server) handleDeletePushBinding(w http.ResponseWriter, r *http.Request) {
	tid, _ := s.tenantFromSession(r)
	id, _ := strconv.ParseInt(chi.URLParam(r, "bindingID"), 10, 64)
	if err := s.Store.DeletePushServerBinding(tid, pushHostID(r), id); err != nil {
		errJSON(w, 404, "binding not found")
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleCreatePushDeployment(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, 404, "project not found")
		return
	}
	var req createPushDeploymentRequest
	if err := readJSON(r, &req); err != nil {
		errJSON(w, 400, "invalid body")
		return
	}
	if req.TagMatch == "" {
		req.TagMatch = "all"
	}
	if req.TagMatch != "all" && req.TagMatch != "any" {
		errJSON(w, 400, "tag_match must be all or any")
		return
	}
	if len(req.Tags) == 0 {
		req.Tags = []string{"test"}
	}
	if req.Version == "" {
		req.Version, err = s.Store.LatestPublishedVersion(pid)
		if err != nil {
			errJSON(w, 400, "version required when no published version exists")
			return
		}
	}
	v, err := s.Store.GetVersion(pid, req.Version)
	if err != nil || v.Status != "published" {
		errJSON(w, 400, "version must be published")
		return
	}
	targets, err := s.Store.ListPushBindingsForSelector(pc.TenantID, req.ServerIDs, req.Tags, req.TagMatch == "all")
	if err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	selector, _ := json.Marshal(req)
	sess, _ := s.currentSession(r)
	d, err := s.Store.CreatePushDeployment(store.PushDeployment{TenantID: pc.TenantID, ProjectID: pid, Version: req.Version, RequestedBy: sess.Username, Selector: string(selector)}, targets)
	if err != nil {
		errJSON(w, 400, err.Error())
		return
	}
	s.auditSession(r, "push.deploy.create", fmt.Sprintf("project_id=%d version=%s targets=%d", pid, req.Version, len(targets)))
	if req.DryRun {
		s.completePushDryRun(d.ID)
		d.Status = "success"
	} else {
		go s.runPushDeployment(d.ID, pc.TenantID, pc.ProjectName)
	}
	writeJSON(w, 201, d)
}
func (s *Server) completePushDryRun(id int64) {
	_ = s.Store.MarkPushDeploymentRunning(id)
	d, err := s.Store.GetPushDeploymentByID(id)
	if err != nil {
		return
	}
	for _, t := range d.Targets {
		_ = s.Store.MarkPushTarget(t.ID, "success", "dry-run: target selected; no SSH connection made")
	}
	_ = s.Store.CompletePushDeployment(id)
}
func (s *Server) handleListPushDeployments(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, 404, "project not found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.Store.ListPushDeployments(pc.TenantID, pid, limit)
	if err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) handleGetPushDeployment(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, 404, "project not found")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "deploymentID"), 10, 64)
	d, err := s.Store.GetPushDeployment(pc.TenantID, pid, id)
	if err != nil {
		errJSON(w, 404, "deployment not found")
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) handleDeletePushDeployment(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pc, err := s.projectByID(r, pid)
	if err != nil {
		errJSON(w, 404, "project not found")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "deploymentID"), 10, 64)
	if err := s.Store.DeletePushDeployment(pc.TenantID, pid, id); err != nil {
		errJSON(w, 404, "deployment not found")
		return
	}
	w.WriteHeader(204)
}

func (s *Server) runPushDeployment(id, tenantID int64, projectName string) {
	_ = s.Store.MarkPushDeploymentRunning(id)
	d, err := s.Store.GetPushDeploymentByID(id)
	if err != nil {
		return
	}
	centralURL, err := pushPublicURL()
	if err != nil {
		for _, target := range d.Targets {
			_ = s.Store.MarkPushTarget(target.ID, "failed", appendDeploymentError("", err))
		}
		_ = s.Store.CompletePushDeployment(id)
		return
	}
	// A single request is intentionally serial. This avoids stopping multiple
	// game servers concurrently when several bindings point to one host.
	for _, target := range d.Targets {
		_ = s.Store.MarkPushTarget(target.ID, "running", "")
		binding, err := s.Store.GetPushBinding(tenantID, target.BindingID)
		if err != nil {
			_ = s.Store.MarkPushTarget(target.ID, "failed", err.Error())
			continue
		}
		host, _, err := s.Store.GetPushHost(tenantID, target.HostID)
		if err != nil {
			_ = s.Store.MarkPushTarget(target.ID, "failed", err.Error())
			continue
		}
		// The remote machine installs/runs the same CLI used in pull mode. Its
		// stdout+stderr includes the CLI, safe deploy, and post-hook output.
		user, err := s.Store.GetUserByNameInTenant(tenantID, d.RequestedBy)
		if err != nil {
			_ = s.Store.MarkPushTarget(target.ID, "failed", err.Error())
			continue
		}
		output, err := s.pushExecutor.Deploy(withPushCommand(withAPIStore(context.Background(), s.Store), pushCommand{CentralURL: centralURL, Project: projectName, Version: d.Version, ServerID: binding.ServerID, PullToken: user.Token}), *host, *binding, d.Version, nil)
		status := "success"
		if err != nil {
			status = "failed"
			output = appendDeploymentError(output, err)
		}
		_ = s.Store.MarkPushTarget(target.ID, status, output)
	}
	_ = s.Store.CompletePushDeployment(id)
}
func splitCSV(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
func appendDeploymentError(output string, err error) string {
	if output != "" {
		output += "\n"
	}
	return output + "ERROR: " + err.Error()
}
