package investigation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalRecoveryAndConflict(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	p := filepath.Join(filepath.Dir(root), ".investigations", sampleID, "investigation.md")
	before, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	after := append(append([]byte{}, before...), []byte("\nReviewed.\n")...)
	privateRel := ".investigations-private/" + sampleID + "/private.md"
	j := writeJournal{1, []fileChange{{".investigations/" + sampleID + "/investigation.md", before, after}, {privateRel, nil, []byte("private data")}}}
	b, _ := json.Marshal(j)
	if e = os.MkdirAll(filepath.Dir(journalPath(root)), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(journalPath(root), b, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p, after, 0600); e != nil {
		t.Fatal(e)
	} // interrupted after first file
	var out bytes.Buffer
	if e = Run([]string{"load", "--root", root, "--id", sampleID}, &out); e == nil {
		t.Fatal("partially committed transaction exposed")
	}
	if e = recoverJournal(root); e != nil {
		t.Fatal(e)
	}
	actual, e := os.ReadFile(filepath.Join(filepath.Dir(root), filepath.FromSlash(privateRel)))
	if e != nil || string(actual) != "private data" {
		t.Fatalf("recovery failed: %v", e)
	}
	if _, e = os.Stat(journalPath(root)); !os.IsNotExist(e) {
		t.Fatal("journal not cleared")
	}
	if e = os.WriteFile(journalPath(root), b, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p, []byte("user concurrent edit"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = recoverJournal(root); e == nil {
		t.Fatal("concurrent edit overwritten")
	}
	actual, _ = os.ReadFile(p)
	if string(actual) != "user concurrent edit" {
		t.Fatal("conflict changed user file")
	}
}
func TestSaveAndPrivateCAS(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	p := filepath.Join(filepath.Dir(root), ".investigations", sampleID, "investigation.md")
	before, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	candidate := strings.Replace(string(before), "## Evidence\n", "## Evidence\n\n- E-001: reviewed evidence\n", 1)
	cp := filepath.Join(t.TempDir(), "candidate.md")
	if e = os.WriteFile(cp, []byte(candidate), 0600); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	args := []string{"save", "--root", root, "--id", sampleID, "--public-candidate", cp, "--expected-public-sha256", digest(before), "--private-root", filepath.Join(filepath.Dir(root), ".investigations-private"), "--target", "E-001", "--source", "user:review", "--timestamp", "2026-09-24T12:01:00Z"}
	if e = Run(args, &out); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Contains(after, []byte("Updated registers `E-001`")) {
		t.Fatal("save attribution missing")
	}
	if e = Run(args, &out); e == nil {
		t.Fatal("stale save succeeded")
	}
}
