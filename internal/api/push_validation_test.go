package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestValidatePushHostRequiresPinnedHostKey(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ssh.NewPublicKey(private.Public())
	if err != nil {
		t.Fatal(err)
	}
	hostKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	if err := validatePushHostRequest(pushHostRequest{Name: "logic-a", Address: "10.0.0.1", Username: "deploy", AuthMode: "password", Credential: "safe-password", HostKey: hostKey}, true); err != nil {
		t.Fatalf("valid host rejected: %v", err)
	}
	if err := validatePushHostRequest(pushHostRequest{Name: "logic-a", Address: "10.0.0.1", Username: "deploy", AuthMode: "password", Credential: "safe-password"}, true); err == nil {
		t.Fatal("expected unpinned host to be rejected")
	}
}

func TestPushPublicURLRequiresTLS(t *testing.T) {
	t.Setenv("EXPRESS233_ALLOW_INSECURE_PUSH_URL", "")
	t.Setenv("EXPRESS233_PUBLIC_URL", "")
	if _, err := pushPublicURL(); err == nil {
		t.Fatal("missing public URL accepted")
	}
	t.Setenv("EXPRESS233_PUBLIC_URL", "http://controller.example")
	if _, err := pushPublicURL(); err == nil {
		t.Fatal("insecure public URL accepted")
	}
	t.Setenv("EXPRESS233_PUBLIC_URL", "https://controller.example")
	if got, err := pushPublicURL(); err != nil || got != "https://controller.example" {
		t.Fatalf("url=%q err=%v", got, err)
	}
}
