package store

import "testing"

func TestPushBindingsSupportRepeatedServerIDByLabel(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const tid int64 = 1
	host, err := st.CreatePushHost(PushHost{TenantID: tid, Name: "logic-a", Address: "10.0.0.10", Username: "deploy", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"test", "canary"} {
		if _, err := st.CreatePushServerBinding(PushServerBinding{TenantID: tid, HostID: host.ID, ServerID: "logic-01", Labels: label, RemoteRoot: "/opt/game"}); err != nil {
			t.Fatalf("create %s binding: %v", label, err)
		}
	}
	targets, err := st.ListPushBindingsForSelector(tid, []string{"logic-01"}, []string{"test"}, true)
	if err != nil || len(targets) != 1 || targets[0].Labels != "test" {
		t.Fatalf("test selector: targets=%+v err=%v", targets, err)
	}
	all, err := st.ListPushBindingsForSelector(tid, []string{"logic-01"}, nil, true)
	if err != nil || len(all) != 2 {
		t.Fatalf("all selector: targets=%+v err=%v", all, err)
	}
}

func TestPushHostTOFUDoesNotOverwriteKnownKey(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const tid int64 = 1
	host, err := st.CreatePushHost(PushHost{TenantID: tid, Name: "logic-b", Address: "10.0.0.11", Username: "deploy", AuthMode: "agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordPushHostKey(tid, host.ID, "ssh-ed25519 AAAAfirst", "SHA256:first"); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordPushHostKey(tid, host.ID, "ssh-ed25519 AAAAsecond", "SHA256:second"); err != nil {
		t.Fatal(err)
	}
	got, _, err := st.GetPushHost(tid, host.ID)
	if err != nil || got.HostKeyFingerprint != "SHA256:first" {
		t.Fatalf("TOFU key=%+v err=%v", got, err)
	}
}
