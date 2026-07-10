package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

func TestLoginIPBanAndAdminClear(t *testing.T) {
	t.Setenv("EXPRESS233_LOGIN_FAILURE_LIMIT", "2")
	t.Setenv("EXPRESS233_LOGIN_FAILURE_WINDOW", "1h")
	t.Setenv("EXPRESS233_LOGIN_BAN_BASE", "1h")
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ts := httptest.NewServer(New(st).Router())
	defer ts.Close()

	jar, _ := cookiejar.New(nil)
	admin := &http.Client{Jar: jar}
	if got := doLoginStatus(t, admin, ts.URL, "root", "root"); got != http.StatusOK {
		t.Fatalf("initial login=%d", got)
	}
	attacker := &http.Client{}
	for i := 0; i < 2; i++ {
		if got := doLoginStatus(t, attacker, ts.URL, "root", "wrong-password"); got != http.StatusUnauthorized && got != http.StatusTooManyRequests {
			t.Fatalf("failure %d status=%d", i, got)
		}
	}
	if got := doLoginStatus(t, attacker, ts.URL, "root", "root"); got != http.StatusTooManyRequests {
		t.Fatalf("blocked login=%d", got)
	}

	resp, err := admin.Get(ts.URL + "/api/security/login-ip-bans")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list bans=%d", resp.StatusCode)
	}
	var bans []store.LoginIPBan
	if err := json.NewDecoder(resp.Body).Decode(&bans); err != nil || len(bans) != 1 {
		t.Fatalf("bans=%+v err=%v", bans, err)
	}
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/security/login-ip-bans/127.0.0.1", nil)
	resp, err = admin.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("clear ban=%d", resp.StatusCode)
	}
	if got := doLoginStatus(t, attacker, ts.URL, "root", "root"); got != http.StatusOK {
		t.Fatalf("login after clear=%d", got)
	}
}

func doLoginStatus(t *testing.T, client *http.Client, base, username, password string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := client.Post(base+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
