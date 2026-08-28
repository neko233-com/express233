package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestPutProjectVersionRetentionPersistsProjectLimit(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, st.TestRootUserID(), "retention-api")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/version-retention", bytes.NewBufferString(`{"max_published_versions":5}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("root", "root")
	response := httptest.NewRecorder()
	New(st).Router().ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var updated store.Project
	if err := json.NewDecoder(response.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.MaxPublishedVersions != 5 {
		t.Fatalf("response max_published_versions=%d want=5", updated.MaxPublishedVersions)
	}
	maxPublishedVersions, err := st.ProjectMaxPublishedVersions(1, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if maxPublishedVersions != 5 {
		t.Fatalf("stored max_published_versions=%d want=5", maxPublishedVersions)
	}
}
