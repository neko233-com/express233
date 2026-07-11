package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

const dashboardRetention = 400 * 24 * time.Hour

// UploadEvent is a normalized, tenant-scoped record of one upload request.
// Archive bytes are the transferred archive size; file_count is the number of
// extracted files. Legacy audit backfills have zero for values not recorded.
type UploadEvent struct {
	ID            int64  `json:"id"`
	At            string `json:"at"`
	TenantID      int64  `json:"tenant_id,omitempty"`
	ProjectID     int64  `json:"project_id"`
	Version       string `json:"version"`
	Kind          string `json:"kind"`
	Bytes         int64  `json:"bytes"`
	FileCount     int64  `json:"file_count"`
	Status        string `json:"status"`
	Username      string `json:"username,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Error         string `json:"error,omitempty"`
	IP            string `json:"ip,omitempty"`
	SourceAuditID int64  `json:"-"`
}

type DashboardDay struct {
	Date                string `json:"date"`
	Uploads             int64  `json:"uploads"`
	UploadFailures      int64  `json:"upload_failures"`
	UploadBytes         int64  `json:"upload_bytes"`
	UploadedFiles       int64  `json:"uploaded_files"`
	Publishes           int64  `json:"publishes"`
	Pulls               int64  `json:"pulls"`
	PullFailures        int64  `json:"pull_failures"`
	Deployments         int64  `json:"deployments"`
	DeploymentSuccesses int64  `json:"deployment_successes"`
	DeploymentFailures  int64  `json:"deployment_failures"`
	Targets             int64  `json:"targets"`
	TargetFailures      int64  `json:"target_failures"`
	DeploymentMillis    int64  `json:"deployment_millis"`
	CompletedDeploys    int64  `json:"completed_deploys"`
}

type DashboardSummary struct {
	Uploads                 int64   `json:"uploads"`
	UploadFailures          int64   `json:"upload_failures"`
	UploadBytes             int64   `json:"upload_bytes"`
	UploadedFiles           int64   `json:"uploaded_files"`
	Publishes               int64   `json:"publishes"`
	Pulls                   int64   `json:"pulls"`
	PullFailures            int64   `json:"pull_failures"`
	PullSuccessRate         float64 `json:"pull_success_rate"`
	Deployments             int64   `json:"deployments"`
	DeploymentSuccesses     int64   `json:"deployment_successes"`
	DeploymentFailures      int64   `json:"deployment_failures"`
	DeploymentSuccessRate   float64 `json:"deployment_success_rate"`
	Targets                 int64   `json:"targets"`
	TargetFailures          int64   `json:"target_failures"`
	AverageDeploymentMillis int64   `json:"average_deployment_millis"`
}

type DashboardRecord struct {
	At        string `json:"at"`
	Kind      string `json:"kind"`
	ProjectID int64  `json:"project_id"`
	Project   string `json:"project"`
	Version   string `json:"version,omitempty"`
	Status    string `json:"status"`
	Actor     string `json:"actor,omitempty"`
	ServerID  string `json:"server_id,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	Files     int64  `json:"files,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type DashboardSnapshot struct {
	GeneratedAt string            `json:"generated_at"`
	Days        int               `json:"days"`
	Summary     DashboardSummary  `json:"summary"`
	Series      []DashboardDay    `json:"series"`
	Recent      []DashboardRecord `json:"recent"`
}

func (s *Store) migrateDashboard() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS upload_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  at TEXT NOT NULL,
  tenant_id INTEGER NOT NULL,
  project_id INTEGER NOT NULL,
  version TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT 'file',
  bytes INTEGER NOT NULL DEFAULT 0,
  file_count INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'success',
  username TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT '',
  source_audit_id INTEGER UNIQUE,
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_upload_events_tenant_at ON upload_events(tenant_id, at DESC);
CREATE INDEX IF NOT EXISTS idx_upload_events_project_at ON upload_events(project_id, at DESC);
CREATE INDEX IF NOT EXISTS idx_upload_events_at ON upload_events(at);
CREATE INDEX IF NOT EXISTS idx_versions_project_published ON versions(project_id, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_project_logs_project_action_at ON project_logs(project_id, action, at DESC);
CREATE INDEX IF NOT EXISTS idx_project_logs_at ON project_logs(at);
CREATE INDEX IF NOT EXISTS idx_push_deployments_project_created ON push_deployments(project_id, created_at DESC);
`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM upload_events WHERE at < ?`, time.Now().Add(-dashboardRetention).Format(timeLayout)); err != nil {
		return err
	}
	return s.backfillLegacyUploads()
}

