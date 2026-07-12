package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// VCSProvenance is the immutable source identity attached to a versioned
// artifact. It intentionally stores an opaque commit rather than credentials;
// repository URLs containing user info are rejected.
type VCSProvenance struct {
	Provider   string `json:"provider,omitempty"`
	Repository string `json:"repository,omitempty"`
	Ref        string `json:"ref,omitempty"`
	Commit     string `json:"commit,omitempty"`
	RunURL     string `json:"run_url,omitempty"`
	RunID      string `json:"run_id,omitempty"`
}

// ArtifactManifest binds a published version to its exact sorted file content.
// SHA256 is a manifest digest, not an archive digest: it remains stable whether
// CI uploaded a zip, tarball, or individual files.
type ArtifactManifest struct {
	SHA256    string `json:"sha256,omitempty"`
	FileCount int64  `json:"file_count,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
}

const versionColumns = `id,project_id,version,status,created_at,COALESCE(published_at,''),vcs_provider,vcs_repository,vcs_ref,vcs_commit,vcs_run_url,vcs_run_id,artifact_sha256,artifact_file_count,artifact_bytes`

var vcsCommitRE = regexp.MustCompile(`^[A-Fa-f0-9]{7,128}$`)
var vcsRefRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@:+-]{0,255}$`)

func (s *Store) migrateVersionProvenance() error {
	// ALTER is intentionally best-effort: SQLite lacks ADD COLUMN IF NOT EXISTS
	// and fresh installations already have these columns in the base schema.
	for _, statement := range []string{
		`ALTER TABLE versions ADD COLUMN vcs_provider TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE versions ADD COLUMN vcs_repository TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE versions ADD COLUMN vcs_ref TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE versions ADD COLUMN vcs_commit TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE versions ADD COLUMN vcs_run_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE versions ADD COLUMN vcs_run_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE versions ADD COLUMN artifact_sha256 TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE versions ADD COLUMN artifact_file_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE versions ADD COLUMN artifact_bytes INTEGER NOT NULL DEFAULT 0`,
	} {
		_, _ = s.db.Exec(statement)
	}
	return nil
}

func scanVersion(scanner interface{ Scan(...any) error }, version *Version) error {
	return scanner.Scan(&version.ID, &version.ProjectID, &version.Version, &version.Status, &version.CreatedAt, &version.PublishedAt, &version.VCS.Provider, &version.VCS.Repository, &version.VCS.Ref, &version.VCS.Commit, &version.VCS.RunURL, &version.VCS.RunID, &version.Artifact.SHA256, &version.Artifact.FileCount, &version.Artifact.Bytes)
}

// ValidateVCSProvenance validates metadata before a CI upload begins. An empty
// value remains valid for manually created drafts, but partial provenance is
// rejected so the UI never presents an ambiguous source identity.
func ValidateVCSProvenance(vcs VCSProvenance) error {
	vcs.Provider, vcs.Repository, vcs.Ref, vcs.Commit = strings.TrimSpace(vcs.Provider), strings.TrimSpace(vcs.Repository), strings.TrimSpace(vcs.Ref), strings.TrimSpace(vcs.Commit)
	vcs.RunURL, vcs.RunID = strings.TrimSpace(vcs.RunURL), strings.TrimSpace(vcs.RunID)
	if vcs.Provider == "" && vcs.Repository == "" && vcs.Ref == "" && vcs.Commit == "" && vcs.RunURL == "" && vcs.RunID == "" {
		return nil
	}
	if vcs.Provider != "git" && vcs.Provider != "github" && vcs.Provider != "gitea" {
		return fmt.Errorf("vcs.provider must be git, github, or gitea")
	}
	if vcs.Repository == "" || vcs.Ref == "" || !vcsCommitRE.MatchString(vcs.Commit) {
		return fmt.Errorf("vcs.repository, vcs.ref, and a 7-128 character hexadecimal vcs.commit are required")
	}
	if !vcsRefRE.MatchString(vcs.Ref) || len(vcs.Repository) > 512 || len(vcs.RunID) > 128 {
		return fmt.Errorf("invalid vcs reference metadata")
	}
	repo, err := url.Parse(vcs.Repository)
	if err != nil || repo.Scheme != "https" || repo.Host == "" || repo.User != nil {
		return fmt.Errorf("vcs.repository must be a credential-free HTTPS URL")
	}
	if vcs.RunURL != "" {
		run, err := url.Parse(vcs.RunURL)
		if err != nil || run.Scheme != "https" || run.Host == "" || run.User != nil {
			return fmt.Errorf("vcs.run_url must be a credential-free HTTPS URL")
		}
	}
	return nil
}

// SetVersionVCSProvenance may only change a draft. Publication snapshots both
// source provenance and manifest so later CI retries cannot rewrite history.
func (s *Store) SetVersionVCSProvenance(projectID int64, version string, vcs VCSProvenance) error {
	if err := ValidateVCSProvenance(vcs); err != nil {
		return err
	}
	result, err := s.db.Exec(`UPDATE versions SET vcs_provider=?,vcs_repository=?,vcs_ref=?,vcs_commit=?,vcs_run_url=?,vcs_run_id=? WHERE project_id=? AND version=? AND status='draft'`, strings.TrimSpace(vcs.Provider), strings.TrimSpace(vcs.Repository), strings.TrimSpace(vcs.Ref), strings.TrimSpace(vcs.Commit), strings.TrimSpace(vcs.RunURL), strings.TrimSpace(vcs.RunID), projectID, version)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// BuildArtifactManifest produces a canonical content manifest. Paths, mode,
// size and every file digest participate, preventing a path-only or timestamp
// based collision from being recorded as the same production artifact.
func BuildArtifactManifest(root string) (ArtifactManifest, error) {
	paths := make([]string, 0)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return ArtifactManifest{}, err
	}
	sort.Strings(paths)
	digest := sha256.New()
	manifest := ArtifactManifest{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return ArtifactManifest{}, err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return ArtifactManifest{}, err
		}
		file, err := os.Open(path)
		if err != nil {
			return ArtifactManifest{}, err
		}
		fileDigest := sha256.New()
		_, copyErr := io.Copy(fileDigest, file)
		closeErr := file.Close()
		if copyErr != nil {
			return ArtifactManifest{}, copyErr
		}
		if closeErr != nil {
			return ArtifactManifest{}, closeErr
		}
		// hash.Hash writes cannot fail; keep the assignment explicit for errcheck.
		_, _ = fmt.Fprintf(digest, "%s\x00%o\x00%d\x00%s\n", filepath.ToSlash(rel), info.Mode().Perm(), info.Size(), hex.EncodeToString(fileDigest.Sum(nil)))
		manifest.FileCount++
		manifest.Bytes += info.Size()
	}
	manifest.SHA256 = hex.EncodeToString(digest.Sum(nil))
	return manifest, nil
}
