package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PushDeploymentTask is a reusable release selector. Executions snapshot its
// name and selector into PushDeployment so deleting a task never deletes logs.
type PushDeploymentTask struct {
	ID        int64    `json:"id"`
	TenantID  int64    `json:"tenant_id,omitempty"`
	ProjectID int64    `json:"project_id"`
	Name      string   `json:"name"`
	Version   string   `json:"version,omitempty"`
	ServerIDs []string `json:"server_ids"`
	Tags      []string `json:"tags"`
	TagMatch  string   `json:"tag_match"`
	CreatedBy string   `json:"created_by,omitempty"`
	RunCount  int64    `json:"run_count"`
	LastRunAt string   `json:"last_run_at,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

func normalizeTaskValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizePushTask(task *PushDeploymentTask) error {
	task.Name = strings.TrimSpace(task.Name)
	task.Version = strings.TrimSpace(task.Version)
	if task.Name == "" || len(task.Name) > 100 {
		return fmt.Errorf("task name is required and must not exceed 100 characters")
	}
	if task.TagMatch == "" {
		task.TagMatch = "all"
	}
	if task.TagMatch != "all" && task.TagMatch != "any" {
		return fmt.Errorf("tag_match must be all or any")
	}
	task.ServerIDs = normalizeTaskValues(task.ServerIDs)
	task.Tags = normalizeTaskValues(task.Tags)
	if len(task.Tags) == 0 {
		task.Tags = []string{"test"}
	}
	return nil
}

func taskJSON(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func scanPushTask(scanner pushHostScanner, task *PushDeploymentTask) error {
	var serverIDs, tags string
	if err := scanner.Scan(&task.ID, &task.TenantID, &task.ProjectID, &task.Name, &task.Version, &serverIDs, &tags, &task.TagMatch, &task.CreatedBy, &task.RunCount, &task.LastRunAt, &task.CreatedAt, &task.UpdatedAt); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(serverIDs), &task.ServerIDs); err != nil {
		return fmt.Errorf("decode task server_ids: %w", err)
	}
	if err := json.Unmarshal([]byte(tags), &task.Tags); err != nil {
		return fmt.Errorf("decode task tags: %w", err)
	}
	return nil
}

const pushTaskColumns = `id,tenant_id,project_id,name,version,server_ids,tags,tag_match,created_by,run_count,last_run_at,created_at,updated_at`

func (s *Store) CreatePushDeploymentTask(task PushDeploymentTask) (*PushDeploymentTask, error) {
	if err := normalizePushTask(&task); err != nil {
		return nil, err
	}
	now := time.Now().Format(timeLayout)
	res, err := s.db.Exec(`INSERT INTO push_deployment_tasks(tenant_id,project_id,name,version,server_ids,tags,tag_match,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, task.TenantID, task.ProjectID, task.Name, task.Version, taskJSON(task.ServerIDs), taskJSON(task.Tags), task.TagMatch, task.CreatedBy, now, now)
	if err != nil {
		return nil, err
	}
	task.ID, _ = res.LastInsertId()
	task.CreatedAt, task.UpdatedAt = now, now
	return &task, nil
}

func (s *Store) ListPushDeploymentTasks(tenantID, projectID int64) ([]PushDeploymentTask, error) {
	rows, err := s.db.Query(`SELECT `+pushTaskColumns+` FROM push_deployment_tasks WHERE tenant_id=? AND project_id=? ORDER BY id DESC`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]PushDeploymentTask, 0)
	for rows.Next() {
		var task PushDeploymentTask
		if err := scanPushTask(rows, &task); err != nil {
			return nil, err
		}
		items = append(items, task)
	}
	return items, rows.Err()
}

func (s *Store) GetPushDeploymentTask(tenantID, projectID, id int64) (*PushDeploymentTask, error) {
	var task PushDeploymentTask
	err := scanPushTask(s.db.QueryRow(`SELECT `+pushTaskColumns+` FROM push_deployment_tasks WHERE tenant_id=? AND project_id=? AND id=?`, tenantID, projectID, id), &task)
	return &task, err
}

func (s *Store) UpdatePushDeploymentTask(task PushDeploymentTask) error {
	if err := normalizePushTask(&task); err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE push_deployment_tasks SET name=?,version=?,server_ids=?,tags=?,tag_match=?,updated_at=? WHERE tenant_id=? AND project_id=? AND id=?`, task.Name, task.Version, taskJSON(task.ServerIDs), taskJSON(task.Tags), task.TagMatch, time.Now().Format(timeLayout), task.TenantID, task.ProjectID, task.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeletePushDeploymentTask(tenantID, projectID, id int64) error {
	var hooks int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM release_hooks WHERE tenant_id=? AND project_id=? AND task_id=?`, tenantID, projectID, id).Scan(&hooks); err != nil {
		return err
	}
	if hooks > 0 {
		return fmt.Errorf("release task is referenced by %d hook(s); delete the hooks first", hooks)
	}
	res, err := s.db.Exec(`DELETE FROM push_deployment_tasks WHERE tenant_id=? AND project_id=? AND id=?`, tenantID, projectID, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
