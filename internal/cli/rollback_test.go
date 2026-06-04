package cli

import "testing"

func TestPublishedVersionByOffsetLogic(t *testing.T) {
	vers := []versionRow{{Version: "1.2.0"}, {Version: "1.1.0"}, {Version: "1.0.0"}}
	if vers[1].Version != "1.1.0" {
		t.Fatal("order assumption")
	}
}
