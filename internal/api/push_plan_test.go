package api

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/config"
	"github.com/neko233-com/express233/internal/store"
)

type fleetExecutorFixture struct {
	calls   int
	fail    bool
	unknown bool
	check   func()
}

func (f *fleetExecutorFixture) Deploy(context.Context, store.PushHost, store.PushServerBinding, string, io.Reader) (string, error) {
	f.calls++
	if f.check != nil {
		f.check()
	}
	if f.unknown {
		return "connection lost", errPushOutcomeUnknown
	}
	if f.fail {
		return "fixture failure", errors.New("fixture deployment failed")
	}
	return "fixture completed", nil
}

func TestFrozenFleetRejectsDriftAndStopsAfterFirstFailure(t *testing.T) {
	for _, mode := range []string{"drift", "config-drift", "fail", "unknown", "success"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("EXPRESS233_PUBLIC_URL", "https://express.example.invalid")
			st, err := store.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			project, err := st.CreateProject(1, 1, "fleet")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.CreateVersion(1, project.ID, project.Name, "1.0.0"); err != nil {
				t.Fatal(err)
			}
			if err := st.WriteVersionFile(1, project.Name, "1.0.0", "binary", strings.NewReader("fixture")); err != nil {
				t.Fatal(err)
			}
			if err := st.PublishVersion(1, project.ID, "1.0.0"); err != nil {
				t.Fatal(err)
			}
			host, err := st.CreatePushHost(store.PushHost{TenantID: 1, Name: "fixture", Address: "192.0.2.1", Username: "deploy", AuthMode: "agent"}, "")
			if err != nil {
				t.Fatal(err)
			}
			var targets []store.PushServerBinding
			for _, id := range []string{"one", "two"} {
				b, err := st.CreatePushServerBinding(store.PushServerBinding{TenantID: 1, HostID: host.ID, ProjectName: project.Name, ServerID: id, RemoteRoot: "/srv/fixture"})
				if err != nil {
					t.Fatal(err)
				}
				targets = append(targets, *b)
			}
			s := New(st)
			frozen, err := s.freezePushPlan(1, project.ID, project.Name, "1.0.0", targets)
			if err != nil {
				t.Fatal(err)
			}
			d, err := st.CreatePushDeployment(store.PushDeployment{TenantID: 1, ProjectID: project.ID, Version: "1.0.0", RequestedBy: "root", Selector: "{}"}, frozen)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "drift" {
				b := frozen[1]
				b.RemoteRoot = "/srv/other"
				if err := st.UpdatePushServerBinding(b); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "config-drift" {
				if err := s.saveTenantServerFile(1, &config.ServerFile{Servers: map[string]config.ServerEntry{"one": {PostHookEnv: map[string]string{"MODE": "changed"}}}}); err != nil {
					t.Fatal(err)
				}
			}
			executor := &fleetExecutorFixture{fail: mode == "fail", unknown: mode == "unknown"}
			executor.check = func() {
				if err := s.ReloadAllServerYAML(); err == nil {
					t.Fatal("configuration reload crossed active deployment")
				}
			}
			s.pushExecutor = executor
			s.runPushDeployment(d.ID, 1, project.Name)
			s.runPushDeployment(d.ID, 1, project.Name) // A repeated delivery must not execute SSH again.
			got, err := st.GetPushDeploymentByID(d.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]int{"drift": 0, "config-drift": 0, "fail": 1, "unknown": 1, "success": 2}[mode]
			if executor.calls != want {
				t.Fatalf("calls=%d want=%d", executor.calls, want)
			}
			if (mode == "fail" || mode == "unknown") && got.Targets[1].Status != "skipped" {
				t.Fatalf("second node was not stopped: %+v", got.Targets)
			}
			if mode == "unknown" && got.Targets[0].Status != "unknown" {
				t.Fatal("transport interruption reported as known failure")
			}
			if (got.Status == "success") != (mode == "success") {
				t.Fatalf("status=%s", got.Status)
			}
		})
	}
}
