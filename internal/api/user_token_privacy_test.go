package api

import (
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestUserListNeverExposesPullTokens(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar := login(t, ts, "root", "root")
	users := mustGET[[]map[string]any](t, ts, jar, "/api/users")
	if len(users) == 0 {
		t.Fatal("expected root user")
	}
	for _, user := range users {
		if _, exists := user["token"]; exists {
			t.Fatal("user list must not expose pull token")
		}
	}
}
