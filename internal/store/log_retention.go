package store

import "time"

// LogRetention is the single retention boundary for every operational log.
// Task definitions and release artifacts are not logs and are not pruned.
const LogRetention = 30 * 24 * time.Hour

type LogPruneResult struct {
	AuditLogs         int64 `json:"audit_logs"`
	UploadEvents      int64 `json:"upload_events"`
	ProjectLogs       int64 `json:"project_logs"`
	PushDeployments   int64 `json:"push_deployments"`
	PushHostChecks    int64 `json:"push_host_checks"`
	ReleaseHookEvents int64 `json:"release_hook_events"`
	LoginIPHistories  int64 `json:"login_ip_histories"`
}

// PruneLogs removes log records outside the 30-day rolling window in one
// transaction. Cascading deletion removes deployment target output together
// with its parent execution while reusable task definitions remain intact.
func (s *Store) PruneLogs(now time.Time) (LogPruneResult, error) {
	var out LogPruneResult
	tx, err := s.db.Begin()
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback() }()
	cutoff := now.Add(-LogRetention).Format(timeLayout)
	statements := []struct {
		query string
		args  []any
		count *int64
	}{
		{`DELETE FROM audit_logs WHERE at < ?`, []any{cutoff}, &out.AuditLogs},
		{`DELETE FROM upload_events WHERE at < ?`, []any{cutoff}, &out.UploadEvents},
		{`DELETE FROM project_logs WHERE at < ?`, []any{cutoff}, &out.ProjectLogs},
		{`DELETE FROM push_deployment_targets WHERE deployment_id IN (SELECT id FROM push_deployments WHERE created_at < ?)`, []any{cutoff}, new(int64)},
		{`DELETE FROM push_deployments WHERE created_at < ?`, []any{cutoff}, &out.PushDeployments},
		{`DELETE FROM push_host_checks WHERE checked_at < ?`, []any{cutoff}, &out.PushHostChecks},
		{`DELETE FROM release_hook_events WHERE created_at < ?`, []any{cutoff}, &out.ReleaseHookEvents},
		{`DELETE FROM login_ip_bans WHERE last_failure < ? AND (banned_until='' OR banned_until<=?)`, []any{now.Add(-LogRetention).UTC().Format(time.RFC3339Nano), now.UTC().Format(time.RFC3339Nano)}, &out.LoginIPHistories},
	}
	for _, statement := range statements {
		result, err := tx.Exec(statement.query, statement.args...)
		if err != nil {
			return out, err
		}
		*statement.count, _ = result.RowsAffected()
	}
	if err := tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}
