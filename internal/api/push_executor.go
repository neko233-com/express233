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
type pushHealthChecker interface {
	Check(context.Context, store.PushHost) error
}
type sshPushExecutor struct{ credentials *pushCredentialCipher }
type pushCommand struct {
	CentralURL, Project, Version, ServerID, PullToken string
	DeploymentID, IdempotencyKey, TargetTag           string
}
type pushCommandContextKey struct{}

// Check performs exactly one TCP connection and SSH handshake. It intentionally
// has no retry loop: the scheduler records the result and waits for the next
// configured interval after either success or failure.
func (e sshPushExecutor) Check(ctx context.Context, host store.PushHost) error {
	config, cleanup, err := e.clientConfig(ctx, host)
	if err != nil {
		return err
	}
	defer cleanup()
	addr := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))
	dialer := net.Dialer{Timeout: config.Timeout}
	connection, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connect %s: %w", host.Name, err)
	}
	defer func() { _ = connection.Close() }()
	sshConnection, channels, requests, err := ssh.NewClientConn(connection, addr, config)
	if err != nil {
		return fmt.Errorf("SSH handshake %s: %w", host.Name, err)
	}
	client := ssh.NewClient(sshConnection, channels, requests)
	return client.Close()
}

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
	config, cleanup, err := e.clientConfig(ctx, host)
	if err != nil {
		return "", err
	}
	defer cleanup()
	addr := net.JoinHostPort(host.Address, strconv.Itoa(host.Port))
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("connect %s: %w", host.Name, err)
	}
	defer func() { _ = conn.Close() }()
	session, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer func() { _ = session.Close() }()
	var output bytes.Buffer
	session.Stdout, session.Stderr = &output, &output
	// The command intentionally delegates extraction, safe swap and post hook to
	// express233-cli. Capturing both streams makes the task log the source of
	// truth for installation, pull, deploy and hook diagnostics.
	// Push deployment intentionally delegates the atomic stop/swap/start to the
	// pre-installed host script.  A controller must never overwrite a running
	// game binary directly. The pull token is an environment value rather than
	// an argv argument so ordinary process listings do not disclose it.
	script := "set -eu\n" +
		"command -v safe-deploy.sh >/dev/null 2>&1 || { echo 'safe-deploy.sh is required on the target'; exit 127; }\n" +
		"EXPRESS233_SERVER=" + shellQuote(command.CentralURL) + " EXPRESS233_TOKEN=" + shellQuote(command.PullToken) + " EXPRESS233_PROJECT=" + shellQuote(command.Project) + " EXPRESS233_SERVER_ID=" + shellQuote(command.ServerID) + " VERSION=" + shellQuote(command.Version) + " EXPRESS233_TAGS=" + shellQuote(strings.Join(splitCSV(binding.ContentTags), ",")) + " GAME_ROOT=" + shellQuote(binding.RemoteRoot) + " EXPRESS233_DEPLOYMENT_ID=" + shellQuote(command.DeploymentID) + " EXPRESS233_IDEMPOTENCY_KEY=" + shellQuote(command.IdempotencyKey) + " EXPRESS233_TARGET_TAG=" + shellQuote(command.TargetTag) + " safe-deploy.sh --backup --server-id " + shellQuote(command.ServerID) + "\n"
	// Feed the script over stdin so the pull token and other environment values
	// never appear in the remote shell process argv or ordinary process lists.
	session.Stdin = strings.NewReader(script)
	if err := session.Run("sh -s"); err != nil {
		return output.String(), fmt.Errorf("remote CLI deploy: %w", err)
	}
	return output.String(), nil
}

func (e sshPushExecutor) clientConfig(ctx context.Context, host store.PushHost) (*ssh.ClientConfig, func(), error) {
	cleanup := func() {}
	if e.credentials == nil && host.AuthMode != "agent" {
		return nil, cleanup, fmt.Errorf("push SSH credentials are not configured")
	}
	st := apiStoreFromContext(ctx)
	if st == nil {
		return nil, cleanup, fmt.Errorf("push deployment store unavailable")
	}
	_, encoded, err := st.GetPushHost(host.TenantID, host.ID)
	if err != nil {
		return nil, cleanup, err
	}
	methods := []ssh.AuthMethod{}
	if host.AuthMode != "agent" {
		secret, err := e.credentials.decrypt(encoded)
		if err != nil {
			return nil, cleanup, fmt.Errorf("decrypt SSH credential: %w", err)
		}
		if host.AuthMode == "password" {
			methods = append(methods, ssh.Password(secret))
		} else {
			signer, err := ssh.ParsePrivateKey([]byte(secret))
			if err != nil {
				return nil, cleanup, fmt.Errorf("parse SSH private key: %w", err)
			}
			methods = append(methods, ssh.PublicKeys(signer))
		}
	} else {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return nil, cleanup, fmt.Errorf("SSH_AUTH_SOCK is required for agent authentication")
		}
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return nil, cleanup, fmt.Errorf("connect SSH agent: %w", err)
		}
		cleanup = func() { _ = conn.Close() }
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
	return &ssh.ClientConfig{User: host.Username, Auth: methods, HostKeyCallback: callback, Timeout: 20 * time.Second}, cleanup, nil
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
