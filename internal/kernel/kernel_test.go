package kernel

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func install(t *testing.T) string {
	t.Helper()
	v := t.TempDir()
	os.WriteFile(filepath.Join(v, "instance.yaml"), []byte("version: 1\ncell:\n  name: C\n  purpose: p\nsystems:\n  - id: s\n    name: S\nadapters: []\n"), 0o644)
	os.WriteFile(filepath.Join(v, "00-Home.md"), []byte("# Home\n"), 0o644)
	os.WriteFile(filepath.Join(v, LockName), []byte("version: \"5\"\nkernel_version: \"0.0.1\"\ndistribution_revision: \"x\"\ndistribution_dirty: false\nadapters:\n  []\nmanaged_hashes:\n"), 0o644)
	p, e := Preview(v)
	if e != nil {
		t.Fatal(e)
	}
	if e := Apply(v, p); e != nil {
		t.Fatal(e)
	}
	return v
}

func run(t *testing.T, args ...string) (map[string]any, error) {
	var b bytes.Buffer
	e := Run(args, &b)
	m := map[string]any{}
	_ = json.Unmarshal(b.Bytes(), &m)
	return m, e
}

func TestUpdateInstallsPreviewsAndProtectsLocalChanges(t *testing.T) {
	v := install(t)
	lock, e := ReadLock(v)
	if e != nil || lock.KernelVersion != Version() || lock.Hashes["AGENTS.md"] == "" {
		t.Fatalf("lock after install: %v %+v", e, lock)
	}
	if target, _ := os.Readlink(filepath.Join(v, ".claude", "skills")); target != filepath.Join("..", ".agents", "skills") {
		t.Fatalf(".claude/skills link: %q", target)
	}
	if b, _ := os.ReadFile(filepath.Join(v, ".gitignore")); !strings.Contains(string(b), "/.investigations/") || !strings.Contains(string(b), "/.operations/") {
		t.Fatalf("local stores are ignored: %s", b)
	}
	if res, _ := run(t, "status", "--vault", v); res["current"] != true {
		t.Fatalf("a fresh install is current: %v", res)
	}

	// A kernel file the distribution no longer ships (recorded in the lock) is retired; a changed one is listed.
	os.WriteFile(filepath.Join(v, "90-Meta", "retired.md"), []byte("old\n"), 0o644)
	lock.Hashes["90-Meta/retired.md"] = digest([]byte("old\n"))
	lock.Hashes["AGENTS.md"] = digest([]byte("previous router\n"))
	os.WriteFile(filepath.Join(v, "AGENTS.md"), []byte("previous router\n"), 0o644)
	os.WriteFile(filepath.Join(v, LockName), []byte(lock.dump()), 0o644)
	res, e := run(t, "update", "--vault", v, "--dry-run")
	if e != nil || res["changes"] != float64(2) || !strings.Contains(res["diff"].(string), "AGENTS.md") {
		t.Fatalf("dry run lists the change and the retirement: %v %v", e, res)
	}
	if b, _ := os.ReadFile(filepath.Join(v, "AGENTS.md")); string(b) != "previous router\n" {
		t.Fatal("a dry run writes nothing")
	}

	// A managed file edited in the vault since installation blocks the update until --force.
	os.WriteFile(filepath.Join(v, "90-Meta", "retired.md"), []byte("edited by the cell\n"), 0o644)
	if res, e := run(t, "update", "--vault", v); e == nil || res["status"] != "conflict" {
		t.Fatalf("local edits are protected: %v %v", e, res)
	}
	if res, e := run(t, "update", "--vault", v, "--force"); e != nil || res["status"] != "updated" {
		t.Fatalf("--force restores the distribution: %v %v", e, res)
	}
	if _, e := os.Stat(filepath.Join(v, "90-Meta", "retired.md")); !os.IsNotExist(e) {
		t.Fatal("the retired file is removed")
	}
	if res, _ := run(t, "status", "--vault", v); res["current"] != true {
		t.Fatalf("current after update: %v", res)
	}
}

