package investigation

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func closeFixture(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(filepath.Dir(root), ".investigations", sampleID, "investigation.md")
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = Run([]string{"close", "--root", root, "--id", sampleID, "--decision", "abandoned", "--reason", "closed", "--limitations", "not evaluated", "--source", "user:request", "--expected-public-sha256", digest(b)}, &out); e != nil {
		t.Fatal(e)
	}
}
func TestRetireUnpublished(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	closeFixture(t, root)
	p := filepath.Join(filepath.Dir(root), ".investigations", sampleID, "investigation.md")
	b, _ := os.ReadFile(p)
	args := []string{"retire", "--root", root, "--id", sampleID, "--expected-public-sha256", digest(b), "--reason", "completed review", "--source", "user:request", "--dependency-review", "none", "--absorption-review", "none", "--summary", "no longer needed"}
	var out bytes.Buffer
	if e := Run(args, &out); e == nil {
		t.Fatal("unauthorized retirement")
	}
	if e := Run(append(args, "--authorized"), &out); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Dir(p)); !os.IsNotExist(e) {
		t.Fatal("case not retired")
	}
}
func TestRetirePublishedAndReadLedger(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	closeFixture(t, root)
	p := filepath.Join(filepath.Dir(root), ".investigations", sampleID, "investigation.md")
	b, _ := os.ReadFile(p)
	h, _, _ := Snapshot(filepath.Dir(p))
	var out bytes.Buffer
	if e := Run([]string{"publish", "--root", root, "--id", sampleID, "--expected-public-sha256", digest(b), "--expected-tree-sha256", h, "--source", "user:publish"}, &out); e != nil {
		t.Fatal(e)
	}
	for _, args := range [][]string{{"add", "investigations"}, {"commit", "-m", "test: published case"}} {
		if b, e := exec.Command("git", append([]string{"-C", filepath.Dir(root)}, args...)...).CombinedOutput(); e != nil {
			t.Fatalf("git %v %s", e, b)
		}
	}
	commit, e := git(root, "rev-parse", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	p = filepath.Join(root, sampleID, "investigation.md")
	b, _ = os.ReadFile(p)
	if e = Run([]string{"retire", "--root", root, "--id", sampleID, "--expected-public-sha256", digest(b), "--reason", "completed review", "--source", "user:request", "--dependency-review", "none", "--absorption-review", "none", "--summary", "no longer needed", "--snapshot-commit", string(bytes.TrimSpace(commit)), "--authorized"}, &out); e != nil {
		t.Fatal(e)
	}
	out.Reset()
	if e = Run([]string{"list", "--root", root}, &out); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"commit_state":"pending"`)) {
		t.Fatal(out.String())
	}
	for _, args := range [][]string{{"add", "-A", "investigations"}, {"commit", "-m", "test: retire case"}} {
		if b, e := exec.Command("git", append([]string{"-C", filepath.Dir(root)}, args...)...).CombinedOutput(); e != nil {
			t.Fatalf("git %v %s", e, b)
		}
	}
	out.Reset()
	if e = Run([]string{"load", "--root", root, "--id", sampleID}, &out); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"commit_state":"committed"`)) {
		t.Fatal(out.String())
	}
}
func TestConsolidatePreservesArchiveAndLineage(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	other := "20260924-130000-other-case"
	_, e := openCase(root, options{"id": {other}, "title": {"Other case"}, "objective": {"Check"}, "dedupe-key": {"other-case"}, "purpose": {"knowledge"}, "vault-outcome": {"none"}, "learning-outcome": {"no-learning"}, "request-summary": {"Check"}})
	if e != nil {
		t.Fatal(e)
	}
	base := filepath.Join(filepath.Dir(root), ".investigations")
	canonical := filepath.Join(base, sampleID, "investigation.md")
	retiring := filepath.Join(base, other, "investigation.md")
	mapping := filepath.Join(filepath.Dir(canonical), "artifacts", "consolidation-"+other+"-mapping.md")
	if e = os.WriteFile(mapping, []byte("retired-id: "+other+"\ndrafts: none\nlearning-assessment: preserved\n"), 0600); e != nil {
		t.Fatal(e)
	}
	a, _ := os.ReadFile(canonical)
	b, _ := os.ReadFile(retiring)
	var out bytes.Buffer
	if e = Run([]string{"consolidate", "--root", root, "--canonical", sampleID, "--retire", other, "--expected-canonical-sha256", digest(a), "--expected-retire-sha256", digest(b)}, &out); e != nil {
		t.Fatal(e)
	}
	archived, e := os.ReadFile(filepath.Join(filepath.Dir(canonical), "artifacts", "consolidated", other, "investigation.md"))
	if e != nil || !bytes.Equal(archived, b) {
		t.Fatalf("archive differs %v", e)
	}
	if _, e = os.Stat(retiring); !os.IsNotExist(e) {
		t.Fatal("retiring case remains")
	}
	out.Reset()
	if e = Run([]string{"load", "--root", root, "--id", other}, &out); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"id":"`+sampleID+`"`)) {
		t.Fatal(out.String())
	}
}
