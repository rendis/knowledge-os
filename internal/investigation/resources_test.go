package investigation

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishAndResources(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	source := filepath.Join(filepath.Dir(root), ".investigations", sampleID)
	b, e := os.ReadFile(filepath.Join(source, "investigation.md"))
	if e != nil {
		t.Fatal(e)
	}
	tree, _, e := Snapshot(source)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = Run([]string{"publish", "--root", root, "--id", sampleID, "--expected-public-sha256", digest(b), "--expected-tree-sha256", tree, "--source", "user:publish", "--timestamp", "2026-09-24T12:01:00Z"}, &out); e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(root, sampleID)
	if _, e = os.Stat(source); !os.IsNotExist(e) {
		t.Fatal("unpublished source remains")
	}
	if _, e = os.Stat(filepath.Join(dest, "investigation.md")); e != nil {
		t.Fatal(e)
	}
	candidate := filepath.Join(t.TempDir(), "case")
	if e = copyTree(dest, candidate); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(candidate, "artifacts", "A-001-sample.txt")
	content := []byte("Evidence bytes")
	if e = os.WriteFile(file, content, 0600); e != nil {
		t.Fatal(e)
	}
	cp := filepath.Join(candidate, "investigation.md")
	b, e = os.ReadFile(cp)
	if e != nil {
		t.Fatal(e)
	}
	text := strings.Replace(string(b), "## References and attachments\n", "## References and attachments\n\n- A-001: artifact\n  artifacts/A-001-sample.txt SHA-256 "+digest(content)+"\n", 1)
	if e = os.WriteFile(cp, []byte(text), 0600); e != nil {
		t.Fatal(e)
	}
	before, _, e := Snapshot(dest)
	if e != nil {
		t.Fatal(e)
	}
	after, _, e := Snapshot(candidate)
	if e != nil {
		t.Fatal(e)
	}
	if e = Run([]string{"save-resources", "--root", root, "--id", sampleID, "--candidate-dir", candidate, "--expected-tree-sha256", before, "--expected-candidate-tree-sha256", after, "--target", "A-001", "--source", "user:review", "--timestamp", "2026-09-24T12:02:00Z"}, &out); e != nil {
		t.Fatal(e)
	}
	actual, e := os.ReadFile(filepath.Join(dest, "artifacts", "A-001-sample.txt"))
	if e != nil || !bytes.Equal(actual, content) {
		t.Fatalf("artifact missing: %v", e)
	}
	if e = Run([]string{"validate", "--root", root}, &out); e != nil {
		t.Fatal(e)
	}
}
func TestTreeRecoveryAfterSourceRename(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	source := filepath.Join(filepath.Dir(root), ".investigations", sampleID)
	staging := filepath.Join(filepath.Dir(source), ".native-tree-fixture")
	if e := copyTree(source, staging); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(staging, "investigation.md")
	if e := os.WriteFile(p, []byte("replacement bytes"), 0600); e != nil {
		t.Fatal(e)
	}
	before, _, _ := Snapshot(source)
	after, _, _ := Snapshot(staging)
	backup := staging + ".backup"
	if e := os.Rename(source, backup); e != nil {
		t.Fatal(e)
	}
	rel := func(p string) string { s, _ := filepath.Rel(filepath.Dir(root), p); return filepath.ToSlash(s) }
	j := treeJournal{1, rel(source), rel(source), rel(staging), rel(backup), before, after, "", ""}
	if e := os.MkdirAll(filepath.Dir(treeJournalPath(root)), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(treeJournalPath(root), []byte("pending"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := replayTree(root, j); e != nil {
		t.Fatal(e)
	}
	h, _, e := Snapshot(source)
	if e != nil || h != after {
		t.Fatalf("tree recovery failed: %v", e)
	}
}
