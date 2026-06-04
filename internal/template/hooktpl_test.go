package template

import "testing"

func TestRenderHookTemplate(t *testing.T) {
	got := RenderHookTemplate("scripts/restart-{{SERVER_ID}}.sh", HookTemplateVars("p", "1.0", "logic-01", nil))
	if got != "scripts/restart-logic-01.sh" {
		t.Fatalf("got %q", got)
	}
}
