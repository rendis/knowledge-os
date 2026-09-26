package vaults

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"knowledge-os/internal/kernel"
)

func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KOS_VAULTS_FILE", filepath.Join(dir, "vaults.json"))
	t.Setenv("CI", "")
	empty := filepath.Join(dir, "gitconfig")
	os.WriteFile(empty, nil, 0o644)
	for k, v := range map[string]string{"GIT_CONFIG_GLOBAL": empty, "GIT_CONFIG_NOSYSTEM": "1", "GIT_AUTHOR_NAME": "t",
		"GIT_AUTHOR_EMAIL": "t@example.com", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com"} {
		t.Setenv(k, v)
	}
	return canonical(dir)
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if b, e := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); e != nil {
		t.Fatalf("git %v: %s", args, b)
	}
}

// vault installs the current kernel in a Git checkout of cell under parent.
func vault(t *testing.T, parent, name, cell string) string {
	t.Helper()
	v := filepath.Join(parent, name)
	os.MkdirAll(v, 0o755)
	os.WriteFile(filepath.Join(v, "instance.yaml"), []byte("version: 1\ncell:\n  name: "+cell+"\n  purpose: p\nsystems:\n  - id: s\n    name: S\nadapters: []\n"), 0o644)
	os.WriteFile(filepath.Join(v, "00-Home.md"), []byte("# Home\n"), 0o644)
	os.WriteFile(filepath.Join(v, kernel.LockName), []byte("version: \"5\"\nkernel_version: \"0.0.1\"\nadapters:\n  []\nmanaged_hashes:\n"), 0o644)
	p, e := kernel.Preview(v)
	if e != nil {
		t.Fatal(e)
	}
	if e := kernel.Apply(v, p); e != nil {
		t.Fatal(e)
	}
	run(t, v, "init", "-q", "-b", "main")
	run(t, v, "add", "-A")
	run(t, v, "commit", "-q", "-m", "init")
	return v
}

// age makes the vault carry an older kernel with one managed file missing, as a release would find it.
func age(t *testing.T, v string) {
	t.Helper()
	lock := filepath.Join(v, kernel.LockName)
	b, _ := os.ReadFile(lock)
	os.WriteFile(lock, bytes.Replace(b, []byte(`kernel_version: "`+kernel.Version()+`"`), []byte(`kernel_version: "0.0.1"`), 1), 0o644)
	os.Remove(filepath.Join(v, "AGENTS.md"))
	os.Remove(filepath.Join(v, "Repos.base"))
	run(t, v, "add", "-A")
	run(t, v, "commit", "-q", "-m", "old kernel")
}

func states(t *testing.T) map[string]State {
	t.Helper()
	list, e := List()
	if e != nil {
		t.Fatal(e)
	}
	out := map[string]State{}
	for _, s := range list {
		out[s.Path] = s
	}
	return out
}

func TestListChecksEveryVaultAtItsRecordedPath(t *testing.T) {
	dir := setup(t)
	a := vault(t, dir, "a", "A")
	b := vault(t, dir, "b", "B")
	age(t, b)
	for _, v := range []string{a, b} {
		if s, e := remember(v, false); e != nil || s != "new" {
			t.Fatalf("remember %s: %s %v", v, s, e)
		}
	}
	if s, _ := remember(a, false); s != "known" {
		t.Fatalf("a known vault is not added twice: %s", s)
	}
	got := states(t)
	if got[a].Found != "yes" || got[a].Kernel != "current" || got[a].Branch != "main" || got[b].Kernel != "outdated" {
		t.Fatalf("found vaults report branch and kernel: %+v", got)
	}

	// A vault moved away is reported, not dropped; a scan finds it and moves the same entry.
	moved := filepath.Join(dir, "elsewhere", "a")
	os.MkdirAll(filepath.Dir(moved), 0o755)
	os.Rename(a, moved)
	if s := states(t)[a]; s.Found != "missing" || !strings.Contains(s.Fix, "vaults scan") {
		t.Fatalf("a vault not at its path is reported with its fix: %+v", s)
	}
	found, e := Scan(dir, 6)
	if e != nil {
		t.Fatal(e)
	}
	statuses := map[string]string{}
	for _, f := range found {
		statuses[f["path"]] = f["status"]
	}
	if statuses[moved] != "relocated" || statuses[b] != "known" {
		t.Fatalf("scan: %v", found)
	}
	got = states(t)
	if len(got) != 2 || got[moved].Found != "yes" {
		t.Fatalf("the moved vault keeps one entry at its new path: %+v", got)
	}

	// Another cell at a recorded path is not taken for the remembered one.
	os.WriteFile(filepath.Join(b, "instance.yaml"), []byte("version: 1\ncell:\n  name: Other\n  purpose: p\nsystems:\n  - id: s\n    name: S\n"), 0o644)
	if s := states(t)[b]; s.Found != "other-cell" {
		t.Fatalf("a different cell at the path is reported: %+v", s)
	}
	if _, e := remember(filepath.Join(dir, "elsewhere"), false); e == nil {
		t.Fatal("a directory without a vault is refused")
	}
}

