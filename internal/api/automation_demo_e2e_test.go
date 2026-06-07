package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/store"
	"gopkg.in/yaml.v3"
)

func TestE2E_AutomationDemoMultiVersion(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	putReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/servers/game-a", bytes.NewReader(mustJSON(t, map[string]any{
		"replacements": map[string]any{
			"game.properties": map[string]any{
				"server.port": "9001",
			},
			"application.yaml": map[string]any{
				"spring.profiles.active": "game-a",
			},
		},
		"post_hook": "scripts/reload-{{SERVER_ID}}.sh",
	})))
	putReq.Header.Set("Content-Type", "application/json")
	setBasicRootAuth(putReq)
	resp, err := http.DefaultClient.Do(putReq)
	assertStatus(t, resp, err, http.StatusOK)

	projReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/projects", bytes.NewReader(mustJSON(t, map[string]string{"name": "demo-auto"})))
	projReq.Header.Set("Content-Type", "application/json")
	setBasicRootAuth(projReq)
	resp, err = http.DefaultClient.Do(projReq)
	if err != nil {
		t.Fatal(err)
	}
	var proj struct {
		ID int64 `json:"id"`
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("create project %d %s", resp.StatusCode, b)
	}
	if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	pid := strconv.FormatInt(proj.ID, 10)

	for _, version := range []string{"1.0.0", "1.1.0"} {
		verReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/projects/"+pid+"/versions", bytes.NewReader(mustJSON(t, map[string]string{"name": version})))
		verReq.Header.Set("Content-Type", "application/json")
		setBasicRootAuth(verReq)
		resp, err = http.DefaultClient.Do(verReq)
		assertStatus(t, resp, err, http.StatusCreated)
	}

	uploadArchive(t, ts.URL+"/api/projects/"+pid+"/versions/1.0.0/files", map[string]string{
		"conf/game.properties":  "server.port=8080\nfeature.flag=base\n",
		"conf/application.yaml": "spring:\n  profiles:\n    active: default\n",
	})
	uploadArchive(t, ts.URL+"/api/projects/"+pid+"/versions/1.1.0/files", map[string]string{
		"conf/game.properties":  "server.port=8181\nfeature.flag=blue\n",
		"conf/application.yaml": "spring:\n  profiles:\n    active: staging\n",
	})

	prevReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/deploy/preview?project=demo-auto&version=1.1.0&server_id=game-a", nil)
	setBasicRootAuth(prevReq)
	resp, err = http.DefaultClient.Do(prevReq)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("preview %d %s", resp.StatusCode, b)
	}
	var prev struct {
		PostHook string `json:"post_hook"`
		Files    []struct {
			Basename string `json:"basename"`
			Changes  []struct {
				Key   string `json:"key"`
				After string `json:"after"`
			} `json:"changes"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prev); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if prev.PostHook != "scripts/reload-game-a.sh" {
		t.Fatalf("unexpected post_hook: %s", prev.PostHook)
	}
	if !hasPreviewAfter(prev.Files, "game.properties", "server.port", "9001") {
		t.Fatalf("preview replacements missing: %+v", prev.Files)
	}
	if !hasPreviewAfter(prev.Files, "application.yaml", "spring.profiles.active", "game-a") {
		t.Fatalf("preview yaml replacements missing: %+v", prev.Files)
	}

	for _, version := range []string{"1.0.0", "1.1.0"} {
		pubReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/projects/"+pid+"/versions/"+version+"/publish", nil)
		setBasicRootAuth(pubReq)
		resp, err = http.DefaultClient.Do(pubReq)
		assertStatus(t, resp, err, http.StatusOK)
	}

	diffReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/deploy/diff?project=demo-auto&from=1.0.0&to=1.1.0&server_id=game-a", nil)
	setBasicRootAuth(diffReq)
	resp, err = http.DefaultClient.Do(diffReq)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("diff %d %s", resp.StatusCode, b)
	}
	var diff struct {
		Files []struct {
			Basename string `json:"basename"`
			Keys     []struct {
				Key    string `json:"key"`
				From   string `json:"from"`
				To     string `json:"to"`
				Change string `json:"change"`
			} `json:"keys"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&diff); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if !hasDiffKey(diff.Files, "game.properties", "feature.flag", "base", "blue", "modified") {
		t.Fatalf("diff missing feature flag change: %+v", diff.Files)
	}

	users, err := st.ListUsers(1)
	if err != nil || len(users) == 0 {
		t.Fatalf("list users: %v %v", users, err)
	}
	token := users[0].Token
	resp, err = http.Get(ts.URL + "/api/pull/versions?token=" + token + "&project=demo-auto")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("pull versions %d %s", resp.StatusCode, b)
	}
	var vers struct {
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vers); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(vers.Versions) != 2 {
		t.Fatalf("unexpected versions: %+v", vers)
	}

	resp, err = http.Get(ts.URL + "/api/pull?token=" + token + "&project=demo-auto&server_id=game-a")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("pull latest %d %s", resp.StatusCode, b)
	}
	bundle, err := readTarGzResponse(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bundle["conf/game.properties"], "server.port=9001") || !strings.Contains(bundle["conf/game.properties"], "feature.flag=blue") {
		t.Fatalf("unexpected pulled game.properties: %q", bundle["conf/game.properties"])
	}
	if !strings.Contains(bundle["conf/application.yaml"], "active: game-a") {
		t.Fatalf("unexpected pulled application.yaml: %q", bundle["conf/application.yaml"])
	}
}

