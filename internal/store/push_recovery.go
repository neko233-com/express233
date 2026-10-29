package store

import "time"

// InterruptPushDeployments 必须在独占控制器锁内启动调用，未知 SSH 结果不能自动重放。
func (s *Store) InterruptPushDeployments() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().Format(timeLayout)
	if _, err := tx.Exec(`UPDATE push_deployment_targets SET status=CASE WHEN status='running' THEN 'unknown' ELSE 'skipped' END,
output=output||CHAR(10)||'Controller interrupted; remote outcome requires verification; no automatic retry', completed_at=?
WHERE status IN ('running','queued') AND deployment_id IN (SELECT id FROM push_deployments WHERE status IN ('running','queued'))`, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE push_deployments SET status='failed',completed_at=? WHERE status IN ('running','queued')`, now); err != nil {
		return err
	}
	return tx.Commit()
}