func TestUpdateAllUpdatesOnlyOutdatedVaults(t *testing.T) {
	dir := setup(t)
	current := vault(t, dir, "current", "Current")
	old := vault(t, dir, "old", "Old")
	age(t, old)
	busy := vault(t, dir, "busy", "Busy")
	age(t, busy)
	os.WriteFile(filepath.Join(busy, "00-Home.md"), []byte("# Home, edited\n"), 0o644)
	gone := vault(t, dir, "gone", "Gone")
	foreign := vault(t, dir, "foreign", "Foreign")
	age(t, foreign)
	lock := filepath.Join(foreign, kernel.LockName)
	b, _ := os.ReadFile(lock)
	lines := []string{}
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.Contains(l, ".agents/skills/use-vault-cli/SKILL.md") {
			lines = append(lines, l)
		}
	}
	os.WriteFile(lock, []byte(strings.Join(lines, "\n")), 0o644)
	os.WriteFile(filepath.Join(foreign, ".agents", "skills", "use-vault-cli", "SKILL.md"), []byte("the cell's own\n"), 0o644)
	run(t, foreign, "commit", "-qam", "a cell skill with a kernel name")
	for _, v := range []string{current, old, busy, gone, foreign} {
		remember(v, false)
	}
	os.RemoveAll(gone)
	head := func(v string) string {
		out, _ := exec.Command("git", "-C", v, "rev-parse", "HEAD").Output()
		return string(out)
	}
	before := head(old)

	res, e := UpdateAll(kernel.Options{DryRun: true}, false)
	if e == nil || head(old) != before {
		t.Fatalf("a dry run writes nothing: %v", e)
	}
	if _, e := os.Stat(filepath.Join(old, "AGENTS.md")); !os.IsNotExist(e) {
		t.Fatal("a dry run restores nothing")
	}
	actions := func(res map[string]any) map[string]string {
		out := map[string]string{}
		for _, item := range res["vaults"].([]map[string]any) {
			out[filepath.Base(item["path"].(string))] = item["action"].(string)
		}
		return out
	}
	if a := actions(res); a["old"] != "would-update" || a["busy"] != "would-update" || a["foreign"] != "would-refuse" || a["current"] != "skipped" || a["gone"] != "skipped" {
		t.Fatalf("dry run actions: %v", a)
	}

	res, e = UpdateAll(kernel.Options{DryRun: true}, true)
	if a := actions(res); a["busy"] != "would-refuse" || a["old"] != "would-update" {
		t.Fatalf("a dry run with --commit previews the commit refusals: %v", a)
	}
	res, e = UpdateAll(kernel.Options{}, true)
	if e == nil || !strings.Contains(e.Error(), "2 vault") {
		t.Fatalf("the busy vault is reported: %v", e)
	}
	if a := actions(res); a["old"] != "updated" || a["busy"] != "refused" || a["foreign"] != "refused" || a["current"] != "skipped" || a["gone"] != "skipped" {
		t.Fatalf("actions: %v", a)
	}
	if head(old) == before {
		t.Fatal("the outdated vault is committed")
	}
	if out, _ := exec.Command("git", "-C", old, "status", "--porcelain").Output(); len(out) != 0 {
		t.Fatalf("the commit holds everything the update wrote: %s", out)
	}
	msg, _ := exec.Command("git", "-C", old, "log", "-1", "--format=%s").Output()
	if strings.TrimSpace(string(msg)) != "chore: update the knowledge-os kernel to "+kernel.Version() {
		t.Fatalf("commit message: %s", msg)
	}
	if s := states(t)[old]; s.Kernel != "current" {
		t.Fatalf("the updated vault is current: %+v", s)
	}
	if b, _ := os.ReadFile(filepath.Join(busy, "00-Home.md")); string(b) != "# Home, edited\n" {
		t.Fatal("a refused vault is left as it was")
	}

	var out bytes.Buffer
	if e := Run([]string{"list"}, &out); e != nil {
		t.Fatal(e)
	}
	var list map[string]any
	json.Unmarshal(out.Bytes(), &list)
	if next, _ := list["next"].(string); !strings.Contains(next, "kernel update --all") {
		t.Fatalf("the list names the next step while a vault is outdated: %v", list["next"])
	}
}

func TestConcurrentRegistrationsKeepEveryVault(t *testing.T) {
	dir := setup(t)
	paths := []string{}
	for i := 0; i < 20; i++ {
		v := filepath.Join(dir, "v", string(rune('a'+i)))
		os.MkdirAll(v, 0o755)
		os.WriteFile(filepath.Join(v, "instance.yaml"), []byte("version: 1\ncell:\n  name: C"+string(rune('a'+i))+"\n  purpose: p\nsystems:\n  - id: s\n    name: S\n"), 0o644)
		os.WriteFile(filepath.Join(v, kernel.LockName), []byte("version: \"5\"\nkernel_version: \"0.0.1\"\n"), 0o644)
		paths = append(paths, v)
	}
	done := make(chan error)
	for _, v := range paths {
		go func(v string) { _, e := remember(v, false); done <- e }(v)
	}
	for range paths {
		if e := <-done; e != nil {
			t.Fatal(e)
		}
	}
	if got := states(t); len(got) != len(paths) {
		t.Fatalf("every concurrent registration is kept: %d of %d", len(got), len(paths))
	}
}
