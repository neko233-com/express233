package api

import (
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestBuildPushDeployScriptUsesBindingBackupPolicy(t *testing.T) {
	command := pushCommand{CentralURL: "https://express.example.com", Project: "game", Version: "1.0.0", ServerID: "1001"}
	binding := store.PushServerBinding{ServerID: "1001", RemoteRoot: "/opt/game"}
	withBackup := buildPushDeployScript(command, binding)
	if !strings.Contains(withBackup, "safe-deploy.sh --backup --server-id '1001'") {
		t.Fatalf("backup-enabled command missing --backup: %s", withBackup)
	}
	binding.SkipBackup = true
	withoutBackup := buildPushDeployScript(command, binding)
	if strings.Contains(withoutBackup, " --backup ") || !strings.Contains(withoutBackup, "safe-deploy.sh --server-id '1001'") {
		t.Fatalf("skip-backup command is incorrect: %s", withoutBackup)
	}
}
