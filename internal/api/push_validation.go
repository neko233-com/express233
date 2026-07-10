package api

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

var pushUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func allowPushTOFU() bool { return os.Getenv("EXPRESS233_PUSH_ALLOW_TOFU") == "1" }

func validatePushHostRequest(req pushHostRequest, requireCredential bool) error {
	req.Name, req.Address, req.Username = strings.TrimSpace(req.Name), strings.TrimSpace(req.Address), strings.TrimSpace(req.Username)
	if req.Name == "" || req.Address == "" || !pushUsernamePattern.MatchString(req.Username) {
		return fmt.Errorf("host name, address and a valid SSH username are required")
	}
	if req.Port != 0 && (req.Port < 1 || req.Port > 65535) {
		return fmt.Errorf("SSH port must be between 1 and 65535")
	}
	if strings.ContainsAny(req.Address, "/\\@ ") || (net.ParseIP(req.Address) == nil && !regexp.MustCompile(`^[A-Za-z0-9.-]{1,253}$`).MatchString(req.Address)) {
		return fmt.Errorf("SSH address must be a hostname or IP address, without a scheme or port")
	}
	if req.AuthMode == "" {
		req.AuthMode = "private_key"
	}
	if req.AuthMode != "password" && req.AuthMode != "private_key" && req.AuthMode != "agent" {
		return fmt.Errorf("auth_mode must be password, private_key, or agent")
	}
	if req.HostKey == "" && !allowPushTOFU() {
		return fmt.Errorf("host_key is required; set EXPRESS233_PUSH_ALLOW_TOFU=1 only for controlled first enrollment")
	}
	if req.HostKey != "" {
		if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.HostKey)); err != nil {
			return fmt.Errorf("invalid SSH host_key: %w", err)
		}
	}
	secret := req.Credential
	if secret == "" {
		secret = req.PrivateKey
	}
	if requireCredential && req.AuthMode != "agent" && strings.TrimSpace(secret) == "" {
		return fmt.Errorf("SSH credential is required")
	}
	if secret != "" && req.AuthMode == "password" && len(secret) < 8 {
		return fmt.Errorf("SSH password must contain at least 8 characters")
	}
	if secret != "" && req.AuthMode == "private_key" {
		if _, err := ssh.ParsePrivateKey([]byte(secret)); err != nil {
			return fmt.Errorf("invalid SSH private key: %w", err)
		}
	}
	return nil
}

func validatePushBindingRequest(req pushBindingRequest) error {
	if strings.TrimSpace(req.ServerID) == "" || strings.TrimSpace(req.RemoteRoot) == "" {
		return fmt.Errorf("server_id and remote_root are required")
	}
	if !strings.HasPrefix(req.RemoteRoot, "/") || strings.ContainsRune(req.RemoteRoot, '\x00') {
		return fmt.Errorf("remote_root must be an absolute POSIX path")
	}
	if req.OS != "" && req.OS != "linux" {
		return fmt.Errorf("SSH push currently supports linux targets only; use pull mode for Windows targets")
	}
	return nil
}
