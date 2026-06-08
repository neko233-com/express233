package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestSystemUpdateRootOnly(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	defer st.Close()
	srv := New(st)
	srv.updateRunner = func(target, dataDir string) (string, error) {
		if target != "latest" {
			t.Fatalf("target = %q", target)
		}
		if dataDir != st.DataDir() {
			t.Fatalf("dataDir = %q, want %q", dataDir, st.DataDir())
		}
		return "scheduled update", nil
	}
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	rootJar := login(t, ts, "root", "root")
	status, body := doSystemUpdate(t, ts, rootJar)
	if status != http.StatusAccepted {
		t.Fatalf("root update status = %d body = %+v", status, body)
	}
	waitForSystemUpdateDone(t, ts, rootJar)

	mustPOST(t, ts, rootJar, "/api/users", map[string]any{
		"username": "admin2", "password": "admin2", "role": "admin", "is_admin": true,
	})
	adminJar := login(t, ts, "admin2", "admin2")
	status, body = doSystemUpdate(t, ts, adminJar)
	if status != http.StatusForbidden {
		t.Fatalf("admin update status = %d body = %+v", status, body)
	}
}

func doSystemUpdate(t *testing.T, ts *httptest.Server, jar []*http.Cookie) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/system/update", nil)
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func waitForSystemUpdateDone(t *testing.T, ts *httptest.Server, jar []*http.Cookie) {
	t.Helper()
	for i := 0; i < 20; i++ {
		status, body := getJSONMap(t, ts, jar, "/api/system/update")
		if status != http.StatusOK {
			t.Fatalf("status poll = %d body = %+v", status, body)
		}
		if running, _ := body["running"].(bool); !running {
			if ok, _ := body["ok"].(bool); !ok {
				t.Fatalf("update not ok: %+v", body)
			}
			return
		}
	}
	t.Fatal("system update did not finish")
}
