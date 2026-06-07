package main

import (
	"path/filepath"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestNormalizePortValue(t *testing.T) {
	got, err := normalizePortValue("127.0.0.1:23380", "32380")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got != "127.0.0.1:32380" {
		t.Fatalf("unexpected addr: %s", got)
	}

	got, err = normalizePortValue("127.0.0.1:23380", ":33333")
	if err != nil {
		t.Fatalf("normalize explicit: %v", err)
	}
	if got != "127.0.0.1:33333" {
		t.Fatalf("unexpected explicit addr: %s", got)
	}
}

func TestRuntimeConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := runtimeConfig{Addr: "127.0.0.1:24444"}
	if err := saveRuntimeConfig(dir, want); err != nil {
		t.Fatalf("save runtime config: %v", err)
	}
	got, err := loadRuntimeConfig(dir)
	if err != nil {
		t.Fatalf("load runtime config: %v", err)
	}
	if got.Addr != want.Addr {
		t.Fatalf("runtime config mismatch: %#v", got)
	}
}

func TestBackupAndRestoreServerConfig(t *testing.T) {
	dir := t.TempDir()
	path, err := serverConfigPath(dir)
	if err != nil {
		t.Fatalf("server config path: %v", err)
	}
	seed := filepath.Join("..", "..", "configs", "server.yaml.example")
	if err := copyFile(seed, path); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	backup, err := backupServerConfig(dir)
	if err != nil {
		t.Fatalf("backup config: %v", err)
	}
	if backup == "" {
		t.Fatal("backup path empty")
	}
	if err := copyFile(filepath.Join("..", "..", "configs", "post-hook.yaml.example"), path); err != nil {
		t.Fatalf("mutate config: %v", err)
	}
	restored, err := restoreServerConfig(dir, false)
	if err != nil {
		t.Fatalf("restore config: %v", err)
	}
	if restored == "" {
		t.Fatal("restore source empty")
	}
	if err := validateServerConfig(dir); err != nil {
		t.Fatalf("restored config invalid: %v", err)
	}
}

func TestResetRootPassword(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()

	if _, _, err := st.Authenticate("root", "root"); err != nil {
		t.Fatalf("default root auth: %v", err)
	}
	if err := resetRootPassword(dir, "new-secret"); err != nil {
		t.Fatalf("reset root password: %v", err)
	}
	if _, _, err := st.Authenticate("root", "root"); err == nil {
		t.Fatal("old password should be rejected")
	}
	if _, _, err := st.Authenticate("root", "new-secret"); err != nil {
		t.Fatalf("new root auth: %v", err)
	}
}