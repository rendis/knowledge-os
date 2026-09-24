package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassification(t *testing.T) {
	s := scope{root: t.TempDir(), prefixes: []string{"team-"}, approved: map[string]bool{}}
	notes := []object{{"note": "reader", "repo": "team-reader", "recorded_sha": "123456789abc", "recorded_branch": "main", "valid": true}}
	repos := []object{{"name": "team-reader", "branch": "main", "sha": "123456789abc"}, {"name": "team-new", "branch": "main", "sha": "fedcba987654"}}
	result, e := classify(s, notes, nil, repos)
	if e != nil {
		t.Fatal(e)
	}
	if len(result) != 2 || result[0]["status"] != "new" || result[1]["status"] != "current" {
		t.Fatal(result)
	}
	repos[0]["sha"] = "000000000000"
	result, e = classify(s, notes, nil, repos)
	if e != nil || result[1]["status"] != "changed" {
		t.Fatal(result, e)
	}
	acks := []object{{"repository": "team-reader", "decision": "no-documentation-change", "analyzed_sha": "000000000000", "branch": "main"}}
	result, e = classify(s, notes, acks, repos)
	if e != nil || result[1]["status"] != "acknowledged-no-change" {
		t.Fatal(result, e)
	}
	acks[0]["decision"] = "no-durable-node"
	if _, e = classify(s, notes, acks, repos); e == nil {
		t.Fatal("incompatible acknowledgement accepted")
	}
	acks[0]["decision"] = "no-documentation-change"
	if _, e = classify(s, nil, acks, repos); e == nil {
		t.Fatal("missing no-change note accepted")
	}
}
func TestAcknowledgementValidation(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "90-Meta"), 0700)
	s := scope{prefixes: []string{"team-"}, approved: map[string]bool{}}
	good := `{"version":1,"repositories":[{"repository":"team-reader","branch":"main","analyzed_sha":"123456789abc","decision":"no-documentation-change","analysis_date":"2026-09-24"}]}`
	path := filepath.Join(root, "90-Meta/.sync-acknowledgements.json")
	os.WriteFile(path, []byte(good), 0600)
	if _, e := acknowledgements(root, s); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{`{"version":1,"version":1,"repositories":[]}`, `{"version":1,"repositories":[]} {}`, `{"version":1,"repositories":[{"repository":"other-reader"}]}`} {
		os.WriteFile(path, []byte(bad), 0600)
		if _, e := acknowledgements(root, s); e == nil {
			t.Fatal("accepted", bad)
		}
	}
}
func TestScopeAndNotes(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "90-Meta"), 0700)
	os.MkdirAll(filepath.Join(root, "20-Repos/System"), 0700)
	os.WriteFile(filepath.Join(root, "90-Meta/Alcance.md"), []byte("## Repositorios fuente\n| `other-reader` | yes |\n## Else\n| `excluded` | no |"), 0600)
	inst := object{"sources": object{"repo_prefixes": []any{"team"}}, "vault": object{"remote": "https://github.com/example/team-vault"}}
	s, e := newScope(root, inst)
	if e != nil {
		t.Fatal(e)
	}
	if !s.tracked("other-reader") || s.tracked("excluded") || !s.tracked("team-new") {
		t.Fatal(s)
	}
	os.WriteFile(filepath.Join(root, "20-Repos/System/reader.md"), []byte("---\naliases: [other-reader]\ncommit-analizado: 123456789abc\nrama-analizada: main\n---\n"), 0600)
	notes, e := loadNotes(root, s)
	if e != nil || len(notes) != 1 || notes[0]["valid"] != true {
		t.Fatal(notes, e)
	}
}
func TestTokenEnvironmentNeverLeaksAmbientWhenStored(t *testing.T) {
	t.Setenv("GH_TOKEN", "ambient")
	t.Setenv("GITHUB_TOKEN", "ambient2")
	env := envToken("")
	for _, e := range env {
		if e == "GH_TOKEN=ambient" || e == "GITHUB_TOKEN=ambient2" {
			t.Fatal("ambient token retained")
		}
	}
	env = envToken("selected")
	found := 0
	for _, e := range env {
		if e == "GH_TOKEN=selected" {
			found++
		}
	}
	if found != 1 {
		t.Fatal(env)
	}
}
