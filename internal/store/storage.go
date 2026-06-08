package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StorageCategory 磁盘占用分类。
type StorageCategory struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Bytes int64  `json:"bytes"`
}

// StorageOverview 存储总览。
type StorageOverview struct {
	DataDir          string            `json:"data_dir"`
	TotalBytes       int64             `json:"total_bytes"`
	AvailableBytes   int64             `json:"available_bytes,omitempty"`
	Categories       []StorageCategory `json:"categories"`
	BlobStats        BlobStats         `json:"blob_stats"`
	IndexEntryCount  int               `json:"index_entry_count"`
	IndexUpdatedAt   string            `json:"index_updated_at,omitempty"`
	ProjectCount     int               `json:"project_count"`
	VersionCount     int               `json:"version_count"`
}

// StorageTreeNode 存储树节点。
type StorageTreeNode struct {
	Name      string             `json:"name"`
	Path      string             `json:"path"`
	Kind      string             `json:"kind"`
	SizeBytes int64              `json:"size_bytes"`
	Meta      map[string]string  `json:"meta,omitempty"`
	Children  []StorageTreeNode  `json:"children,omitempty"`
}

// StorageDeletePlan 关联删除预览。
type StorageDeletePlan struct {
	Path       string   `json:"path"`
	Kind       string   `json:"kind"`
	SizeBytes  int64    `json:"size_bytes"`
	Allowed    bool     `json:"allowed"`
	DenyReason string   `json:"deny_reason,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	Related    []string `json:"related,omitempty"`
}

// StorageOverviewForTenant 计算租户存储总览。
func (s *Store) StorageOverviewForTenant(tenantID int64) (StorageOverview, error) {
	var ov StorageOverview
	ov.DataDir = s.dataDir
	ov.AvailableBytes = diskAvailable(s.dataDir)

	cats := []struct {
		name, label, path string
	}{
		{"userdata", "项目数据", filepath.Join(s.dataDir, "userdata")},
		{"blobs", "Blob 去重", s.blobsRoot()},
		{"database", "SQLite", filepath.Join(s.dataDir, "express233.db")},
		{"run", "运行态", filepath.Join(s.dataDir, "run")},
	}
	for _, c := range cats {
		var bytes int64
		if fi, err := os.Stat(c.path); err == nil {
			if fi.IsDir() {
				bytes = dirSize(c.path)
			} else {
				bytes = fi.Size()
			}
		}
		ov.Categories = append(ov.Categories, StorageCategory{Name: c.name, Label: c.label, Bytes: bytes})
		ov.TotalBytes += bytes
	}

	blobSt, err := s.BlobStats()
	if err != nil {
		return ov, err
	}
	ov.BlobStats = blobSt

	idx, err := s.StorageIndexMeta()
	if err != nil {
		return ov, err
	}
	ov.IndexEntryCount = idx.EntryCount
	ov.IndexUpdatedAt = idx.UpdatedAt

	projects, err := s.ListProjects(tenantID, 0, RoleAdmin)
	if err != nil {
		return ov, err
	}
	ov.ProjectCount = len(projects)
	for _, p := range projects {
		vers, err := s.ListVersions(p.ID)
		if err != nil {
			return ov, err
		}
		ov.VersionCount += len(vers)
	}
	return ov, nil
}

// StorageTreeAt 返回相对 dataDir 路径下的子树（空路径 = 租户根）。
func (s *Store) StorageTreeAt(tenantID int64, relPath string) (StorageTreeNode, error) {
	t, err := s.TenantByID(tenantID)
	if err != nil {
		return StorageTreeNode{}, err
	}
	relPath = strings.Trim(filepath.ToSlash(strings.TrimSpace(relPath)), "/")
	tenantPrefix := filepath.ToSlash(filepath.Join("userdata", t.Slug))
	if relPath == "" {
		relPath = tenantPrefix
	}
	if relPath != tenantPrefix && !strings.HasPrefix(relPath, tenantPrefix+"/") && relPath != "blobs" && !strings.HasPrefix(relPath, "blobs/") {
		return StorageTreeNode{}, fmt.Errorf("path out of tenant scope")
	}
	abs := filepath.Join(s.dataDir, filepath.FromSlash(relPath))
	if !pathWithin(s.dataDir, abs) {
		return StorageTreeNode{}, fmt.Errorf("invalid path")
	}
	return s.buildTreeNode(relPath, abs, tenantID)
}

func (s *Store) buildTreeNode(relPath, abs string, tenantID int64) (StorageTreeNode, error) {
	fi, err := os.Stat(abs)
	if err != nil {
		return StorageTreeNode{}, err
	}
	name := filepath.Base(abs)
	if relPath == "userdata" || strings.HasSuffix(relPath, "/userdata") {
		name = "userdata"
	}
	node := StorageTreeNode{
		Name:      name,
		Path:      relPath,
		SizeBytes: fi.Size(),
		Meta:      map[string]string{},
	}
	if fi.IsDir() {
		node.SizeBytes = dirSize(abs)
		node.Kind = s.inferTreeKind(relPath, true)
		entries, err := os.ReadDir(abs)
		if err != nil {
			return node, err
		}
		for _, e := range entries {
			if e.Name() == "." || e.Name() == ".." {
				continue
			}
			childRel := filepath.ToSlash(filepath.Join(relPath, e.Name()))
			childAbs := filepath.Join(abs, e.Name())
			child, err := s.buildTreeNode(childRel, childAbs, tenantID)
			if err != nil {
				continue
			}
			node.Children = append(node.Children, child)
		}
		s.enrichTreeMeta(&node, tenantID)
		return node, nil
	}
	node.Kind = s.inferTreeKind(relPath, false)
	return node, nil
}

func (s *Store) inferTreeKind(relPath string, isDir bool) string {
	parts := strings.Split(relPath, "/")
	if parts[0] == "blobs" {
		if isDir {
			return "folder"
		}
		return "blob"
	}
	if len(parts) >= 5 && parts[0] == "userdata" && parts[2] == "projects" {
		switch {
		case len(parts) == 3:
			return "folder"
		case len(parts) == 4 && isDir:
			return "project"
		case len(parts) == 5 && isDir:
			return "version"
		case !isDir:
			return "file"
		}
	}
	if isDir {
		return "folder"
	}
	return "file"
}

func (s *Store) enrichTreeMeta(node *StorageTreeNode, tenantID int64) {
	switch node.Kind {
	case "project":
		if p, err := s.GetProjectByName(tenantID, node.Name); err == nil {
			node.Meta["project_id"] = fmt.Sprintf("%d", p.ID)
			if vers, err := s.ListVersions(p.ID); err == nil {
				node.Meta["version_count"] = fmt.Sprintf("%d", len(vers))
			}
		}
	case "version":
		parts := strings.Split(node.Path, "/")
		if len(parts) >= 5 {
			projectName := parts[3]
			version := parts[4]
			node.Meta["project_name"] = projectName
			if p, err := s.GetProjectByName(tenantID, projectName); err == nil {
				for _, v := range mustListVersions(s, p.ID) {
					if v.Version == version {
						node.Meta["status"] = v.Status
						node.Meta["version_id"] = fmt.Sprintf("%d", v.ID)
						break
					}
				}
			}
		}
	}
	for i := range node.Children {
		s.enrichTreeMeta(&node.Children[i], tenantID)
	}
}

func mustListVersions(s *Store, projectID int64) []Version {
	vers, _ := s.ListVersions(projectID)
	return vers
}

// PlanStorageDelete 分析删除影响与权限。
func (s *Store) PlanStorageDelete(tenantID int64, relPath string, tenantRole string) (StorageDeletePlan, error) {
	relPath = strings.Trim(filepath.ToSlash(strings.TrimSpace(relPath)), "/")
	plan := StorageDeletePlan{Path: relPath}
	abs := filepath.Join(s.dataDir, filepath.FromSlash(relPath))
	if !pathWithin(s.dataDir, abs) {
		plan.DenyReason = "invalid path"
		return plan, nil
	}
	fi, err := os.Stat(abs)
	if err != nil {
		plan.DenyReason = "not found"
		return plan, nil
	}
	if fi.IsDir() {
		plan.SizeBytes = dirSize(abs)
	} else {
		plan.SizeBytes = fi.Size()
	}
	plan.Kind = s.inferTreeKind(relPath, fi.IsDir())

	t, err := s.TenantByID(tenantID)
	if err != nil {
		return plan, err
	}
	tenantPrefix := filepath.ToSlash(filepath.Join("userdata", t.Slug))
	if strings.HasPrefix(relPath, "blobs/") || relPath == "blobs" {
		if tenantRole != RoleAdmin {
			plan.DenyReason = "admin required for blob storage"
			return plan, nil
		}
		if plan.Kind == "orphan_blob" || plan.Kind == "blob" {
			hash := filepath.Base(abs)
			var refs int
			err := s.db.QueryRow(`SELECT ref_count FROM blobs WHERE hash = ?`, hash).Scan(&refs)
			if err == sql.ErrNoRows {
				plan.Allowed = true
				plan.Warnings = append(plan.Warnings, "删除未索引的 blob 文件")
				return plan, nil
			}
			if refs > 0 {
				plan.DenyReason = fmt.Sprintf("blob 仍被 %d 个版本文件引用", refs)
				plan.Related = append(plan.Related, fmt.Sprintf("ref_count=%d", refs))
				return plan, nil
			}
			plan.Allowed = true
			plan.Warnings = append(plan.Warnings, "删除孤立 blob 可释放磁盘")
			return plan, nil
		}
		plan.DenyReason = "cannot delete blob folder directly"
		return plan, nil
	}
	if !strings.HasPrefix(relPath, tenantPrefix) {
		plan.DenyReason = "path out of tenant scope"
		return plan, nil
	}

	parts := strings.Split(relPath, "/")
	switch plan.Kind {
	case "version":
		if len(parts) < 5 {
			plan.DenyReason = "invalid version path"
			return plan, nil
		}
		projectName, version := parts[3], parts[4]
		p, err := s.GetProjectByName(tenantID, projectName)
		if err != nil {
			plan.DenyReason = "project not found"
			return plan, nil
		}
		vers, _ := s.ListVersions(p.ID)
		status := ""
		for _, v := range vers {
			if v.Version == version {
				status = v.Status
				break
			}
		}
		plan.Related = append(plan.Related, "project="+projectName)
		if status == "published" {
			plan.Warnings = append(plan.Warnings, "已发布版本删除后节点无法再拉取此版本")
		}
		plan.Allowed = true
		return plan, nil
	case "project":
		projectName := parts[3]
		if vers, err := s.ListVersions(mustProjectID(s, tenantID, projectName)); err == nil {
			plan.Related = append(plan.Related, fmt.Sprintf("versions=%d", len(vers)))
			for _, v := range vers {
				plan.Related = append(plan.Related, v.Version+" ("+v.Status+")")
			}
		}
		plan.Warnings = append(plan.Warnings, "将删除项目下全部版本与文件")
		plan.Allowed = true
		return plan, nil
	case "file":
		if len(parts) < 6 {
			plan.DenyReason = "invalid file path"
			return plan, nil
		}
		projectName, version := parts[3], parts[4]
		fileRel := strings.Join(parts[5:], "/")
		for _, v := range mustListVersions(s, mustProjectID(s, tenantID, projectName)) {
			if v.Version == version && v.Status != "draft" {
				plan.DenyReason = "only draft version files can be deleted"
				return plan, nil
			}
		}
		plan.Related = append(plan.Related, "file="+fileRel)
		plan.Allowed = true
		return plan, nil
	default:
		plan.DenyReason = "unsupported delete target"
		return plan, nil
	}
}

func mustProjectID(s *Store, tenantID int64, name string) int64 {
	p, err := s.GetProjectByName(tenantID, name)
	if err != nil {
		return 0
	}
	return p.ID
}

// ExecuteStorageDelete 执行经确认的关联删除。
func (s *Store) ExecuteStorageDelete(tenantID int64, relPath string) error {
	plan, err := s.PlanStorageDelete(tenantID, relPath, RoleAdmin)
	if err != nil {
		return err
	}
	if !plan.Allowed {
		if plan.DenyReason != "" {
			return fmt.Errorf("%s", plan.DenyReason)
		}
		return fmt.Errorf("delete not allowed")
	}
	abs := filepath.Join(s.dataDir, filepath.FromSlash(relPath))
	parts := strings.Split(relPath, "/")

	switch plan.Kind {
	case "version":
		projectName, version := parts[3], parts[4]
		p, err := s.GetProjectByName(tenantID, projectName)
		if err != nil {
			return err
		}
		return s.DeleteVersion(tenantID, p.ID, projectName, version)
	case "project":
		projectName := parts[3]
		p, err := s.GetProjectByName(tenantID, projectName)
		if err != nil {
			return err
		}
		return s.DeleteProject(tenantID, p.ID)
	case "file":
		projectName, version := parts[3], parts[4]
		fileRel := strings.Join(parts[5:], "/")
		return s.DeleteVersionFile(tenantID, projectName, version, fileRel)
	case "blob", "orphan_blob":
		hash := filepath.Base(abs)
		if err := os.Remove(abs); err != nil {
			return err
		}
		_, _ = s.db.Exec(`DELETE FROM blobs WHERE hash = ? AND ref_count = 0`, hash)
		return nil
	default:
		return fmt.Errorf("unsupported kind %s", plan.Kind)
	}
}

func dirSize(root string) int64 {
	var total int64
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func pathWithin(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

