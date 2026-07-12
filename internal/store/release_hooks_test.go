package store

import (
	"testing"
	"time"
)

func TestReleaseHookTrailingDebounceCoalescesTriggers(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.CreateProject(1, 1, "hook-coalesce-test")
	if err != nil {
		t.Fatal(err)
	}
	task, err := st.CreatePushDeploymentTask(PushDeploymentTask{TenantID: 1, ProjectID: project.ID, Name: "fictional-logic-fleet"})
	if err != nil {
		t.Fatal(err)
	}
	hook, err := st.CreateReleaseHook(ReleaseHook{TenantID: 1, ProjectID: project.ID, TaskID: task.ID, Name: "publish-after-upload", Enabled: true, DebounceSeconds: 30, CreatedBy: "root"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2030, 1, 2, 3, 4, 5, 0, time.Local)
	for index, trigger := range []struct {
		after   time.Duration
		version string
	}{{0, "1.0.0"}, {10 * time.Second, "1.0.1"}, {20 * time.Second, "1.0.2"}} {
		queued, err := st.QueueReleaseHook(1, project.ID, hook.ID, trigger.version, "root", "version_publish", start.Add(trigger.after))
		if err != nil {
			t.Fatal(err)
		}
		if queued.PendingEvents != int64(index+1) {
			t.Fatalf("pending events after trigger %d: %+v", index+1, queued)
		}
	}
	current, err := st.GetReleaseHook(1, project.ID, hook.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.PendingVersion != "1.0.2" || current.TriggerCount != 3 || current.MergeCount != 2 || current.DueAt != start.Add(50*time.Second).Format(timeLayout) {
		t.Fatalf("coalesced hook: %+v", current)
	}
	before, err := st.ClaimDueReleaseHooks(start.Add(49*time.Second), 10)
	if err != nil || len(before) != 0 {
		t.Fatalf("claimed before trailing window: %+v err=%v", before, err)
	}
	runs, err := st.ClaimDueReleaseHooks(start.Add(51*time.Second), 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("due runs: %+v err=%v", runs, err)
	}
	if runs[0].Version != "1.0.2" || runs[0].EventCount != 3 {
		t.Fatalf("run snapshot: %+v", runs[0])
	}
	tooSoon, err := st.ListStaleReleaseHookRuns(start.Add(2*time.Minute), 2*time.Minute, 10)
	if err != nil || len(tooSoon) != 0 {
		t.Fatalf("fresh running hook considered stale: %+v err=%v", tooSoon, err)
	}
	stale, err := st.ListStaleReleaseHookRuns(start.Add(3*time.Minute), 2*time.Minute, 10)
	if err != nil || len(stale) != 1 || stale[0].EventID != runs[0].EventID || stale[0].RequestedBy != "root" {
		t.Fatalf("stale recovery snapshot: %+v err=%v", stale, err)
	}
	again, err := st.ClaimDueReleaseHooks(start.Add(60*time.Second), 10)
	if err != nil || len(again) != 0 {
		t.Fatalf("duplicate claim: %+v err=%v", again, err)
	}
	if err := st.CompleteReleaseHookRun(runs[0], 99, "dispatched", "test dispatch", start.Add(51*time.Second)); err != nil {
		t.Fatal(err)
	}
	events, err := st.ListReleaseHookEvents(1, project.ID, 20)
	if err != nil || len(events) != 4 || events[0].Kind != "dispatch" || events[0].MergedEvents != 3 {
		t.Fatalf("hook events: %+v err=%v", events, err)
	}
}

func TestDisabledReleaseHookCancelsPendingAndProtectsTask(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, _ := st.CreateProject(1, 1, "hook-toggle-test")
	task, _ := st.CreatePushDeploymentTask(PushDeploymentTask{TenantID: 1, ProjectID: project.ID, Name: "protected-task"})
	hook, _ := st.CreateReleaseHook(ReleaseHook{TenantID: 1, ProjectID: project.ID, TaskID: task.ID, Name: "toggle-hook", Enabled: true, DebounceSeconds: 30})
	if _, err := st.QueueReleaseHook(1, project.ID, hook.ID, "2.0.0", "root", "manual", time.Now()); err != nil {
		t.Fatal(err)
	}
	hook.Enabled = false
	if err := st.UpdateReleaseHook(*hook); err != nil {
		t.Fatal(err)
	}
	disabled, _ := st.GetReleaseHook(1, project.ID, hook.ID)
	if disabled.PendingEvents != 0 || disabled.DueAt != "" || disabled.LastStatus != "disabled" {
		t.Fatalf("disabled hook retained pending work: %+v", disabled)
	}
	if err := st.DeletePushDeploymentTask(1, project.ID, task.ID); err == nil {
		t.Fatal("task referenced by a disabled hook was deleted")
	}
	if err := st.DeleteReleaseHook(1, project.ID, hook.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePushDeploymentTask(1, project.ID, task.ID); err != nil {
		t.Fatal(err)
	}
}
