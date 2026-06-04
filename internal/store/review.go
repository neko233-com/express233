package store

import "fmt"

// SubmitVersionReview 草稿/驳回 -> 待审批。
func (s *Store) SubmitVersionReview(projectID int64, projectName, version string) error {
	st, err := s.getVersionStatus(projectID, version)
	if err != nil {
		return err
	}
	if st != "draft" && st != "rejected" {
		return fmt.Errorf("only draft or rejected versions can be submitted")
	}
	res, err := s.db.Exec(
		`UPDATE versions SET status = 'pending_review' WHERE project_id = ? AND version = ?`,
		projectID, version,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("version not found")
	}
	return nil
}

// RejectVersionReview 待审批 -> 驳回。
func (s *Store) RejectVersionReview(projectID int64, version string) error {
	st, err := s.getVersionStatus(projectID, version)
	if err != nil {
		return err
	}
	if st != "pending_review" {
		return fmt.Errorf("version is not pending review")
	}
	_, err = s.db.Exec(
		`UPDATE versions SET status = 'rejected' WHERE project_id = ? AND version = ?`,
		projectID, version,
	)
	return err
}

func (s *Store) getVersionStatus(projectID int64, version string) (string, error) {
	var st string
	err := s.db.QueryRow(
		`SELECT status FROM versions WHERE project_id = ? AND version = ?`, projectID, version,
	).Scan(&st)
	return st, err
}

func (s *Store) assertMutable(tenantID int64, projectName, version string) error {
	p, err := s.GetProjectByName(tenantID, projectName)
	if err != nil {
		return err
	}
	st, err := s.getVersionStatus(p.ID, version)
	if err != nil {
		return err
	}
	switch st {
	case "draft", "rejected":
		return nil
	case "pending_review":
		return fmt.Errorf("version is pending review and immutable")
	default:
		return fmt.Errorf("version is published and immutable")
	}
}