func TestE2E_GetOrCreateProjectAutoVersionAndStructuredMergePull(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	putReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/servers/merge-srv", bytes.NewReader(mustJSON(t, map[string]any{
		"replacements": map[string]any{
			"application.yaml": map[string]any{
				"database": map[string]any{
					"mysql": map[string]any{
						"password": "secret",
						"pool": map[string]any{
							"max_open": 16,
						},
					},
				},
				"feature.flags.pvp": true,
			},
			"settings.json": map[string]any{
				"service": map[string]any{
					"region": "cn-shanghai",
				},
				"limits.daily": 200,
			},
		},
	})))
	putReq.Header.Set("Content-Type", "application/json")
	setBasicRootAuth(putReq)
	resp, err := http.DefaultClient.Do(putReq)
	assertStatus(t, resp, err, http.StatusOK)

	firstProject := createProjectBasic(t, ts.URL, "merge-auto")
	secondProject := createProjectBasic(t, ts.URL, "merge-auto")
	if firstProject.ID != secondProject.ID {
		t.Fatalf("get-or-create returned different projects: first=%+v second=%+v", firstProject, secondProject)
	}
	pid := strconv.FormatInt(firstProject.ID, 10)

	if v := createVersionBasic(t, ts.URL, pid, nil); v.Version != "0.0.1" {
		t.Fatalf("first auto version = %s, want 0.0.1", v.Version)
	}
	if v := createVersionBasic(t, ts.URL, pid, nil); v.Version != "0.0.2" {
		t.Fatalf("second auto version = %s, want 0.0.2", v.Version)
	}
	if v := createVersionBasic(t, ts.URL, pid, map[string]string{"name": "1.2.3"}); v.Version != "1.2.3" {
		t.Fatalf("explicit version = %s, want 1.2.3", v.Version)
	}
	latest := createVersionBasic(t, ts.URL, pid, map[string]string{})
	if latest.Version != "1.2.4" {
		t.Fatalf("latest auto version = %s, want 1.2.4", latest.Version)
	}

	uploadArchive(t, ts.URL+"/api/projects/"+pid+"/versions/"+latest.Version+"/files", map[string]string{
		"conf/application.yaml": `database:
  mysql:
    host: db.internal
    port: 3306
    password: old
    pool:
      max_open: 8
      timeout: 5s
  redis:
    host: redis.internal
feature:
  flags:
    pvp: false
    pve: true
owners:
  - core
`,
		"conf/settings.json": `{
  "service": {
    "name": "logic",
    "region": "us-west",
    "endpoints": {
      "primary": "logic.internal",
      "backup": "logic-backup.internal"
    }
  },
  "limits": {
    "daily": 100,
    "burst": 10
  },
  "features": ["chat", "match"]
}`,
	})

	pubReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/projects/"+pid+"/versions/"+latest.Version+"/publish", nil)
	setBasicRootAuth(pubReq)
	resp, err = http.DefaultClient.Do(pubReq)
	assertStatus(t, resp, err, http.StatusOK)

	users, err := st.ListUsers(1)
	if err != nil || len(users) == 0 {
		t.Fatalf("list users: %v %v", users, err)
	}
	resp, err = http.Get(ts.URL + "/api/pull?token=" + users[0].Token + "&project=merge-auto&server_id=merge-srv")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("pull latest %d %s", resp.StatusCode, b)
	}
	bundle, err := readTarGzResponse(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	assertPulledStructuredMerge(t, bundle, latest.Version)
}

type projectResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type versionResponse struct {
	Version string `json:"version"`
}

