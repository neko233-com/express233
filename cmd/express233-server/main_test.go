package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestRotatingFileWriterRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")
	writer, err := newRotatingFileWriter(path, 16, 2)
	if err != nil {
		t.Fatalf("new rotating writer: %v", err)
	}
	defer func() { _ = writer.Close() }()
	if _, err := writer.Write([]byte("1234567890\n")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := writer.Write([]byte("abcdefghij\n")); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated log file: %v", err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read current log: %v", err)
	}
	if !strings.Contains(string(current), "abcdefghij") {
		t.Fatalf("unexpected current log contents: %q", string(current))
	}
}

func TestUpdaterScriptContentIncludesRestart(t *testing.T) {
	content := updaterScriptContent("C:/bin/express233-server.exe", "C:/tmp/new.exe", "C:/data", "127.0.0.1:23380", 42, true)
	if !strings.Contains(content, "express233-server.exe") {
		t.Fatalf("expected target path in updater script: %q", content)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(content, " start -data ") {
			t.Fatalf("expected restart command in updater script: %q", content)
		}
		return
	}
	if !strings.Contains(content, " start -data ") {
		t.Fatalf("expected restart command in updater script: %q", content)
	}
}
