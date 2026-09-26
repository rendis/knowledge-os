package discover

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Package tests change vaults and repositories between checks in one process; they run uncached
	// except where a test turns the cache on for itself.
	os.Setenv("KOS_NO_CACHE", "1")
	os.Exit(m.Run())
}

func TestNoteCheckCacheReusesOnlyUnchangedInputs(t *testing.T) {
	t.Setenv("KOS_NO_CACHE", "")
	t.Setenv("KOS_CACHE_DIR", t.TempDir())
	vault := t.TempDir()
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: C\n  purpose: p\nsystems:\n  - id: s\n    name: S\n")
	write(t, vault, "20-Repos/n.md", "---\ntipo: api\n---\n# n\n")
	first, e := checkNote(vault, "20-Repos/n.md", "")
	if e != nil {
		t.Fatal(e)
	}
	entries, _ := os.ReadDir(checkCacheDir())
	if len(entries) != 1 {
		t.Fatalf("a settled result is stored: %d entries", len(entries))
	}
	again, _ := checkNote(vault, "20-Repos/n.md", "")
	if again.Note != first.Note || again.OK != first.OK {
		t.Fatalf("the stored result is reused: %+v %+v", first, again)
	}
	write(t, vault, "20-Repos/n.md", "---\ntipo: api\n---\n# n changed\n")
	checkNote(vault, "20-Repos/n.md", "")
	if entries, _ = os.ReadDir(checkCacheDir()); len(entries) != 2 {
		t.Fatalf("a changed note is checked again: %d entries", len(entries))
	}
}
