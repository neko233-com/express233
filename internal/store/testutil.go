package store

// TestRootUserID 返回测试库中 root 用户 ID。
func (s *Store) TestRootUserID() int64 {
	users, err := s.ListUsers(1)
	if err != nil || len(users) == 0 {
		return 1
	}
	for _, u := range users {
		if u.Username == "root" {
			return u.ID
		}
	}
	return users[0].ID
}
