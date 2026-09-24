package check

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func closureFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if e := os.MkdirAll(filepath.Join(root, "20-Repos/demo"), 0700); e != nil {
		t.Fatal(e)
	}
	files := map[string]string{"20-Repos/demo/a.md": "- [ ] first\n  - [ ] second\n- [x] done\n", "20-Repos/demo/b.md": "no pending items\n", "00-Home.md": "**Remaining: 2 verification items in 1 notes.**\n", "coverage.md": "Permanecen 2 verificaciones en 1 notas.\n"}
	for p, b := range files {
		if e := os.WriteFile(filepath.Join(root, p), []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
	}
	cp := filepath.Join(root, "checkpoint.json")
	setClosureCheckpoint(t, cp, 2, 1, []any{"00-Home.md", "coverage.md"})
	return root, cp
}
func setClosureCheckpoint(t *testing.T, p string, count, notes any, paths []any) {
	t.Helper()
	b, e := json.Marshal(map[string]any{"visible_coverage": map[string]any{"remaining_verification_items": count, "notes_with_verifications": notes, "paths": paths}})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestMapClosurePassAndStale(t *testing.T) {
	root, cp := closureFixture(t)
	r, e := MapClosure(root, cp)
	if e != nil || r.Status != "pass" || r.Observed.Remaining != 2 || r.Observed.Notes != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	if e = os.WriteFile(filepath.Join(root, "20-Repos/demo/b.md"), []byte("- [ ] new\n"), 0600); e != nil {
		t.Fatal(e)
	}
	r, e = MapClosure(root, cp)
	if e == nil || len(r.Errors) != 4 {
		t.Fatalf("stale checkpoint and both visible summaries should fail: %+v %v", r, e)
	}
}
func TestMapClosureTypeAndPathBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count any
		paths []any
		want  string
	}{{"boolean", true, []any{"00-Home.md", "coverage.md"}, "remaining_verification_items"}, {"string", "2", []any{"00-Home.md", "coverage.md"}, "remaining_verification_items"}, {"duplicate", 2, []any{"00-Home.md", "./00-Home.md"}, "duplicate summary path"}, {"escape", 2, []any{"00-Home.md", "../escape.md"}, "invalid summary path"}, {"missing-home", 2, []any{"coverage.md", "coverage.md"}, "Home and linked"}, {"object-path", 2, []any{"00-Home.md", map[string]any{"x": 1}}, "invalid summary path"}} {
		t.Run(tc.name, func(t *testing.T) {
			root, cp := closureFixture(t)
			setClosureCheckpoint(t, cp, tc.count, 1, tc.paths)
			r, e := MapClosure(root, cp)
			if e == nil || !strings.Contains(strings.Join(r.Errors, "\n"), tc.want) {
				t.Fatalf("%+v %v", r, e)
			}
		})
	}
}
func TestMapClosureAmbiguousSummaryAndJSON(t *testing.T) {
	root, cp := closureFixture(t)
	if e := os.WriteFile(filepath.Join(root, "coverage.md"), []byte("Remaining: 2 verification items in 1 notes.\nRemaining: 2 verification items in 1 notes."), 0600); e != nil {
		t.Fatal(e)
	}
	r, e := MapClosure(root, cp)
	if e == nil || !strings.Contains(strings.Join(r.Errors, "\n"), "ambiguous") {
		t.Fatalf("%+v %v", r, e)
	}
	for _, bad := range []string{"null", "[]", "{} {}"} {
		if e = os.WriteFile(cp, []byte(bad), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e = MapClosure(root, cp); e == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
func TestMapClosureSymlinkEscape(t *testing.T) {
	root, cp := closureFixture(t)
	outside := filepath.Join(t.TempDir(), "coverage.md")
	if e := os.WriteFile(outside, []byte("Remaining: 2 verification items in 1 notes."), 0600); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(root, "outside.md")
	if e := os.Symlink(outside, link); e != nil {
		t.Skipf("symlinks unavailable: %v", e)
	}
	setClosureCheckpoint(t, cp, 2, 1, []any{"00-Home.md", "outside.md"})
	r, e := MapClosure(root, cp)
	if e == nil || !strings.Contains(strings.Join(r.Errors, "\n"), "invalid summary path") {
		t.Fatalf("%+v %v", r, e)
	}
}