func TestUpdateKeepsCellSkills(t *testing.T) {
	v := install(t)
	own := filepath.Join(v, ".agents", "skills", "cell-skill", "SKILL.md")
	os.MkdirAll(filepath.Dir(own), 0o755)
	os.WriteFile(own, []byte("cell skill\n"), 0o644)

	// A cell file at a path the kernel now ships, which the lock never recorded, is never replaced.
	shipped := ".agents/skills/use-vault-cli/SKILL.md"
	lock, _ := ReadLock(v)
	delete(lock.Hashes, shipped)
	os.WriteFile(filepath.Join(v, LockName), []byte(lock.dump()), 0o644)
	os.WriteFile(filepath.Join(v, filepath.FromSlash(shipped)), []byte("cell skill with a kernel name\n"), 0o644)
	for _, flags := range [][]string{{}, {"--force"}} {
		res, e := run(t, append([]string{"update", "--vault", v}, flags...)...)
		if e == nil || res["status"] != "foreign" {
			t.Fatalf("a cell file at a kernel path is refused %v: %v %v", flags, e, res)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(v, filepath.FromSlash(shipped))); string(b) != "cell skill with a kernel name\n" {
		t.Fatal("the cell file is kept")
	}

	os.Remove(filepath.Join(v, filepath.FromSlash(shipped)))
	if res, e := run(t, "update", "--vault", v, "--force"); e != nil || res["status"] != "updated" {
		t.Fatalf("update after the cell file moved: %v %v", e, res)
	}
	if b, _ := os.ReadFile(own); string(b) != "cell skill\n" {
		t.Fatal("a cell skill survives the update")
	}
}

func TestUpdateRejectsInvalidSettingsBeforeChangingFiles(t *testing.T) {
	for _, settings := range []string{"invalid JSON", "null"} {
		t.Run(settings, func(t *testing.T) {
			v := install(t)
			if e := os.WriteFile(filepath.Join(v, "AGENTS.md"), []byte("local router edit\n"), 0o644); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(v, ".gitignore"), []byte("local-ignore\n"), 0o644); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(v, ".obsidian", "app.json"), []byte(settings), 0o644); e != nil {
				t.Fatal(e)
			}
			before := map[string][]byte{}
			for _, rel := range []string{"AGENTS.md", ".gitignore", ".obsidian/app.json", LockName} {
				b, e := os.ReadFile(filepath.Join(v, filepath.FromSlash(rel)))
				if e != nil {
					t.Fatal(e)
				}
				before[rel] = b
			}
			if _, e := Update(v, Options{Force: true}); e == nil || !strings.Contains(e.Error(), "app.json") {
				t.Fatalf("invalid settings must stop the update: %v", e)
			}
			for rel, want := range before {
				if got, _ := os.ReadFile(filepath.Join(v, filepath.FromSlash(rel))); !bytes.Equal(got, want) {
					t.Errorf("a refused update changed %s", rel)
				}
			}
		})
	}
}

func TestSourcesIncludeSelectedAdaptersOnly(t *testing.T) {
	plain, e := Sources(nil)
	if e != nil {
		t.Fatal(e)
	}
	with, e := Sources([]string{"reports"})
	if e != nil || len(with) <= len(plain) {
		t.Fatalf("an adapter adds its skills: %v %d %d", e, len(plain), len(with))
	}
	if _, ok := plain[catalogPath]; ok {
		t.Fatal("a cell's catalog is never payload")
	}
	if _, e := Sources([]string{"nope"}); e == nil {
		t.Fatal("an unknown adapter is refused")
	}
}

func TestGatesWorkflowRunsOnTheCellRunner(t *testing.T) {
	v := install(t)
	wf := filepath.Join(v, ".github", "workflows", "knowledge-gates.yml")
	if b, _ := os.ReadFile(wf); !strings.Contains(string(b), "runs-on: ubuntu-latest") {
		t.Fatalf("the default runner is GitHub-hosted: %s", b)
	}
	if b, _ := os.ReadFile(wf); !strings.Contains(string(b), "ref: ${{ github.event.pull_request.head.sha || github.sha }}") {
		t.Fatalf("a pull request verifies the branch head, not GitHub's temporary merge: %s", b)
	}
	os.WriteFile(filepath.Join(v, "instance.yaml"), []byte("version: 1\ncell:\n  name: C\n  purpose: p\nsystems:\n  - id: s\n    name: S\nadapters: []\nci:\n  runner: corp-runner\n"), 0o644)
	if res, e := run(t, "update", "--vault", v); e != nil {
		t.Fatalf("update after changing ci.runner: %v %v", e, res)
	}
	b, _ := os.ReadFile(wf)
	if !strings.Contains(string(b), "    runs-on: corp-runner\n") || strings.Contains(string(b), "ubuntu-latest\n") {
		t.Fatalf("the workflow runs on the cell's runner: %s", b)
	}
	if res, _ := run(t, "status", "--vault", v); res["current"] != true {
		t.Fatalf("the rendered workflow is the kernel's content, not a local edit: %v", res)
	}
}
