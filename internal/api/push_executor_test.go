package api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/neko233-com/express233/internal/store"
	"golang.org/x/crypto/ssh"
)

func TestDeploymentOutputSerializesStreamsAndBoundsMemory(t *testing.T) {
	var output deploymentOutput
	var writers sync.WaitGroup
	for i := 0; i < 4; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for j := 0; j < 100; j++ {
				_, _ = output.Write([]byte(strings.Repeat("x", 4096)))
			}
		}()
	}
	writers.Wait()
	_, _ = output.Write([]byte(strings.Repeat("y", 9*1024*1024) + "final-receipt"))
	if text := output.String(); len(text) > 8*1024*1024 || !strings.HasSuffix(text, "final-receipt") {
		t.Fatal("bounded output lost final receipt")
	}
}

func TestPushSSHRetryOnlyTransientFailures(t *testing.T) {
	attempts := 0
	_, err := dialPushSSHWithRetry(context.Background(), func(context.Context) (*ssh.Client, error) {
		attempts++
		if attempts < 3 {
			return nil, syscall.ECONNRESET
		}
		return nil, nil
	}, []time.Duration{0, 0, 0})
	if err != nil || attempts != 3 {
		t.Fatalf("reset retry: attempts=%d err=%v", attempts, err)
	}
	attempts = 0
	authError := errors.New("SSH host key mismatch")
	_, err = dialPushSSHWithRetry(context.Background(), func(context.Context) (*ssh.Client, error) {
		attempts++
		return nil, authError
	}, []time.Duration{0, 0})
	if !errors.Is(err, authError) || attempts != 1 {
		t.Fatalf("host key failure retried: attempts=%d err=%v", attempts, err)
	}
}

func TestPushSSHRetryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	_, err := dialPushSSHWithRetry(ctx, func(context.Context) (*ssh.Client, error) {
		attempts++
		cancel()
		return nil, syscall.ECONNRESET
	}, []time.Duration{time.Hour})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("cancellation: attempts=%d err=%v", attempts, err)
	}
}

func TestBuildPushDeployScriptUsesBindingBackupPolicy(t *testing.T) {
	command := pushCommand{CentralURL: "https://express.example.com", Project: "game", Version: "1.0.0", ServerID: "1001"}
	binding := store.PushServerBinding{ServerID: "1001", RemoteRoot: "/opt/game"}
	withBackup, err := buildPushDeployScript(command, binding)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withBackup, "safe-deploy.sh --backup --server-id '1001'") {
		t.Fatalf("backup-enabled command missing --backup: %s", withBackup)
	}
	binding.SkipBackup = true
	withoutBackup, err := buildPushDeployScript(command, binding)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(withoutBackup, " --backup ") || !strings.Contains(withoutBackup, "safe-deploy.sh --server-id '1001'") {
		t.Fatalf("skip-backup command is incorrect: %s", withoutBackup)
	}
}

func TestBuildPushDeployScriptForwardsQuotedHookEnvironment(t *testing.T) {
	command := pushCommand{CentralURL: "https://express.example.com", Project: "game", Version: "1.0.0", ServerID: "1001", HookEnv: map[string]string{"RESTART_MODE": "force", "LITERAL": "line one\n$(touch /tmp/forbidden)'end", "VERSION": "untrusted"}}
	script, err := buildPushDeployScript(command, store.PushServerBinding{ServerID: "1001", RemoteRoot: "/opt/game"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{shellQuote("RESTART_MODE=force"), shellQuote("LITERAL=" + command.HookEnv["LITERAL"])} {
		if !strings.Contains(script, value) {
			t.Fatal("post-hook environment was lost or not quoted")
		}
	}
	if strings.Index(script, shellQuote("VERSION=untrusted")) > strings.Index(script, "VERSION='1.0.0'") {
		t.Fatal("hook environment replaced trusted deployment context")
	}
	for _, key := range []string{"", "-i", "A=B", "X;touch /tmp/forbidden", "1BAD", "X\nY"} {
		command.HookEnv = map[string]string{key: "fixture"}
		if _, err := buildPushDeployScript(command, store.PushServerBinding{}); err == nil {
			t.Fatalf("invalid environment name accepted: %q", key)
		}
	}
	command.HookEnv = map[string]string{"VALID": "invalid\x00value"}
	if _, err := buildPushDeployScript(command, store.PushServerBinding{}); err == nil {
		t.Fatal("NUL environment value accepted")
	}
}
