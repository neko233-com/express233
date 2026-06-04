package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultAdminAndProjectLifecycle(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	uid, admin, err := st.Authenticate("root", "root")
	if err != nil || !admin || uid == 0 {
		t.Fatalf("root login: uid=%d admin=%v err=%v", uid, admin, err)
	}

	const tid int64 = 1
	p, err := st.CreateProject(tid, st.TestRootUserID(), "demo-game")
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "draft" {
		t.Fatalf("expected draft, got %s", v.Status)
	}

	content := strings.NewReader("bin=data")
	if err := st.WriteVersionFile(tid, p.Name, v.Version, "config/game.properties", content); err != nil {
		t.Fatal(err)
	}
	if err := st.PublishVersion(tid, p.ID, v.Version); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetVersion(p.ID, v.Version)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "published" || got.PublishedAt == "" {
		t.Fatalf("publish metadata: %+v", got)
	}

	err = st.WriteVersionFile(tid, p.Name, v.Version, "x.txt", strings.NewReader("nope"))
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable error, got %v", err)
	}

	ok, err := st.ValidatePullToken(st.mustRootToken(t))
	if err != nil || !ok {
		t.Fatalf("token validate: ok=%v err=%v", ok, err)
	}

	ver, err := st.LatestPublishedVersion(p.ID)
	if err != nil || ver != "1.0.0" {
		t.Fatalf("latest: %s err=%v", ver, err)
	}

	_ = filepath.Join(dir, "projects", p.Name, v.Version)
}

func (st *Store) mustRootToken(t *testing.T) string {
	t.Helper()
	users, err := st.ListUsers(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.Username == "root" {
			return u.Token
		}
	}
	t.Fatal("root user not found")
	return ""
}
