package store

// migrateExtra 增量 schema（兼容已有数据库）。
func (s *Store) migrateExtra() error {
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'operator'`)
	_, _ = s.db.Exec(`UPDATE users SET role = 'admin' WHERE is_admin = 1`)
	return nil
}
