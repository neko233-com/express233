package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestDashboardAPIRecordsUploadByDay(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()
	jar := login(t, ts, "root", "root")

	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "dashboard-api"})
	projects := mustGET[[]map[string]any](t, ts, jar, "/api/projects")
	pid := int64(projects[0]["id"].(float64))
	mustPOST(t, ts, jar, fmt.Sprintf("/api/projects/%d/versions", pid), map[string]string{"name": "1.0.0"})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "game.bin")
	_, _ = fw.Write([]byte("binary-payload"))
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/projects/%d/versions/1.0.0/files", ts.URL, pid), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, cookie := range jar {
		req.AddCookie(cookie)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status %d", resp.StatusCode)
	}

	dashboard := mustGET[store.DashboardSnapshot](t, ts, jar, fmt.Sprintf("/api/dashboard?days=7&project_id=%d", pid))
	if dashboard.Days != 7 || len(dashboard.Series) != 7 || dashboard.Summary.Uploads != 1 || dashboard.Summary.UploadedFiles != 1 || dashboard.Summary.UploadBytes != int64(len("binary-payload")) {
		t.Fatalf("dashboard: %+v", dashboard)
	}
	if len(dashboard.Recent) == 0 || dashboard.Recent[0].Kind != "upload" || dashboard.Recent[0].Project != "dashboard-api" {
		t.Fatalf("recent: %+v", dashboard.Recent)
	}
}
