package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestPublicAgentGuideOnlyServesReviewedStaticContent(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()
	response, err := http.Get(ts.URL + "/api/agent/guide")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(response.Header.Get("Cache-Control"), "public") {
		t.Fatalf("guide index status=%d cache=%q", response.StatusCode, response.Header.Get("Cache-Control"))
	}
	_ = response.Body.Close()
	response, err = http.Get(ts.URL + "/api/agent/guide/gitea")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("guide topic status=%d", response.StatusCode)
	}
	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), "Gitea") || strings.Contains(string(body), "root/root") {
		t.Fatalf("unsafe guide body: %q", body)
	}
	missing, _ := http.Get(ts.URL + "/api/agent/guide/not-a-topic")
	if missing == nil || missing.StatusCode != http.StatusNotFound {
		if missing != nil {
			_ = missing.Body.Close()
		}
		t.Fatal("unknown topic must not expose arbitrary files")
	}
	_ = missing.Body.Close()
}
