package store

import (
	"sync"
	"testing"
)

func TestCreateNextPatchVersion(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	p, err := st.CreateProject(1, 1, "auto-version")
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.CreateNextPatchVersion(1, p.ID, p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "0.0.1" {
		t.Fatalf("first version = %s, want 0.0.1", v.Version)
	}
	if _, err := st.CreateVersion(1, p.ID, p.Name, "not-semver"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVersion(1, p.ID, p.Name, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	v, err = st.CreateNextPatchVersion(1, p.ID, p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "1.2.4" {
		t.Fatalf("next version = %s, want 1.2.4", v.Version)
	}
}

func TestCreateNextPatchVersionConcurrent(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	p, err := st.CreateProject(1, 1, "auto-version-concurrent")
	if err != nil {
		t.Fatal(err)
	}

	const n = 6
	var wg sync.WaitGroup
	errs := make(chan error, n)
	versions := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := st.CreateNextPatchVersion(1, p.ID, p.Name)
			if err != nil {
				errs <- err
				return
			}
			versions <- v.Version
		}()
	}
	wg.Wait()
	close(errs)
	close(versions)
	for err := range errs {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for version := range versions {
		if seen[version] {
			t.Fatalf("duplicate auto version %s", version)
		}
		seen[version] = true
	}
	if len(seen) != n {
		t.Fatalf("created %d versions, want %d: %v", len(seen), n, seen)
	}
}
