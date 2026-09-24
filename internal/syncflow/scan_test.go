package syncflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanTrackedWorkingTreeAndBoundedProvenance(t *testing.T) {
	vault := t.TempDir()
	repo := sourceRepo(t)
	_, e := sourceGit(repo, nil, "remote", "add", "origin", "https://example.invalid/team/APP00001-example.git")
	if e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(repo, "app.txt"), "example one\nexample two\n")
	write(t, filepath.Join(repo, "settings.yaml"), "PASSWORD: actual-value\n")
	sourceCommit(t, repo)
	write(t, filepath.Join(repo, "app.txt"), "example dirty\nexample two\n")
	write(t, filepath.Join(repo, "untracked.txt"), "example untracked\n")
	write(t, filepath.Join(vault, "instance.yaml"), "version: 1\ncell: {name: Example, purpose: Test vault}\nsystems: [{id: demo, name: Demo, aliases: []}]\nevidence: {profile: documented-source}\nlocale: {notes: es}\nsources: {repo_prefixes: [APP00001]}\n")
	write(t, filepath.Join(vault, "20-Repos/demo/example.md"), "---\ntipo: repositorio\naliases: [APP00001-example]\ngithub: https://example.invalid/team/APP00001-example.git\n---\n# Example\n")
	rootJSON, _ := json.Marshal(filepath.Dir(repo))
	write(t, filepath.Join(vault, ".knowledge-os-config.yaml"), "version: 1\nworkspace:\n  repository_roots: ["+string(rootJSON)+"]\n")
	result, e := scanRepository(vault, "example", repo, []string{"PASSWORD"}, 1)
	if e != nil {
		t.Fatal(e)
	}
	if result["dirty"] != true || result["hits_truncated"] != true || len(arr(result["hits"])) != 1 {
		t.Fatal(result)
	}
	for _, v := range arr(result["inventory"]) {
		if strings.Contains(str(obj(v)["path"]), "untracked") {
			t.Fatal("scanned untracked file")
		}
	}
	for _, v := range arr(result["hits"]) {
		if obj(v)["path"] == "settings.yaml" {
			t.Fatal("returned credential line")
		}
	}
	outside := sourceRepo(t)
	_, e = sourceGit(outside, nil, "remote", "add", "origin", "https://example.invalid/team/APP00001-example.git")
	if e != nil {
		t.Fatal(e)
	}
	sourceCommit(t, outside)
	context := map[string]any{"roots": []any{map[string]any{"path": repo}}, "clone_authorized": false}
	if scanAllowed(outside, context) {
		t.Fatal("authorized unrelated source")
	}
	write(t, filepath.Join(vault, ".knowledge-os-config.yaml"), "version: 1\nworkspace:\n  repository_roots: []\n")
	if _, e = scanRepository(vault, "example", repo, nil, 12); e == nil {
		t.Fatal("accepted unconfigured sources")
	}
	if e = os.Symlink(repo, filepath.Join(vault, "source-link")); e == nil {
		if _, e = inspectSource(filepath.Join(vault, "source-link")); e == nil {
			t.Fatal("followed source symlink")
		}
	}
}
