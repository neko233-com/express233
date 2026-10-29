package scripts

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIInstallerValidatesAllArtifactsBeforeReplacingAnyFile(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Linux amd64 node installer fixture")
	}
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			root := t.TempDir()
			artifacts := filepath.Join(root, "artifacts")
			bin := filepath.Join(root, "bin")
			installed := filepath.Join(root, "installed")
			for _, dir := range []string{artifacts, bin, installed} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			manifest := ""
			for _, name := range []string{"express233-cli-linux-amd64", "safe-deploy.sh", "batch-deploy.sh"} {
				content := []byte("#!/bin/bash\nexit 0\n")
				digest := sha256.Sum256(content)
				manifest += fmt.Sprintf("%x  dist/%s\n", digest, name)
				if corrupt && name == "safe-deploy.sh" {
					content = []byte("tampered")
				}
				if err := os.WriteFile(filepath.Join(artifacts, name), content, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(artifacts, "SHA256SUMS"), []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			curl := `#!/bin/bash
set -eu
url=""; target=""
while [[ $# -gt 0 ]]; do
  case "$1" in -o) target="$2"; shift 2;; https:*) url="$1"; shift;; *) shift;; esac
done
cp "$FIXTURE_ARTIFACTS/${url##*/}" "$target"
`
			if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(curl), 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "install.sh", "1.3.0")
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "FIXTURE_ARTIFACTS="+artifacts, "EXPRESS233_INSTALL_DIR="+installed, "TMPDIR="+root)
			output, err := cmd.CombinedOutput()
			if corrupt {
				entries, _ := os.ReadDir(installed)
				if err == nil || len(entries) != 0 || !strings.Contains(string(output), "checksum mismatch") {
					t.Fatalf("tampered assets installed: %v\n%s", err, output)
				}
			} else {
				if err != nil {
					t.Fatalf("verified install failed: %v\n%s", err, output)
				}
				for _, name := range []string{"express233-cli", "safe-deploy.sh", "batch-deploy.sh"} {
					if _, err := os.Stat(filepath.Join(installed, name)); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
