package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
)

type failingPushHealthChecker struct{ calls int }

func (c *failingPushHealthChecker) Check(context.Context, store.PushHost) error {
	c.calls++
	return errors.New("connection refused")
}

func TestPushHealthAPIAndAgentCapabilities(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	host, err := st.CreatePushHost(store.PushHost{TenantID: 1, Name: "node-1", Address: "127.0.0.1", Port: 22, Username: "root", AuthMode: "agent", HealthCheckEnabled: true, HealthCheckIntervalSeconds: 3600}, "")
	if err != nil {
		t.Fatal(err)
	}
	srv := New(st)
	checker := &failingPushHealthChecker{}
	srv.pushHealthChecker = checker
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()
	cookies := login(t, ts, "root", "root")

	request, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/push/hosts/"+strconv.FormatInt(host.ID, 10)+"/check", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var check store.PushHostCheck
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&check) != nil || check.Status != "failed" {
		t.Fatalf("status=%d check=%+v", response.StatusCode, check)
	}
	if checker.calls != 1 {
		t.Fatalf("health checker calls=%d, want exactly 1", checker.calls)
	}

	request, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/push/hosts/"+strconv.FormatInt(host.ID, 10)+"/checks", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response, err = ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var checks []store.PushHostCheck
	_ = json.NewDecoder(response.Body).Decode(&checks)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || len(checks) != 1 {
		t.Fatalf("status=%d checks=%+v", response.StatusCode, checks)
	}

	request, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/agent/capabilities", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response, err = ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	_ = json.NewDecoder(response.Body).Decode(&payload)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(payload["credential_policy"].(string), "never returned") {
		t.Fatalf("status=%d payload=%+v", response.StatusCode, payload)
	}
}

func TestPushHostCredentialIsEncryptedAndNeverReturned(t *testing.T) {
	t.Setenv("EXPRESS233_PUSH_CREDENTIAL_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()
	cookies := login(t, ts, "root", "root")
	password := "private-password"
	body := mustJSON(t, map[string]any{
		"name": "encrypted-node", "address": "127.0.0.1", "port": 22, "username": "root",
		"auth_mode": "password", "credential": password,
		"host_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	})
	request, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/push/hosts", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.StatusCode, raw)
	}
	for _, forbidden := range []string{password, "credential", "private_key", "encrypted_private_key"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("response disclosed %q: %s", forbidden, raw)
		}
	}
	var host store.PushHost
	if err := json.Unmarshal(raw, &host); err != nil {
		t.Fatal(err)
	}
	if !host.HealthCheckEnabled || host.HealthCheckIntervalSeconds != 3600 {
		t.Fatalf("health defaults=%+v", host)
	}
	_, encrypted, err := st.GetPushHost(1, host.ID)
	if err != nil || encrypted == "" || encrypted == password {
		t.Fatalf("credential was not encrypted: empty=%v plaintext=%v err=%v", encrypted == "", encrypted == password, err)
	}

	request, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/push/hosts", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response, err = ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if bytes.Contains(raw, []byte(password)) || bytes.Contains(raw, []byte("encrypted_private_key")) {
		t.Fatalf("list response disclosed credential: %s", raw)
	}
}
