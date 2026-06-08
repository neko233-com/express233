package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestE2E_StorageOverviewSearchDelete(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "storage-e2e"})
	projects := mustGET[[]map[string]any](t, ts, jar, "/api/projects")
	var pid int64
	for _, p := range projects {
		if p["name"] == "storage-e2e" {
			pid = int64(p["id"].(float64))
		}
	}
	pidStr := formatInt64(pid)
	mustPOST(t, ts, jar, "/api/projects/"+pidStr+"/versions", map[string]string{"name": "1.0.0"})
	uploadVersionFile(t, ts, jar, pidStr, "1.0.0", "cfg.properties", "k=v\n")

	ov := mustGET[map[string]any](t, ts, jar, "/api/storage/overview")
	if ov["project_count"].(float64) < 1 {
		t.Fatalf("overview: %v", ov)
	}

	mustPOST(t, ts, jar, "/api/storage/reindex", nil)
	search := mustGET[map[string]any](t, ts, jar, "/api/storage/search?q=cfg.properties")
	hits := search["hits"].([]any)
	if len(hits) == 0 {
		t.Fatal("expected search hits")
	}

	tree := mustGET[map[string]any](t, ts, jar, "/api/storage/tree")
	if tree["path"] == nil {
		t.Fatal("expected tree root")
	}

	tn, _ := st.TenantByID(1)
	relVer := "userdata/" + tn.Slug + "/projects/storage-e2e/1.0.0"
	plan := mustGET[map[string]any](t, ts, jar, "/api/storage/delete-plan?path="+relVer)
	if plan["allowed"] != true {
		t.Fatalf("delete plan: %v", plan)
	}

	b, _ := json.Marshal(map[string]string{"path": relVer})
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/storage/items", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status %d", resp.StatusCode)
	}
}

func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
