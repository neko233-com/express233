package store

import (
	"strings"
	"testing"
)

func TestValidateUniqueConfigBasenames(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, _ := st.CreateProject(tid, st.TestRootUserID(), "g")
	v, _ := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	_ = st.WriteVersionFile(tid, p.Name, v.Version, "a/x.properties", strings.NewReader("k=1\n"))
	err = st.WriteVersionFile(tid, p.Name, v.Version, "b/x.properties", strings.NewReader("k=2\n"))
	if err == nil || (!strings.Contains(err.Error(), "唯一") && !strings.Contains(err.Error(), "已存在")) {
		t.Fatalf("expected unique basename error, got %v", err)
	}
}
