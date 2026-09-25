package config

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const instanceFixture = `version: 1
cell:
  name: Example
  purpose: Test vault
systems:
  - id: demo
    name: Demo
    aliases: []
evidence:
  profile: documented-source
locale:
  notes: es
# Keep customer fields
custom:
  answer: 42 # keep inline
capabilities:
  # keep capability comment
  read-db: [Inspect DB]
`

func write(t *testing.T, root, path, content string) {
	t.Helper()
	p := filepath.Join(root, path)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
}
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "instance.yaml", instanceFixture)
	write(t, root, "60-Operacion/Data/Inspect DB.md", "---\ntipo: operacional\narea: Data\nclase: procedimiento\n---\nRead only")
	return root
}
func TestBindPreservesConsumerFieldsAndComments(t *testing.T) {
	root := fixture(t)
	b, _ := os.ReadFile(filepath.Join(root, "instance.yaml"))
	hash := fmt.Sprintf("%x", sha256.Sum256(b))
	if _, e := Bind(root, "database-inspection", []string{"Inspect DB"}, hash); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(root, "instance.yaml"))
	for _, want := range []string{"# Keep customer fields", "answer: 42 # keep inline", "# keep capability comment"} {
		if !bytes.Contains(after, []byte(want)) {
			t.Errorf("lost %s", want)
		}
	}
	if _, e := Bind(root, "database-inspection", []string{"Inspect DB"}, hash); e == nil {
		t.Fatal("accepted stale hash")
	}
	m, e := LoadInstance(root)
	if e != nil || obj(m["custom"])["answer"] != 42 {
		t.Fatal("lost custom", e)
	}
}
func TestBindInvalidProcedureDoesNotWrite(t *testing.T) {
	root := fixture(t)
	before, _ := os.ReadFile(filepath.Join(root, "instance.yaml"))
	if _, e := Bind(root, "read-db", []string{"Missing"}, ""); e == nil {
		t.Fatal("accepted missing")
	}
	after, _ := os.ReadFile(filepath.Join(root, "instance.yaml"))
	if !bytes.Equal(before, after) {
		t.Fatal("changed on failure")
	}
}
func TestConcurrentWorkspaceUpdates(t *testing.T) {
	root := fixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := UpdateWorkspace(root, WorkspaceUpdate{Ports: map[string]string{fmt.Sprintf("env%d", i): fmt.Sprint(5000 + i)}})
			errs <- e
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	w, e := Workspace(root)
	if e != nil {
		t.Fatal(e)
	}
	if len(obj(w["proxy_ports"])) != 8 {
		t.Fatal("lost concurrent update", w)
	}
}
func TestWorkspaceCommentsAndUnknownFields(t *testing.T) {
	root := fixture(t)
	write(t, root, WorkspaceFile, "version: 1\n# customer\ncustom: retained\nworkspace:\n  repository_roots: []\n  local: yes\nskills:\n  inspect-database:\n    environments:\n      prod:\n        proxy_port: 5432\n        note: keep\n")
	if _, e := UpdateWorkspace(root, WorkspaceUpdate{Ports: map[string]string{"prod": "6000"}}); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(root, WorkspaceFile))
	for _, want := range []string{"# customer", "custom: retained", "local: yes", "note: keep"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Fatal("lost", want)
		}
	}
}
func TestSymlinkConfigRejected(t *testing.T) {
	root := fixture(t)
	p := filepath.Join(root, "instance.yaml")
	if e := os.Rename(p, p+".real"); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(p+".real", p); e != nil {
		t.Skip("symlinks unavailable", e)
	}
	if _, e := Bind(root, "read-db", []string{"Inspect DB"}, ""); e == nil {
		t.Fatal("followed symlink")
	}
}
func TestCapabilityRejectsEscapingSymlink(t *testing.T) {
	root := fixture(t)
	p := filepath.Join(root, "60-Operacion/Data/Inspect DB.md")
	os.Remove(p)
	outside := t.TempDir()
	write(t, outside, "note.md", "---\ntipo: operacional\n---\n")
	if e := os.Symlink(filepath.Join(outside, "note.md"), p); e != nil {
		t.Skip(e)
	}
	m, _ := LoadInstance(root)
	if _, e := ResolveCapability(root, m, "read-db"); e == nil {
		t.Fatal("followed escaping link")
	}
}
func TestInvalidYAMLAndContracts(t *testing.T) {
	for _, text := range []string{"cell: x\ncell: y\n", instanceFixture + "---\nfoo: bar\n", strings.Replace(instanceFixture, "documented-source", "invented", 1), strings.Replace(instanceFixture, "read-db: [Inspect DB]", "bad id: [Inspect DB]", 1)} {
		root := t.TempDir()
		write(t, root, "instance.yaml", text)
		if _, e := LoadInstance(root); e == nil {
			t.Fatal("accepted invalid instance")
		}
	}
}
func TestRemoteIdentity(t *testing.T) {
	want := "example.org/team/repo"
	for _, s := range []string{"git@example.org:Team/Repo.git", "https://EXAMPLE.org/Team/Repo.git/", "ssh://git@example.org:22/Team/Repo"} {
		v, e := RemoteIdentity(s)
		if e != nil || v != want {
			t.Errorf("%s = %q %v", s, v, e)
		}
	}
	for _, s := range []string{"https://secret@example.org/repo", "https://example.org/a?token=secret", "file:///repo", "git@example.org:../repo"} {
		if _, e := RemoteIdentity(s); e == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
func git(t *testing.T, root string, args ...string) {
	t.Helper()
	if _, e := exec.LookPath("git"); e != nil {
		t.Skip("Git unavailable")
	}
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
}
func TestRepositoryIdentityDeduplicatesRoots(t *testing.T) {
	root := fixture(t)
	repos := t.TempDir()
	repo := filepath.Join(repos, "example")
	os.Mkdir(repo, 0700)
	git(t, repo, "init", "--quiet")
	git(t, repo, "remote", "add", "origin", "git@example.org:Team/Repo.git")
	if _, e := UpdateWorkspace(root, WorkspaceUpdate{Roots: []string{repos, repo}}); e != nil {
		t.Fatal(e)
	}
	r, e := LocateRepository(root, "https://example.org/team/repo")
	if e != nil || r["status"] != "ok" {
		t.Fatal(r, e)
	}
}
func TestCatalogDuplicateIdentity(t *testing.T) {
	root := fixture(t)
	entry := `  - id: %s
    repository: %s
    domain: d
    scope: s
    summary: summary
    tags: [one]
    relationship: related
    consult_when: [needed]
`
	write(t, root, "90-Meta/vault-catalog.yaml", "version: 1\nvaults:\n"+fmt.Sprintf(entry, "one", "https://example.org/team/repo")+fmt.Sprintf(entry, "two", "git@example.org:team/repo.git"))
	if _, e := Catalog(root); e == nil {
		t.Fatal("accepted duplicate remote")
	}
}
func TestRunJSONAndRequiredArgs(t *testing.T) {
	root := fixture(t)
	var out bytes.Buffer
	if e := Run([]string{"capability", "--vault", root, "--capability", "read-db"}, &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "60-Operacion/Data/Inspect DB.md") {
		t.Fatal(out.String())
	}
	if e := Run([]string{"status"}, &out); e == nil {
		t.Fatal("vault optional")
	}
	if e := Run([]string{"database-target", "--vault", root}, &out); e == nil {
		t.Fatal("target optional")
	}
}
func TestWorkspaceRejectsInvalidPortsWithoutChanging(t *testing.T) {
	root := fixture(t)
	if _, e := UpdateWorkspace(root, WorkspaceUpdate{Ports: map[string]string{"prod": "5432"}}); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(root, WorkspaceFile))
	if _, e := UpdateWorkspace(root, WorkspaceUpdate{Ports: map[string]string{"prod": "99999"}}); e == nil {
		t.Fatal("accepted invalid port")
	}
	after, _ := os.ReadFile(filepath.Join(root, WorkspaceFile))
	if !bytes.Equal(before, after) {
		t.Fatal("changed invalid config")
	}
}

func TestRepositoryIdentityIgnoresTransportRewrite(t *testing.T) {
	root := fixture(t)
	repos := t.TempDir()
	repo := filepath.Join(repos, "repo")
	os.Mkdir(repo, 0700)
	git(t, repo, "init", "--quiet")
	git(t, repo, "remote", "add", "origin", "https://example.org/team/repo.git")
	git(t, repo, "config", "url.file:///transport-mirror/.insteadOf", "https://example.org/")
	expanded, e := exec.Command("git", "-C", repo, "remote", "get-url", "origin").Output()
	if e != nil || !strings.HasPrefix(string(expanded), "file:///transport-mirror/") {
		t.Fatal("fixture did not rewrite transport", string(expanded), e)
	}
	if _, e = UpdateWorkspace(root, WorkspaceUpdate{Roots: []string{repos}}); e != nil {
		t.Fatal(e)
	}
	result, e := LocateRepository(root, "git@example.org:team/repo.git")
	if e != nil || result["status"] != "ok" {
		t.Fatal("stored identity lost", result, e)
	}
}
func TestConfigSizeBound(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "instance.yaml")
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(maxConfigBytes + 1); e != nil {
		t.Fatal(e)
	}
	f.Close()
	if _, _, _, e := readYAML(path); e == nil || !strings.Contains(e.Error(), "4 MiB") {
		t.Fatal("missing config size bound", e)
	}
}
func TestConfigLockSymlinkRejected(t *testing.T) {
	root := fixture(t)
	outside := filepath.Join(t.TempDir(), "target")
	os.WriteFile(outside, []byte("untouched"), 0600)
	if e := os.Symlink(outside, filepath.Join(root, "instance.yaml.lock")); e != nil {
		t.Skip(e)
	}
	if _, e := Bind(root, "read-db", []string{"Inspect DB"}, ""); e == nil {
		t.Fatal("followed lock symlink")
	}
	b, _ := os.ReadFile(outside)
	if string(b) != "untouched" {
		t.Fatal("changed outside file")
	}
}

