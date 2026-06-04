package store

import (
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/config"
)

func TestValidateBeforePublish(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, _ := st.CreateProject(tid, st.TestRootUserID(), "g")
	v, _ := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	_ = st.WriteVersionFile(tid, p.Name, v.Version, "game.properties", strings.NewReader("k=1\n"))

	sf := &config.ServerFile{
		Servers: map[string]config.ServerEntry{
			"s1": {Replacements: map[string]config.FileOverrides{
				"game.properties": {"k": "2"},
				"missing.yaml":    {"a": "b"},
			}},
		},
	}
	r, err := st.ValidateBeforePublish(tid, p.Name, p.ID, v.Version, sf)
	if err != nil || !r.OK {
		t.Fatalf("validate: %+v err=%v", r, err)
	}
	if len(r.Warnings) == 0 {
		t.Fatal("expected warning for missing.yaml")
	}
}
