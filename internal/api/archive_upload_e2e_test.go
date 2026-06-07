package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestE2E_UploadTarGzArchive(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")

	mustPOST(t, ts, jar, "/api/projects", map[string]string{"name": "bundle"})
	projects := mustGET[[]map[string]any](t, ts, jar, "/api/projects")
	var pid float64
	for _, p := range projects {
		if p["name"] == "bundle" {
			pid = p["id"].(float64)
		}
	}
	if pid == 0 {
		t.Fatal("project not found")
	}
	pidStr := strconv.FormatInt(int64(pid), 10)
	mustPOST(t, ts, jar, "/api/projects/"+pidStr+"/versions", map[string]string{"name": "2.0.0"})

	archive := mustTarGzArchive(t, map[string]string{
		"conf/app.properties": "port=8080\n",
		"config/app.yaml":     "http:\n  port: 8080\n",
		"scripts/restart.sh":  "#!/bin/sh\nexit 0\n",
	})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "bundle.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(archive); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/"+pidStr+"/versions/2.0.0/files", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range jar {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload archive %d", resp.StatusCode)
	}

	files := mustGET[[]string](t, ts, jar, "/api/projects/"+pidStr+"/versions/2.0.0/files")
	if len(files) != 3 {
		t.Fatalf("unexpected files: %v", files)
	}
	stats, err := st.BlobStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.BlobCount != 3 || stats.TotalRefs != 3 {
		t.Fatalf("tar upload should ingest blobs, got %+v", stats)
	}
}

func mustTarGzArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