func TestResolveExplicitInteriorPath(t *testing.T) {
	root := fixture(t)
	for _, p := range markers {
		if p != "instance.yaml" {
			write(t, root, p, "marker")
		}
	}
	t.Setenv("PATH", t.TempDir())
	r, e := ResolvePath(filepath.Join(root, "60-Operacion", "Data", "Inspect DB.md"))
	if e != nil {
		t.Fatal(e)
	}
	canonical, _ := CanonicalRoot(root)
	if r["vault_root"] != canonical || r["interaction_mode"] != "filesystem" || r["obsidian_available"] != false {
		t.Fatal(r)
	}
}
func TestObsidianRegistrationExactPath(t *testing.T) {
	root := t.TempDir()
	canonical, _ := CanonicalRoot(root)
	other := t.TempDir()
	if got := matchingObsidianVault(canonical, "Wrong\t"+other+"\nVault with spaces\t"+root+"\n"); got != "Vault with spaces" {
		t.Fatal(got)
	}
	if got := matchingObsidianVault(canonical, "One\t"+root+"\nTwo\t"+root+"\n"); got != "" {
		t.Fatal("ambiguous registration selected", got)
	}
}

func TestVaultOnlyResolutionSurvivesInvalidWorkspace(t *testing.T) {
	root := fixture(t)
	for _, p := range markers {
		if p != "instance.yaml" {
			write(t, root, p, "marker")
		}
	}
	write(t, root, WorkspaceFile, "version: malformed\n")
	t.Setenv("PATH", t.TempDir())
	r, e := ResolvePath(root)
	if e != nil {
		t.Fatal("vault-only blocked", e)
	}
	if r["status"] != "resolved" || obj(r["source_context"])["status"] != "unavailable" {
		t.Fatal(r)
	}
	if _, e := Workspace(root); e == nil {
		t.Fatal("dependent workspace operation accepted invalid config")
	}
}

func TestReferenceBranchOrder(t *testing.T) {
	base := func(order any) Object {
		return Object{"version": 1, "cell": map[string]any{"name": "C", "purpose": "p"}, "systems": []any{map[string]any{"id": "s", "name": "S"}},
			"sources": map[string]any{"reference_branch_order": order}}
	}
	if got := ReferenceBranchOrder(Object{}); len(got) != 2 || got[0] != "main" || got[1] != "master" {
		t.Fatalf("default order is main, master: %v", got)
	}
	if got := ReferenceBranchOrder(base([]any{"develop", "main"})); got[0] != "develop" {
		t.Fatalf("the cell's order is kept: %v", got)
	}
	for _, bad := range []any{[]any{}, []any{"main", "main"}, []any{"refs/heads/main"}, "main"} {
		if ValidateInstance(base(bad)) == nil {
			t.Fatalf("invalid order accepted: %v", bad)
		}
	}
	if e := ValidateInstance(base([]any{"develop", "main", "master"})); e != nil {
		t.Fatal(e)
	}
}
