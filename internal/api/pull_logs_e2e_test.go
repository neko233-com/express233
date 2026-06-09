package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestPullWithBasicAuthRecordsProjectLog(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()
	jar := login(t, ts, "root", "root")

	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "pull-log-game"})
	projects := mustGET[[]map[string]any](t, ts, jar, "/api/projects")
	pid := int(projects[0]["id"].(float64))
	mustPOST(t, ts, jar, "/api/projects/"+itoa(pid)+"/versions", map[string]string{"name": "2.0.0"})
	uploadTaggedFile(t, ts, jar, pid, "2.0.0", "config/app.yaml", "id: template\n", "all")
	_ = os.WriteFile(mustServerYAML(st), []byte("servers:\n  s1:\n    replacements: {}\n"), 0o644)
	srv.reloadServerYAML(1)
	mustPOST(t, ts, jar, "/api/projects/"+itoa(pid)+"/versions/2.0.0/publish", nil)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/pull?project=pull-log-game&version=2.0.0&server_id=s1&os=linux&arch=amd64&tags=canary", nil)
	req.SetBasicAuth("root", "root")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pull %d", resp.StatusCode)
	}

	logs := mustGET[[]store.ProjectLog](t, ts, jar, "/api/projects/"+itoa(pid)+"/logs?server_id=s1&version=2.0.0")
	if len(logs) != 1 {
		t.Fatalf("logs len=%d: %+v", len(logs), logs)
	}
	got := logs[0]
	if got.Username != "root" || got.Status != "ok" || got.ServerID != "s1" || got.Version != "2.0.0" || got.OS != "linux" || got.Arch != "amd64" || !strings.Contains(got.Tags, "canary") {
		t.Fatalf("log = %+v", got)
	}
}