func createProjectBasic(t *testing.T, baseURL, name string) projectResponse {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/projects", bytes.NewReader(mustJSON(t, map[string]string{"name": name})))
	req.Header.Set("Content-Type", "application/json")
	setBasicRootAuth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create project %d %s", resp.StatusCode, b)
	}
	var out projectResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func createVersionBasic(t *testing.T, baseURL, projectID string, body any) versionResponse {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(mustJSON(t, body))
	}
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/projects/"+projectID+"/versions", r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	setBasicRootAuth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create version %d %s", resp.StatusCode, b)
	}
	var out versionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertPulledStructuredMerge(t *testing.T, bundle map[string]string, version string) {
	t.Helper()
	var app map[string]any
	if err := yaml.Unmarshal([]byte(bundle["conf/application.yaml"]), &app); err != nil {
		t.Fatal(err)
	}
	database := testMap(t, app, "database")
	mysql := testMap(t, database, "mysql")
	pool := testMap(t, mysql, "pool")
	redis := testMap(t, database, "redis")
	feature := testMap(t, app, "feature")
	flags := testMap(t, feature, "flags")
	if mysql["host"] != "db.internal" || mysql["password"] != "secret" || pool["timeout"] != "5s" || redis["host"] != "redis.internal" {
		t.Fatalf("yaml merge overwrote unrelated tree: %s", bundle["conf/application.yaml"])
	}
	if pool["max_open"] != 16 || flags["pvp"] != true || flags["pve"] != true {
		t.Fatalf("yaml merge did not apply expected leaves: %s", bundle["conf/application.yaml"])
	}
	owners, ok := app["owners"].([]any)
	if !ok || len(owners) != 1 || owners[0] != "core" {
		t.Fatalf("yaml merge should preserve list: %#v", app["owners"])
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(bundle["conf/settings.json"]), &settings); err != nil {
		t.Fatal(err)
	}
	service := testMap(t, settings, "service")
	endpoints := testMap(t, service, "endpoints")
	limits := testMap(t, settings, "limits")
	if service["name"] != "logic" || service["region"] != "cn-shanghai" || endpoints["backup"] != "logic-backup.internal" {
		t.Fatalf("json merge overwrote unrelated service tree: %s", bundle["conf/settings.json"])
	}
	if limits["daily"] != float64(200) || limits["burst"] != float64(10) {
		t.Fatalf("json merge limits mismatch: %s", bundle["conf/settings.json"])
	}
	features, ok := settings["features"].([]any)
	if !ok || len(features) != 2 || features[0] != "chat" || features[1] != "match" {
		t.Fatalf("json merge should preserve array: %#v", settings["features"])
	}

	var manifest struct {
		Version  string `json:"version"`
		ServerID string `json:"server_id"`
	}
	if err := json.Unmarshal([]byte(bundle[".express233/manifest.json"]), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != version || manifest.ServerID != "merge-srv" {
		t.Fatalf("manifest mismatch: %+v", manifest)
	}
}

func testMap(t *testing.T, root map[string]any, key string) map[string]any {
	t.Helper()
	out, ok := root[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not a map: %#v", key, root[key])
	}
	return out
}

func uploadArchive(t *testing.T, url string, files map[string]string) {
	t.Helper()
	archive := mustTarGzArchive(t, files)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "bundle.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(archive); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	setBasicRootAuth(req)
	resp, err := http.DefaultClient.Do(req)
	assertStatus(t, resp, err, http.StatusOK)
}

func assertStatus(t *testing.T, resp *http.Response, err error, want int) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d want %d body=%s", resp.StatusCode, want, b)
	}
}

func hasPreviewAfter(files []struct {
	Basename string `json:"basename"`
	Changes  []struct {
		Key   string `json:"key"`
		After string `json:"after"`
	} `json:"changes"`
}, basename, key, want string) bool {
	for _, file := range files {
		if file.Basename != basename {
			continue
		}
		for _, change := range file.Changes {
			if change.Key == key && change.After == want {
				return true
			}
		}
	}
	return false
}

func hasDiffKey(files []struct {
	Basename string `json:"basename"`
	Keys     []struct {
		Key    string `json:"key"`
		From   string `json:"from"`
		To     string `json:"to"`
		Change string `json:"change"`
	} `json:"keys"`
}, basename, key, from, to, change string) bool {
	for _, file := range files {
		if file.Basename != basename {
			continue
		}
		for _, item := range file.Keys {
			if item.Key == key && item.From == from && item.To == to && item.Change == change {
				return true
			}
		}
	}
	return false
}

func readTarGzResponse(r io.Reader) (map[string]string, error) {
	out := make(map[string]string)
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		out[hdr.Name] = string(data)
	}
}
