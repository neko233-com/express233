package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/store"
)

type preparedPushTarget struct {
	target      store.PushDeploymentTarget
	host        store.PushHost
	binding     store.PushServerBinding
	environment map[string]string
}

// freezePushPlan 固定工件、主机身份、实例目录与钩子环境，排队期间的变更只能创建新任务。
func (s *Server) freezePushPlan(tenantID, projectID int64, projectName, version string, targets []store.PushServerBinding) ([]store.PushServerBinding, error) {
	s.pushConfigMu.RLock()
	defer s.pushConfigMu.RUnlock()
	frozen := append([]store.PushServerBinding(nil), targets...)
	seen := map[string]bool{}
	for i := range frozen {
		b := &frozen[i]
		current, err := s.Store.GetPushBinding(tenantID, b.ID)
		if err != nil {
			return nil, fmt.Errorf("deployment binding unavailable")
		}
		*b = *current
		host, _, err := s.Store.GetPushHost(tenantID, b.HostID)
		if err != nil {
			return nil, fmt.Errorf("deployment host unavailable")
		}
		key := fmt.Sprintf("%s:%d/%s/%s", strings.ToLower(host.Address), host.Port, path.Clean(b.RemoteRoot), b.ServerID)
		if seen[key] {
			return nil, fmt.Errorf("duplicate physical deployment target")
		}
		seen[key] = true
		digest, _, err := s.pushPlanDigest(tenantID, projectID, projectName, version, *host, *b)
		if err != nil {
			return nil, err
		}
		b.PlanDigest = digest
	}
	return frozen, nil
}

func (s *Server) pushPlanDigest(tenantID, projectID int64, projectName, version string, host store.PushHost, binding store.PushServerBinding) (string, map[string]string, error) {
	if binding.ProjectName != projectName || !safeAPIIdentifier.MatchString(binding.ServerID) || !path.IsAbs(binding.RemoteRoot) || path.Clean(binding.RemoteRoot) == "/" || strings.Contains(binding.RemoteRoot, "\x00") || binding.OS != "linux" {
		return "", nil, fmt.Errorf("deployment target identity, Linux platform or remote root invalid")
	}
	v, err := s.Store.GetVersion(projectID, version)
	if err != nil || v.Status != "published" {
		return "", nil, fmt.Errorf("fixed deployment version is no longer published")
	}
	environment := map[string]string{}
	serverFile := s.getServerFile(tenantID)
	var entrySnapshot *config.ServerEntry
	s.serverMu.RLock()
	if serverFile != nil {
		if entry := serverFile.Entry(binding.ServerID); entry != nil {
			entrySnapshot = entry
			for key, value := range entry.PostHookEnv {
				environment[key] = value
			}
		}
	}
	s.serverMu.RUnlock()
	// Marshal sorts map keys. Only the digest is persisted; the clear-text environment stays in process memory.
	data, err := json.Marshal(struct {
		Project, Version, Artifact  string
		HostID                      int64
		Address                     string
		Port                        int
		Username, AuthMode, HostKey string
		Binding                     store.PushServerBinding
		Environment                 map[string]string
		Configuration               *config.ServerEntry
	}{projectName, version, v.Artifact.SHA256, host.ID, host.Address, host.Port, host.Username, host.AuthMode, host.HostKey, binding, environment, entrySnapshot})
	if err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), environment, nil
}
