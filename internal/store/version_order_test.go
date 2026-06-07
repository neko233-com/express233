package store

import "testing"

func TestLatestPublishedVersion_DeterministicWhenPublishedAtEqual(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const tid int64 = 1
	p, err := st.CreateProject(tid, st.TestRootUserID(), "order-demo")
	if err != nil {
		t.Fatal(err)
	}
	v1, err := st.CreateVersion(tid, p.ID, p.Name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := st.CreateVersion(tid, p.ID, p.Name, "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	const publishedAt = "2026-06-07 13:37:00"
	if _, err := st.db.Exec(`UPDATE versions SET status = 'published', published_at = ? WHERE id IN (?, ?)`, publishedAt, v1.ID, v2.ID); err != nil {
		t.Fatal(err)
	}

	latest, err := st.LatestPublishedVersion(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest != "1.1.0" {
		t.Fatalf("latest=%s", latest)
	}

	first, err := st.PublishedVersionByOffset(p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.PublishedVersionByOffset(p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first != "1.1.0" || second != "1.0.0" {
		t.Fatalf("published order first=%s second=%s", first, second)
	}
}
