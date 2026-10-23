package api

import (
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

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
