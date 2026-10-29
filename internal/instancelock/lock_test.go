package instancelock

import (
	"path/filepath"
	"testing"
)

func TestControllerOwnershipIsExclusiveAndReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.lock")
	release, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Acquire(path); err == nil {
		second()
		release()
		t.Fatal("second controller acquired same data directory")
	}
	release()
	next, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	next()
}
