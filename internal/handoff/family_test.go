package handoff

import (
	"go.yaml.in/yaml/v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func familyFixture(t *testing.T) FamilyOptions {
	t.Helper()
	o := worktreeFixture(t)
	p, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	created, err := CreateWorktree(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	return FamilyOptions{Vault: o.Vault, Bundle: o.Bundle, Worktree: created.Worktree["path"]}
}

func TestLegacyActiveAdoptedWithoutRevision(t *testing.T) {
	o := familyFixture(t)
	p, err := PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ApplyFamily(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	a, err := registry(o.Worktree, false)
	if err != nil {
		t.Fatal(err)
	}
	e := a.Entries[0]
	raw, err := yaml.Marshal(map[string]any{"schema-version": 1, "handoff-id": e.ID, "family": e.Family, "revision": e.Revision, "manifest": e.Manifest, "activated-at": e.ActivatedAt})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(o.Worktree, storeName, "ACTIVE.yaml"), raw)
	p, err = PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Action != "activate" {
		t.Fatalf("legacy wrapper must migrate: %s", p.Action)
	}
	p, err = ApplyFamily(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if p.Handoff["revision"] != "v0001" {
		t.Fatal("legacy wrapper upgrade should not create revision")
	}
	raw, err = os.ReadFile(filepath.Join(o.Worktree, storeName, "ACTIVE.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var upgraded activeRegistry
	if err = decode(raw, &upgraded); err != nil {
		t.Fatal(err)
	}
	if upgraded.Schema != 2 {
		t.Fatal("legacy wrapper not upgraded")
	}
}

func TestPythonReadsNativeFamily(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python compatibility oracle unavailable")
	}
	if err = exec.Command(python, "-c", "import ruamel.yaml").Run(); err != nil {
		t.Skip("ruamel compatibility oracle unavailable")
	}
	if _, err = os.Stat(filepath.Join("..", "..", "kernel", ".agents", "skills", "manage-development-handoff", "scripts", "development-handoff.py")); err != nil {
		t.Skip("Python oracle retired with the legacy handoff workflow")
	}
	o := familyFixture(t)
	p, err := PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ApplyFamily(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("../../kernel/.agents/skills/manage-development-handoff/scripts/development-handoff.py")
	if err != nil {
		t.Fatal(err)
	}
	code := `import importlib.util,sys,pathlib
s=importlib.util.spec_from_file_location("compat_handoff",sys.argv[1]);m=importlib.util.module_from_spec(s);sys.modules[s.name]=m;s.loader.exec_module(m)
root=pathlib.Path(sys.argv[2]);family=sys.argv[3]
m.read_existing_handoff(root/".knowledge-os-handoffs"/family,family=family,handoff_id=sys.argv[4],normalized_remote=sys.argv[5])
m.read_active(root/".knowledge-os-handoffs")
`
	data, err := exec.Command(python, "-B", "-c", code, script, o.Worktree, p.Handoff["family"].(string), p.Handoff["id"].(string), p.Target["remote"]).CombinedOutput()
	if err != nil {
		t.Fatalf("existing Python rejects native persisted family: %v\n%s", err, data)
	}
}

func TestFamilyCreateUpdateNoop(t *testing.T) {
	o := familyFixture(t)
	p, err := PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Action != "create" {
		t.Fatal(p.Action)
	}
	applied, err := ApplyFamily(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Status != "applied" {
		t.Fatal(applied)
	}
	check, err := ValidateFamilyRepository(o.Vault, "https://example.invalid/team/Example.git", o.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	if check["status"] != "valid" {
		t.Fatal(check)
	}
	p, err = PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Action != "noop" {
		t.Fatalf("expected noop: %+v", p)
	}
	write(t, filepath.Join(o.Bundle, "context.md"), []byte("# New context\n"))
	p, err = PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Action != "update" {
		t.Fatal(p.Action)
	}
	applied, err = ApplyFamily(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Handoff["revision"] != "v0002" {
		t.Fatal(applied)
	}
	if _, err = Inspect(o.Worktree); err != nil {
		t.Fatal(err)
	}
}

func TestFamilyChangedSourceRejectsApproval(t *testing.T) {
	o := familyFixture(t)
	p, err := PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(o.Worktree, "AGENTS.md"), []byte("# Consumer policy\n"))
	if _, err = ApplyFamily(o, p.Token); err == nil || !strings.Contains(err.Error(), "plan_stale") {
		t.Fatalf("expected stale: %v", err)
	}
	if _, err = os.Stat(filepath.Join(o.Worktree, storeName, "ACTIVE.yaml")); !os.IsNotExist(err) {
		t.Fatal("stale approval materialized family")
	}
}

func TestInvalidTargetMutationDoesNotCreateStore(t *testing.T) {
	o := familyFixture(t)
	invalid := t.TempDir()
	o.Worktree = invalid
	if _, err := ApplyFamily(o, strings.Repeat("a", 64)); err == nil {
		t.Fatal("accepted foreign target")
	}
	if _, err := os.Lstat(filepath.Join(invalid, storeName)); !os.IsNotExist(err) {
		t.Fatal("invalid target was mutated")
	}
}

func TestNoopRejectsTrackedHandoffStore(t *testing.T) {
	o := familyFixture(t)
	p, err := PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ApplyFamily(o, p.Token); err != nil {
		t.Fatal(err)
	}
	testGit(t, o.Worktree, "add", "-f", storeName+"/ACTIVE.yaml")
	if _, err = PlanFamily(o); err == nil {
		t.Fatal("noop accepted tracked handoff store")
	}
}

func TestRegistryRejectsStoreSymlink(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, storeName)); err != nil {
		t.Skip(err)
	}
	if _, err := registry(root, false); err == nil {
		t.Fatal("registry followed store symlink")
	}
}

func TestStateClosureAndTransition(t *testing.T) {
	o := familyFixture(t)
	p, err := PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ApplyFamily(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	id := p.Handoff["id"].(string)
	family := p.Handoff["family"].(string)
	remote := p.Target["remote"]
	if _, err = SetState(o.Vault, remote, o.Worktree, id, "production", "", ""); err == nil {
		t.Fatal("allowed direct production")
	}
	if _, err = SetState(o.Vault, remote, o.Worktree, id, "ready-for-production", "", ""); err == nil {
		t.Fatal("allowed missing closure")
	}
	closure, err := ClosureFingerprint(o.Worktree, family)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := SetState(o.Vault, remote, o.Worktree, id, "ready-for-production", closure, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SetState(o.Vault, remote, o.Worktree, id, "ready-for-production", closure, plan["plan_token"].(string)); err != nil {
		t.Fatal(err)
	}
	plan, err = SetState(o.Vault, remote, o.Worktree, id, "production", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SetState(o.Vault, remote, o.Worktree, id, "production", "", plan["plan_token"].(string)); err != nil {
		t.Fatal(err)
	}
}

func TestClosureTracksUntrackedContent(t *testing.T) {
	o := familyFixture(t)
	p, err := PlanFamily(o)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ApplyFamily(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	family := p.Handoff["family"].(string)
	write(t, filepath.Join(o.Worktree, "evidence.txt"), []byte("before"))
	before, err := ClosureFingerprint(o.Worktree, family)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(o.Worktree, "evidence.txt"), []byte("after!"))
	after, err := ClosureFingerprint(o.Worktree, family)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("closure ignored untracked content")
	}
}
