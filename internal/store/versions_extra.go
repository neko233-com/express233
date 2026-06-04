package store

import "database/sql"

// PublishedVersionByOffset 已发布版本列表中第 offset 个（0=最新，1=上一版）。
func (s *Store) PublishedVersionByOffset(projectID int64, offset int) (string, error) {
	if offset < 0 {
		offset = 0
	}
	var ver string
	err := s.db.QueryRow(
		`SELECT version FROM versions WHERE project_id = ? AND status = 'published'
		 ORDER BY published_at DESC LIMIT 1 OFFSET ?`,
		projectID, offset,
	).Scan(&ver)
	if err == sql.ErrNoRows {
		return "", err
	}
	return ver, err
}

// CountPublishedVersions 已发布版本数量。
func (s *Store) CountPublishedVersions(projectID int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM versions WHERE project_id = ? AND status = 'published'`,
		projectID,
	).Scan(&n)
	return n, err
}