// Audit logs predating upload_events have no tenant_id. Backfill them only for
// a single-tenant database, where attribution is unambiguous.
func (s *Store) backfillLegacyUploads() error {
	var tenantCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&tenantCount); err != nil || tenantCount != 1 {
		return err
	}
	var tenantID int64
	if err := s.db.QueryRow(`SELECT id FROM tenants LIMIT 1`).Scan(&tenantID); err != nil {
		return err
	}
	rows, err := s.db.Query(`SELECT a.id,a.at,a.username,a.action,a.detail,a.ip
FROM audit_logs a LEFT JOIN upload_events u ON u.source_audit_id=a.id
WHERE a.action LIKE 'version.upload%' AND u.id IS NULL ORDER BY a.id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type legacy struct {
		id                               int64
		at, username, action, detail, ip string
	}
	var entries []legacy
	for rows.Next() {
		var e legacy
		if err := rows.Scan(&e.id, &e.at, &e.username, &e.action, &e.detail, &e.ip); err != nil {
			return err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, e := range entries {
		project := auditDetailValue(e.detail, "project")
		version := auditDetailValue(e.detail, "version")
		var projectID int64
		if project == "" || s.db.QueryRow(`SELECT id FROM projects WHERE tenant_id=? AND name=?`, tenantID, project).Scan(&projectID) != nil {
			continue
		}
		kind := strings.TrimPrefix(e.action, "version.upload.")
		if kind == e.action {
			kind = "file"
		}
		_, err := s.db.Exec(`INSERT OR IGNORE INTO upload_events(at,tenant_id,project_id,version,kind,status,username,detail,ip,source_audit_id) VALUES(?,?,?,?,?,'success',?,?,?,?)`, e.at, tenantID, projectID, version, kind, e.username, e.detail, e.ip, e.id)
		if err != nil {
			return err
		}
	}
	return nil
}

func auditDetailValue(detail, key string) string {
	prefix := key + "="
	for _, part := range strings.Fields(detail) {
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}

func (s *Store) RecordUploadEvent(event UploadEvent) error {
	if event.At == "" {
		event.At = time.Now().Format(timeLayout)
	}
	if event.Kind == "" {
		event.Kind = "file"
	}
	if event.Status == "" {
		event.Status = "success"
	}
	if len(event.Error) > 1000 {
		event.Error = event.Error[:1000]
	}
	if _, err := s.db.Exec(`DELETE FROM upload_events WHERE at < ?`, time.Now().Add(-dashboardRetention).Format(timeLayout)); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO upload_events(at,tenant_id,project_id,version,kind,bytes,file_count,status,username,detail,error,ip) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, event.At, event.TenantID, event.ProjectID, event.Version, event.Kind, event.Bytes, event.FileCount, event.Status, event.Username, event.Detail, event.Error, event.IP)
	return err
}

func dashboardPlaceholders(ids []int64) (string, []any) {
	parts := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		parts[i], args[i] = "?", id
	}
	return strings.Join(parts, ","), args
}

func (s *Store) Dashboard(projectIDs []int64, days int) (*DashboardSnapshot, error) {
	if days < 1 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	now := time.Now()
	startDay := now.AddDate(0, 0, -(days - 1))
	start := startDay.Format("2006-01-02") + " 00:00:00"
	snapshot := &DashboardSnapshot{GeneratedAt: now.Format(time.RFC3339), Days: days, Series: make([]DashboardDay, days), Recent: []DashboardRecord{}}
	byDate := make(map[string]*DashboardDay, days)
	for i := 0; i < days; i++ {
		date := startDay.AddDate(0, 0, i).Format("2006-01-02")
		snapshot.Series[i].Date = date
		byDate[date] = &snapshot.Series[i]
	}
	if len(projectIDs) == 0 {
		return snapshot, nil
	}
	in, idArgs := dashboardPlaceholders(projectIDs)
	queryDaily := func(query string, scan func(*sql.Rows) error, args ...any) error {
		rows, err := s.db.Query(query, args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			if err := scan(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	args := append([]any{start}, idArgs...)
	if err := queryDaily(fmt.Sprintf(`SELECT substr(at,1,10),COUNT(*),COALESCE(SUM(CASE WHEN status!='success' THEN 1 ELSE 0 END),0),COALESCE(SUM(bytes),0),COALESCE(SUM(file_count),0) FROM upload_events WHERE at>=? AND project_id IN (%s) GROUP BY substr(at,1,10)`, in), func(rows *sql.Rows) error {
		var date string
		var uploads, failures, bytes, files int64
		if err := rows.Scan(&date, &uploads, &failures, &bytes, &files); err != nil {
			return err
		}
		if day := byDate[date]; day != nil {
			day.Uploads, day.UploadFailures, day.UploadBytes, day.UploadedFiles = uploads, failures, bytes, files
		}
		return nil
	}, args...); err != nil {
		return nil, err
	}
	if err := queryDaily(fmt.Sprintf(`SELECT substr(published_at,1,10),COUNT(*) FROM versions WHERE published_at>=? AND project_id IN (%s) GROUP BY substr(published_at,1,10)`, in), func(rows *sql.Rows) error {
		var date string
		var count int64
		if err := rows.Scan(&date, &count); err != nil {
			return err
		}
		if day := byDate[date]; day != nil {
			day.Publishes = count
		}
		return nil
	}, args...); err != nil {
		return nil, err
	}
	if err := queryDaily(fmt.Sprintf(`SELECT substr(at,1,10),COUNT(*),COALESCE(SUM(CASE WHEN status NOT IN ('ok','success') THEN 1 ELSE 0 END),0) FROM project_logs WHERE at>=? AND action='pull' AND project_id IN (%s) GROUP BY substr(at,1,10)`, in), func(rows *sql.Rows) error {
		var date string
		var count, failures int64
		if err := rows.Scan(&date, &count, &failures); err != nil {
			return err
		}
		if day := byDate[date]; day != nil {
			day.Pulls, day.PullFailures = count, failures
		}
		return nil
	}, args...); err != nil {
		return nil, err
	}
	if err := queryDaily(fmt.Sprintf(`SELECT substr(created_at,1,10),COUNT(*),COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN completed_at!='' AND started_at!='' THEN (strftime('%%s',completed_at)-strftime('%%s',started_at))*1000 ELSE 0 END),0),COALESCE(SUM(CASE WHEN completed_at!='' AND started_at!='' THEN 1 ELSE 0 END),0) FROM push_deployments WHERE created_at>=? AND project_id IN (%s) GROUP BY substr(created_at,1,10)`, in), func(rows *sql.Rows) error {
		var date string
		var count, successes, failures, millis, completed int64
		if err := rows.Scan(&date, &count, &successes, &failures, &millis, &completed); err != nil {
			return err
		}
		if day := byDate[date]; day != nil {
			day.Deployments, day.DeploymentSuccesses, day.DeploymentFailures, day.DeploymentMillis, day.CompletedDeploys = count, successes, failures, millis, completed
		}
		return nil
	}, args...); err != nil {
		return nil, err
	}
	if err := queryDaily(fmt.Sprintf(`SELECT substr(t.created_at,1,10),COUNT(*),COALESCE(SUM(CASE WHEN t.status='failed' THEN 1 ELSE 0 END),0) FROM push_deployment_targets t JOIN push_deployments d ON d.id=t.deployment_id WHERE t.created_at>=? AND d.project_id IN (%s) GROUP BY substr(t.created_at,1,10)`, in), func(rows *sql.Rows) error {
		var date string
		var count, failures int64
		if err := rows.Scan(&date, &count, &failures); err != nil {
			return err
		}
		if day := byDate[date]; day != nil {
			day.Targets, day.TargetFailures = count, failures
		}
		return nil
	}, args...); err != nil {
		return nil, err
	}

	var deployMillis, completedDeploys int64
	for _, day := range snapshot.Series {
		s := &snapshot.Summary
		s.Uploads += day.Uploads
		s.UploadFailures += day.UploadFailures
		s.UploadBytes += day.UploadBytes
		s.UploadedFiles += day.UploadedFiles
		s.Publishes += day.Publishes
		s.Pulls += day.Pulls
		s.PullFailures += day.PullFailures
		s.Deployments += day.Deployments
		s.DeploymentSuccesses += day.DeploymentSuccesses
		s.DeploymentFailures += day.DeploymentFailures
		s.Targets += day.Targets
		s.TargetFailures += day.TargetFailures
		deployMillis += day.DeploymentMillis
		completedDeploys += day.CompletedDeploys
	}
	if snapshot.Summary.Pulls > 0 {
		snapshot.Summary.PullSuccessRate = float64(snapshot.Summary.Pulls-snapshot.Summary.PullFailures) * 100 / float64(snapshot.Summary.Pulls)
	}
	completed := snapshot.Summary.DeploymentSuccesses + snapshot.Summary.DeploymentFailures
	if completed > 0 {
		snapshot.Summary.DeploymentSuccessRate = float64(snapshot.Summary.DeploymentSuccesses) * 100 / float64(completed)
	}
	if completedDeploys > 0 {
		snapshot.Summary.AverageDeploymentMillis = deployMillis / completedDeploys
	}

	recordArgs := make([]any, 0, len(idArgs)*4+4)
	for i := 0; i < 4; i++ {
		recordArgs = append(recordArgs, start)
		recordArgs = append(recordArgs, idArgs...)
	}
	recentQuery := fmt.Sprintf(`SELECT at,kind,project_id,project,version,status,actor,server_id,bytes,files,detail FROM (
SELECT u.at AS at,'upload' AS kind,u.project_id AS project_id,p.name AS project,u.version AS version,u.status AS status,u.username AS actor,'' AS server_id,u.bytes AS bytes,u.file_count AS files,CASE WHEN u.error!='' THEN u.error ELSE u.detail END AS detail FROM upload_events u JOIN projects p ON p.id=u.project_id WHERE u.at>=? AND u.project_id IN (%s)
UNION ALL SELECT v.published_at,'publish',v.project_id,p.name,v.version,'success','','',0,0,'version published' FROM versions v JOIN projects p ON p.id=v.project_id WHERE v.published_at>=? AND v.project_id IN (%s)
UNION ALL SELECT l.at,'pull',l.project_id,p.name,l.version,CASE WHEN l.status='ok' THEN 'success' ELSE l.status END,l.username,l.server_id,0,0,CASE WHEN l.error!='' THEN l.error ELSE l.os||'/'||l.arch END FROM project_logs l JOIN projects p ON p.id=l.project_id WHERE l.at>=? AND l.action='pull' AND l.project_id IN (%s)
UNION ALL SELECT d.created_at,'deploy',d.project_id,p.name,d.version,d.status,d.requested_by,'',0,(SELECT COUNT(*) FROM push_deployment_targets t WHERE t.deployment_id=d.id),d.selector FROM push_deployments d JOIN projects p ON p.id=d.project_id WHERE d.created_at>=? AND d.project_id IN (%s)
) ORDER BY at DESC LIMIT 100`, in, in, in, in)
	rows, err := s.db.Query(recentQuery, recordArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var rec DashboardRecord
		if err := rows.Scan(&rec.At, &rec.Kind, &rec.ProjectID, &rec.Project, &rec.Version, &rec.Status, &rec.Actor, &rec.ServerID, &rec.Bytes, &rec.Files, &rec.Detail); err != nil {
			return nil, err
		}
		snapshot.Recent = append(snapshot.Recent, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(snapshot.Recent, func(i, j int) bool { return snapshot.Recent[i].At > snapshot.Recent[j].At })
	return snapshot, nil
}
