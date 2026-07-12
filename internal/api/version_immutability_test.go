package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestPublishedVersionIsImmutableButRemainsReadable(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, st.TestRootUserID(), "fictional-immutable")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVersion(1, project.ID, project.Name, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteVersionFile(1, project.Name, "1.0.0", "config.yaml", bytes.NewBufferString("fixture: true\n")); err != nil {
		t.Fatal(err)
	}
	if err := st.PublishVersion(1, project.ID, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()
	jar := login(t, ts, "root", "root")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, _ := writer.CreateFormFile("file", "replacement.bin")
	_, _ = file.Write([]byte("mutate"))
	_ = writer.Close()
	projectID := strconv.FormatInt(project.ID, 10)
	request, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/projects/"+projectID+"/versions/1.0.0/files", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for _, cookie := range jar {
		request.AddCookie(cookie)
	}
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("published upload status=%d", response.StatusCode)
	}

	put, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/projects/"+projectID+"/versions/1.0.0/file-tags", bytes.NewBufferString(`{"path":"config.yaml","tags":["fixture"]}`))
	put.Header.Set("Content-Type", "application/json")
	for _, cookie := range jar {
		put.AddCookie(cookie)
	}
	response, err = ts.Client().Do(put)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("published tag mutation status=%d", response.StatusCode)
	}

	read := mustGET[map[string]any](t, ts, jar, "/api/projects/"+projectID+"/versions/1.0.0/config-files")
	if read["files"] == nil {
		t.Fatal("published version metadata must stay readable")
	}
}
