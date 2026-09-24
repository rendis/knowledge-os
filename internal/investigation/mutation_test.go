package investigation

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", dir}, {"-C", dir, "config", "user.name", "Case Tester"}, {"-C", dir, "config", "user.email", "case@example.invalid"}} {
		if b, e := exec.Command("git", args...).CombinedOutput(); e != nil {
			t.Fatalf("git: %s %v", b, e)
		}
	}
	return filepath.Join(dir, "investigations")
}
func openFixture(t *testing.T, root string) {
	t.Helper()
	args := []string{"--root", root, "open", "--id", sampleID, "--title", "Investigation sample", "--objective", "Verify native operations", "--dedupe-key", "sample", "--purpose", "knowledge", "--vault-outcome", "none", "--learning-outcome", "no-learning", "--request-summary", "Verify native lifecycle.", "--timestamp", "2026-09-24T12:00:00Z"}
	var out bytes.Buffer
	if e := Run(args, &out); e != nil {
		t.Fatal(e)
	}
}
func TestNativeLifecycle(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	p := filepath.Join(filepath.Dir(root), ".investigations", sampleID, "investigation.md")
	read := func() []byte {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	b := read()
	var out bytes.Buffer
	args := []string{"transition", "--root", root, "--id", sampleID, "--to", "blocked", "--blocked-on", "missing evidence", "--reason", "need evidence", "--source", "user:request", "--expected-public-sha256", digest(b), "--timestamp", "2026-09-24T12:01:00Z"}
	if e := Run(args, &out); e != nil {
		t.Fatal(e)
	}
	after := read()
	if bytes.Equal(after, b) {
		t.Fatal("transition did not save")
	}
	if e := Run(args, &out); e == nil {
		t.Fatal("stale write accepted")
	}
	if !bytes.Equal(after, read()) {
		t.Fatal("failed stale write changed bytes")
	}
	args = []string{"close", "--root", root, "--id", sampleID, "--decision", "abandoned", "--reason", "no longer needed", "--limitations", "unverified cause", "--source", "user:request", "--expected-public-sha256", digest(after), "--timestamp", "2026-09-24T12:02:00Z"}
	if e := Run(args, &out); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(read(), []byte("closure-outcome: abandoned")) {
		t.Fatal("closure absent")
	}
	var validated bytes.Buffer
	if e := Run([]string{"validate", "--root", root}, &validated); e != nil {
		t.Fatal(e)
	}
}
func TestTemplateParity(t *testing.T) {
	b, e := os.ReadFile("../../kernel/.agents/skills/manage-investigation/assets/investigation-template.md")
	if os.IsNotExist(e) {
		t.Skip("source template absent")
	}
	if e != nil {
		t.Fatal(e)
	}
	if string(b) != caseTemplate {
		t.Fatal("embedded template drifted from canonical skill template")
	}
}
func TestOpenTitleDedupe(t *testing.T) {
	root := gitFixture(t)
	openFixture(t, root)
	o := options{"id": {"20260924-130000-other"}, "title": {"Ínvestigation sample"}, "objective": {"test"}, "dedupe-key": {"different"}, "purpose": {"knowledge"}, "vault-outcome": {"none"}, "learning-outcome": {"no-learning"}, "request-summary": {"test"}}
	v, e := openCase(root, o)
	if e != nil {
		t.Fatal(e)
	}
	m := v.(map[string]any)
	if m["status"] != "definite_match" {
		t.Fatalf("dedupe missed: %#v", v)
	}
}

func TestLifecyclePythonParity(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("reference Python unavailable")
	}
	script, e := filepath.Abs("../../kernel/.agents/skills/manage-investigation/scripts/investigation-case.py")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(script); e != nil {
		t.Skip("reference script absent")
	}
	native, old := gitFixture(t), gitFixture(t)
	base := []string{"open", "--id", sampleID, "--title", "Investigación sample", "--objective", "Verify native operations", "--dedupe-key", "sample", "--purpose", "knowledge", "--vault-outcome", "none", "--learning-outcome", "no-learning", "--request-summary", "Verify native lifecycle.", "--timestamp", "2026-09-24T12:00:00Z"}
	read := func(root string) []byte {
		b, e := os.ReadFile(filepath.Join(filepath.Dir(root), ".investigations", sampleID, "investigation.md"))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	runBoth := func(args []string) {
		t.Helper()
		var out bytes.Buffer
		if e := Run(append([]string{"--root", native}, args...), &out); e != nil {
			t.Fatal(e)
		}
		if b, e := exec.Command(python, append([]string{"-B", script, "--root", old}, args...)...).CombinedOutput(); e != nil {
			t.Fatalf("reference: %v %s", e, b)
		}
		if !bytes.Equal(read(native), read(old)) {
			t.Fatalf("byte parity failure\nNative:%s\nPython:%s", read(native), read(old))
		}
	}
	runBoth(base)
	runBoth([]string{"transition", "--id", sampleID, "--to", "blocked", "--blocked-on", "missing evidence", "--reason", "need evidence", "--source", "user:request", "--expected-public-sha256", digest(read(native)), "--timestamp", "2026-09-24T12:01:00Z"})
	runBoth([]string{"close", "--id", sampleID, "--decision", "abandoned", "--reason", "no longer needed", "--limitations", "unverified cause", "--source", "user:request", "--expected-public-sha256", digest(read(native)), "--timestamp", "2026-09-24T12:02:00Z"})
}
