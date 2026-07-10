package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/express233/internal/store"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type pushExecutor interface {
	Deploy(context.Context, store.PushHost, store.PushServerBinding, string, io.Reader) (string, error)
}
type sshPushExecutor struct{ credentials *pushCredentialCipher }
type pushCommand struct{ CentralURL, Project, Version, ServerID, PullToken string }
type pushCommandContextKey struct{}

func withPushCommand(ctx context.Context, command pushCommand) context.Context {
	return context.WithValue(ctx, pushCommandContextKey{}, command)
}
func commandFromContext(ctx context.Context) (pushCommand, bool) {
	command, ok := ctx.Value(pushCommandContextKey{}).(pushCommand)
	return command, ok
}

func (e sshPushExecutor) Deploy(ctx context.Context, host store.PushHost, binding store.PushServerBinding, _ string, _ io.Reader) (string, error) {
	command, ok := commandFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("push command unavailable")
	}
	config, err := e.clientConfig(ctx, host)
	if err != nil {
		return "", err
	}
	addr := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("connect %s: %w", host.Name, err)
	}
	defer conn.Close()
	session, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var output bytes.Buffer
	session.Stdout, session.Stderr = &output, &output
	// The command intentionally delegates extraction, safe swap and post hook to
	// express233-cli. Capturing both streams makes the task log the source of
	// truth for installation, pull, deploy and hook diagnostics.
	// Push deployment intentionally delegates the atomic stop/swap/start to the
	// pre-installed host script.  A controller must never overwrite a running
	// game binary directly. The pull token is an environment value rather than
	// an argv argument so ordinary process listings do not disclose it.
	script := "set -eu; command -v safe-deploy.sh >/dev/null 2>&1 || { echo 'safe-deploy.sh is required on the target'; exit 127; }; EXPRESS233_SERVER=" + shellQuote(command.CentralURL) + " EXPRESS233_TOKEN=" + shellQuote(command.PullToken) + " EXPRESS233_PROJECT=" + shellQuote(command.Project) + " EXPRESS233_SERVER_ID=" + shellQuote(command.ServerID) + " VERSION=" + shellQuote(command.Version) + " EXPRESS233_TAGS=" + shellQuote(strings.Join(splitCSV(binding.ContentTags), ",")) + " GAME_ROOT=" + shellQuote(binding.RemoteRoot) + " safe-deploy.sh --server-id " + shellQuote(command.ServerID)
	if err := session.Run(script); err != nil {
		return output.String(), fmt.Errorf("remote CLI deploy: %w", err)
	}
	return output.String(), nil
}

func (e sshPushExecutor) clientConfig(ctx context.Context, host store.PushHost) (*ssh.ClientConfig, error) {
	if e.credentials == nil && host.AuthMode != "agent" {
		return nil, fmt.Errorf("push SSH credentials are not configured")
	}
	st := apiStoreFromContext(ctx)
	if st == nil {
		return nil, fmt.Errorf("push deployment store unavailable")
	}
	_, encoded, err := st.GetPushHost(host.TenantID, host.ID)
	if err != nil {
		return nil, err
	}
	methods := []ssh.AuthMethod{}
	if host.AuthMode != "agent" {
		secret, err := e.credentials.decrypt(encoded)
		if err != nil {
			return nil, fmt.Errorf("decrypt SSH credential: %w", err)
		}
		if host.AuthMode == "password" {
			methods = append(methods, ssh.Password(secret))
		} else {
			signer, err := ssh.ParsePrivateKey([]byte(secret))
			if err != nil {
				return nil, fmt.Errorf("parse SSH private key: %w", err)
			}
			methods = append(methods, ssh.PublicKeys(signer))
		}
	} else {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return nil, fmt.Errorf("SSH_AUTH_SOCK is required for agent authentication")
		}
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return nil, fmt.Errorf("connect SSH agent: %w", err)
		}
		methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
	}
	callback := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if host.HostKey != "" {
			expected, _, _, _, err := ssh.ParseAuthorizedKey([]byte(host.HostKey))
			if err != nil {
				return fmt.Errorf("parse stored SSH host key: %w", err)
			}
			if !bytes.Equal(expected.Marshal(), key.Marshal()) {
				return fmt.Errorf("SSH host key mismatch: expected %s got %s", ssh.FingerprintSHA256(expected), ssh.FingerprintSHA256(key))
			}
			return nil
		}
		if !allowPushTOFU() {
			return fmt.Errorf("SSH host key is not enrolled")
		}
		return st.RecordPushHostKey(host.TenantID, host.ID, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))), ssh.FingerprintSHA256(key))
	}
	return &ssh.ClientConfig{User: host.Username, Auth: methods, HostKeyCallback: callback, Timeout: 20 * time.Second}, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\\"'\\\"'") + "'"
}

type pushStoreContextKey struct{}

func withAPIStore(ctx context.Context, st *store.Store) context.Context {
	return context.WithValue(ctx, pushStoreContextKey{}, st)
}
func apiStoreFromContext(ctx context.Context) *store.Store {
	st, _ := ctx.Value(pushStoreContextKey{}).(*store.Store)
	return st
}

// pushPublicURL must name the public, TLS-protected controller address. Using
// a loopback fallback here would make remote nodes accidentally pull from
// themselves, which is both unsafe and non-functional.
func pushPublicURL() (string, error) {
	raw := strings.TrimRight(os.Getenv("EXPRESS233_PUBLIC_URL"), "/")
	if raw == "" {
		return "", fmt.Errorf("EXPRESS233_PUBLIC_URL is required for SSH push deployment")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", fmt.Errorf("EXPRESS233_PUBLIC_URL must be an absolute http(s) URL without credentials")
	}
	if u.Scheme != "https" && os.Getenv("EXPRESS233_ALLOW_INSECURE_PUSH_URL") != "1" {
		return "", fmt.Errorf("EXPRESS233_PUBLIC_URL must use https (set EXPRESS233_ALLOW_INSECURE_PUSH_URL=1 only for controlled development)")
	}
	return raw, nil
}
