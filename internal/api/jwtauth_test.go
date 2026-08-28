package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neko233-com/express233/internal/store"
)

func TestPersistentSessionTTLIsThirtyDays(t *testing.T) {
	if got, want := persistentSessionTTL, 30*24*time.Hour; got != want {
		t.Fatalf("persistentSessionTTL=%s want=%s", got, want)
	}
}

func TestJWTSignVerify(t *testing.T) {
	j := newJWTAuth()
	sess := session{UserID: 1, Username: "root", IsAdmin: true, AuthVersion: 1, TenantID: 1, TenantSlug: "default"}
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
	tok, _ := srv.jwt.sign(session{UserID: 1, Username: "root", IsAdmin: true, AuthVersion: 1, TenantID: 1, TenantSlug: "default"}, time.Hour)
	r, _ := http.NewRequest(http.MethodGet, "/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	if _, ok := srv.currentSession(r); !ok {
		t.Fatal("expected bearer session")
	}
}

func TestHandleMeRefreshesBrowserJWTForThirtyDays(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	tok, err := srv.jwt.sign(session{UserID: 1, Username: "root", IsAdmin: true, AuthVersion: 1, TenantID: 1, TenantSlug: "default"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	claims, err := srv.jwt.verify(payload.Token)
	if err != nil {
		t.Fatal(err)
	}
	remaining := time.Until(time.Unix(claims.Exp, 0))
	if remaining < 29*24*time.Hour || remaining > persistentSessionTTL+time.Minute {
		t.Fatalf("refreshed token remaining=%s", remaining)
	}
	cookies := rec.Result().Cookies()
	foundJWT := false
	for _, cookie := range cookies {
		if cookie.Name == jwtCookie {
			foundJWT = true
			if cookie.MaxAge != int(persistentSessionTTL.Seconds()) {
				t.Fatalf("jwt cookie max age=%d", cookie.MaxAge)
			}
		}
	}
	if !foundJWT {
		t.Fatal("refreshed JWT cookie missing")
	}
}

func TestPasswordChangeRevokesJWT(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	tok, err := srv.jwt.sign(session{UserID: 1, Username: "root", IsAdmin: true, AuthVersion: 1, TenantID: 1, TenantSlug: "default"}, persistentSessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	if _, ok := srv.currentSession(req); !ok {
		t.Fatal("expected JWT before password change")
	}
	if err := st.UpdateUserPassword(1, "new-password"); err != nil {
		t.Fatal(err)
	}
	if _, ok := srv.currentSession(req); ok {
		t.Fatal("JWT must be revoked after password change")
	}
}

func TestBasicAuthSession(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	r, _ := http.NewRequest(http.MethodGet, "/api/me", nil)
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("root:root")))
	sess, ok := srv.currentSession(r)
	if !ok {
		t.Fatal("expected basic auth session")
	}
	if sess.Username != "root" || !sess.IsAdmin || sess.TenantID != 1 {
		t.Fatalf("unexpected session: %+v", sess)
	}
}
