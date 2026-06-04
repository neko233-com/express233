package hookspec_test

import (
	"strings"
	"testing"

	"github.com/neko233-com/express233/internal/hookspec"
)

func TestStepMatchesOS(t *testing.T) {
	root := "../../testdata/validation-tree/version"
	plan, err := hookspec.PlanLines(root, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || !strings.Contains(plan[0], "restart.sh") {
		t.Fatalf("linux: %v", plan)
	}
	planWin, err := hookspec.PlanLines(root, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(planWin[0], "restart.ps1") {
		t.Fatalf("windows: %v", planWin)
	}
	planOther, err := hookspec.PlanLines(root, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(planOther[0], "restart.sh") {
		t.Fatalf("darwin (else): %v", planOther)
	}
}
