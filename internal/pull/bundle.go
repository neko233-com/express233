package pull

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/filetags"
	"github.com/neko233-com/express233/internal/hookspec"
	"github.com/neko233-com/express233/internal/store"
	"github.com/neko233-com/express233/internal/template"
)

// Manifest 随包下发的拉取元数据。
type Manifest struct {
	Project      string            `json:"project"`
	Version      string            `json:"version"`
	ServerID     string            `json:"server_id"`
	OS           string            `json:"os,omitempty"`
	Arch         string            `json:"arch,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	PostHook     string            `json:"post_hook"`
	PostHookEnv  map[string]string `json:"post_hook_env"`
	PostHookSpec string            `json:"post_hook_spec,omitempty"`
	PostHookPlan []string          `json:"post_hook_plan,omitempty"`
}

type BundleOptions struct {
	OS   string
	Arch string
	Tags []string
}

// BuildBundle 复制版本目录、应用 server.yaml 替换并打成 tar.gz。
func BuildBundle(st *store.Store, tenantID int64, sf *config.ServerFile, projectName, version, serverID string, w io.Writer) error {
	return BuildBundleWithOptions(st, tenantID, sf, projectName, version, serverID, BundleOptions{}, w)
}

// BuildBundleWithOptions 复制匹配标签的文件、应用 server.yaml 替换并打成 tar.gz。
func BuildBundleWithOptions(st *store.Store, tenantID int64, sf *config.ServerFile, projectName, version, serverID string, opts BundleOptions, w io.Writer) error {
	vdir, err := st.VersionDir(tenantID, projectName, version)
	if err != nil {
		return err
	}
	if _, err := os.Stat(vdir); err != nil {
		return fmt.Errorf("version files: %w", err)
	}
	tagRows, err := st.ListVersionFileTags(tenantID, projectName, version)
	if err != nil {
		return err
	}
	tagByPath := make(map[string][]string, len(tagRows))
	for _, row := range tagRows {
		tagByPath[row.Path] = row.Tags
	}
	target := filetags.Target(opts.OS, opts.Arch, opts.Tags)
	filter := func(rel string) bool {
		return filetags.Matches(tagByPath[filepath.ToSlash(rel)], target)
	}
	return buildBundleFromVersionDir(vdir, sf, projectName, version, serverID, opts, filter, w)
}

// BuildBundleFromDir 从本地版本目录构建包（测试/校验用）。
func BuildBundleFromDir(versionRoot string, sf *config.ServerFile, projectName, version, serverID string, w io.Writer) error {
	if _, err := os.Stat(versionRoot); err != nil {
		return fmt.Errorf("version files: %w", err)
	}
	return buildBundleFromVersionDir(versionRoot, sf, projectName, version, serverID, BundleOptions{}, nil, w)
}

func buildBundleFromVersionDir(vdir string, sf *config.ServerFile, projectName, version, serverID string, opts BundleOptions, filter func(string) bool, w io.Writer) error {
	entry := sf.Entry(serverID)
	if entry == nil {
		return fmt.Errorf("unknown server_id %q in server.yaml", serverID)
	}

	tmp, err := os.MkdirTemp("", "express233-pull-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	if err := copyDir(vdir, tmp, filter); err != nil {
		return err
	}
	if len(entry.Replacements) > 0 {
		rep, err := config.PrepareReplacements(entry.Replacements)
		if err != nil {
			return fmt.Errorf("replacements: %w", err)
		}
		if err := template.ApplyByBasename(tmp, rep); err != nil {
			return fmt.Errorf("apply replacements: %w", err)
		}
	}

	gzw := gzip.NewWriter(w)
	defer func() { _ = gzw.Close() }()
	tw := tar.NewWriter(gzw)
	defer func() { _ = tw.Close() }()

	hookVars := template.HookTemplateVars(projectName, version, serverID, entry.PostHookEnv)
	manifest := Manifest{
		Project:     projectName,
		Version:     version,
		ServerID:    serverID,
		OS:          opts.OS,
		Arch:        opts.Arch,
		Tags:        filetags.Target(opts.OS, opts.Arch, opts.Tags),
		PostHook:    template.RenderHookTemplate(entry.PostHook, hookVars),
		PostHookEnv: entry.PostHookEnv,
	}
	if _, err := os.Stat(filepath.Join(tmp, filepath.FromSlash(hookspec.DefaultPath))); err == nil {
		manifest.PostHookSpec = hookspec.DefaultPath
		if plan, err := hookspec.PlanLines(tmp, hookspec.CurrentOS()); err == nil {
			manifest.PostHookPlan = plan
		}
	}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	if err := writeTarEntry(tw, ".express233/manifest.json", mb, time.Now()); err != nil {
		return err
	}

	return filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(tmp, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeTarEntry(tw, rel, data, info.ModTime())
	})
}

func copyDir(src, dst string, filter func(string) bool) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		relSlash := filepath.ToSlash(rel)
		if filter != nil && !filter(relSlash) {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

func writeTarEntry(tw *tar.Writer, name string, data []byte, mod time.Time) error {
	hdr := &tar.Header{
		Name:    filepath.ToSlash(name),
		Mode:    0o644,
		Size:    int64(len(data)),
		ModTime: mod,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// ExtractBundle 解压 tar.gz 到目标目录。
func ExtractBundle(r io.Reader, dest string) (*Manifest, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gzr.Close() }()
	tr := tar.NewReader(gzr)

	var manifest *Manifest
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		name := filepath.Clean(hdr.Name)
		if err := safeExtractPath(dest, name); err != nil {
			return nil, err
		}
		if hdr.Name == ".express233/manifest.json" {
			var m Manifest
			if err := json.NewDecoder(tr).Decode(&m); err != nil {
				return nil, err
			}
			manifest = &m
			continue
		}
		target := filepath.Join(dest, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
			return nil, err
		}
		_ = f.Close()
	}
	if manifest == nil {
		return nil, fmt.Errorf("missing manifest in bundle")
	}
	return manifest, nil
}

func safeExtractPath(dest, name string) error {
	target := filepath.Join(dest, filepath.FromSlash(name))
	cleanDest := filepath.Clean(dest)
	if !strings.HasPrefix(filepath.Clean(target), cleanDest+string(os.PathSeparator)) && filepath.Clean(target) != cleanDest {
		return fmt.Errorf("invalid tar path: %s", name)
	}
	return nil
}
