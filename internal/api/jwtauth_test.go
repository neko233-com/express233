package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/neko233-com/express233/internal/store"
)

func TestJWTSignVerify(t *testing.T) {
	j := newJWTAuth()
	sess := session{UserID: 1, Username: "root", IsAdmin: true, TenantID: 1, TenantSlug: "default"}
	tok, err := j.sign(sess, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c, err := j.verify(tok)
	if err != nil || c.Username != "root" {
		t.Fatalf("verify: %v %+v", err, c)
	}
}

func TestJWTBearerAuth(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st)
	tok, _ := srv.jwt.sign(session{UserID: 1, Username: "root", IsAdmin: true, TenantID: 1, TenantSlug: "default"}, time.Hour)
	r, _ := http.NewRequest(http.MethodGet, "/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	if _, ok := srv.currentSession(r); !ok {
		t.Fatal("expected bearer session")
	}
}
