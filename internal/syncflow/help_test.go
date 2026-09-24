package syncflow

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyncHelpWithoutVault(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}} {
		var out bytes.Buffer
		if err := Run(args, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		for _, command := range []string{"scan", "build-new", "checkpoint-package", "tool-digest", "verify-published"} {
			if !strings.Contains(out.String(), command) {
				t.Fatalf("%v: missing %s", args, command)
			}
		}
	}
}

func TestUnknownSyncOperationReportsUsage(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"not-a-command"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unknown sync operation") || !strings.Contains(err.Error(), "sync --help") || strings.Contains(err.Error(), "not yet migrated") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatal("unexpected output for invalid operation")
	}
}
