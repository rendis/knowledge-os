package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletePreparation(t *testing.T) {
	o := worktreeFixture(t)
	p, err := PlanComplete(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(p.Worktree["path"]); !os.IsNotExist(err) {
		t.Fatal("complete plan wrote worktree")
	}
	result, err := PrepareComplete(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "prepared" {
		t.Fatal(result)
	}
}

func TestCompletePlanRefusesExternalPolicyFilter(t *testing.T) {
	o := worktreeFixture(t)
	w, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	source := w.Source["path"]
	write(t, filepath.Join(source, "AGENTS.md"), []byte("# Repository\n"))
	write(t, filepath.Join(source, ".gitattributes"), []byte("AGENTS.md filter=external\n"))
	testGit(t, source, "add", "AGENTS.md", ".gitattributes")
	testGit(t, source, "commit", "-m", "filtered policy")
	testGit(t, source, "config", "filter.external.smudge", "false")
	if _, err = PlanComplete(o); err == nil || !strings.Contains(err.Error(), "external checkout filter") {
		t.Fatalf("expected early external filter refusal: %v", err)
	}
	if _, err = os.Stat(w.Worktree["path"]); !os.IsNotExist(err) {
		t.Fatal("planning created persistent worktree")
	}
}

func TestCompletePlanRejectsBaseOverride(t *testing.T) {
	o := worktreeFixture(t)
	p, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	source := p.Source["path"]
	write(t, filepath.Join(source, "AGENTS.override.md"), []byte("hides policy"))
	testGit(t, source, "add", "AGENTS.override.md")
	testGit(t, source, "commit", "-m", "override")
	if _, err = PlanComplete(o); err == nil {
		t.Fatal("accepted incompatible policy override")
	}
}

func TestCompletePreparationWithCRLF(t *testing.T) {
	o := worktreeFixture(t)
	w, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	source := w.Source["path"]
	write(t, filepath.Join(source, "AGENTS.md"), []byte("# Repository\n\nKeep this policy.\n"))
	testGit(t, source, "add", "AGENTS.md")
	testGit(t, source, "commit", "-m", "policy")
	testGit(t, source, "config", "core.autocrlf", "true")
	before := testGit(t, source, "status", "--porcelain=v1")
	p, err := PlanComplete(o)
	if err != nil {
		t.Fatal(err)
	}
	if after := testGit(t, source, "status", "--porcelain=v1"); after != before {
		t.Fatal("projection changed source index or worktree")
	}
	if _, err = PrepareComplete(o, p.Token); err != nil {
		t.Fatal(err)
	}
}
