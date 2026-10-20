package inventory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestHistoricalAttemptsRemainUnresolvedWithoutBlockingInventory(t *testing.T) {
	for _, decision := range []string{"review-rejected", "inspection-limited"} {
		for _, tc := range []struct {
			name, noteSHA, ackBranch, remoteSHA, wantStatus, wantRecorded string
		}{
			{"same-note-and-attempt", "123456789abc", "main", "123456789abc", decisions[decision], "123456789abc"},
			{"older-note", "000000000000", "main", "123456789abc", decisions[decision], "000000000000"},
			{"newer-note", "fedcba987654", "main", "fedcba987654", "current", "fedcba987654"},
			{"remote-moved", "123456789abc", "main", "fedcba987654", "changed", "123456789abc"},
			{"other-branch", "123456789abc", "develop", "123456789abc", "current", "123456789abc"},
			{"no-note", "", "main", "123456789abc", decisions[decision], ""},
			{"no-note-remote-moved", "", "main", "fedcba987654", "new", ""},
			{"repository-missing", "", "main", "", "missing-candidate", ""},
		} {
			t.Run(decision+"/"+tc.name, func(t *testing.T) {
				s := scope{root: t.TempDir(), prefixes: []string{"team-"}}
				notes := []object{{"note": "other", "repo": "team-other", "recorded_sha": "123456789abc", "recorded_branch": "main", "valid": true}}
				if tc.noteSHA != "" {
					notes = append(notes, object{"note": "reader", "repo": "team-reader", "recorded_sha": tc.noteSHA, "recorded_branch": "main", "valid": true})
				}
				acks := []object{{"repository": "team-reader", "decision": decision, "analyzed_sha": "123456789abc", "branch": tc.ackBranch}}
				repos := []object{{"name": "team-other", "branch": "main", "sha": "123456789abc"}}
				if tc.remoteSHA != "" {
					repos = append(repos, object{"name": "team-reader", "branch": "main", "sha": tc.remoteSHA})
				}
				result, err := classify(s, notes, acks, repos)
				if err != nil {
					t.Fatal(err)
				}
				if len(result) != 2 || result[0]["status"] != "current" || result[1]["status"] != tc.wantStatus || str(result[1]["recorded_sha"]) != tc.wantRecorded {
					t.Fatalf("attempt must not block unrelated repositories or advance the note baseline: %v", result)
				}
				if tc.wantRecorded == "" && result[1]["recorded_sha"] != nil {
					t.Fatalf("unsuccessful attempt became a successful cursor: %v", result[1])
				}
				if tc.wantStatus == decisions[decision] && !strings.Contains(str(result[1]["details"]), "unresolved") {
					t.Fatalf("historical attempt needs an explicit unresolved diagnostic: %v", result[1])
				}
			})
		}
	}
}

func TestInventoryReadsHistoricalAttemptWithMatchingNote(t *testing.T) {
	fakeGHPath(t)
	t.Setenv("GH_TOKEN", "ambient-token")
	root := t.TempDir()
	for _, dir := range []string{"20-Repos/System", "90-Meta"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"instance.yaml":             "version: 1\ncell:\n  name: Example\n  purpose: Test historical sync records\nsystems:\n  - id: example\n    name: Example\nsources:\n  github_org: example\n  repo_prefixes: [team]\n",
		"20-Repos/System/reader.md": "---\naliases: [team-reader]\ncommit-analizado: 123456789abc\nrama-analizada: main\n---\n",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, decision := range []string{"review-rejected", "inspection-limited"} {
		t.Run(decision, func(t *testing.T) {
			ack := fmt.Sprintf(`{"version":1,"repositories":[{"repository":"team-reader","branch":"main","analyzed_sha":"123456789abc","decision":%q,"analysis_date":"2026-08-11"}]}`+"\n", decision)
			path := filepath.Join(root, "90-Meta/.sync-acknowledgements.json")
			if err := os.WriteFile(path, []byte(ack), 0600); err != nil {
				t.Fatal(err)
			}
			for _, format := range []string{"json", "markdown"} {
				var out bytes.Buffer
				if err := Run([]string{"--vault", root, "--format", format}, &out); err != nil {
					t.Fatalf("%s: %v", format, err)
				}
				if format == "json" {
					var result struct {
						Summary map[string]int `json:"summary"`
						Repos   []object       `json:"repos"`
					}
					if err := json.Unmarshal(out.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if result.Summary[decisions[decision]] != 1 || result.Summary["current"] != 0 || len(result.Repos) != 1 || result.Repos[0]["recorded_sha"] != "123456789abc" {
						t.Fatalf("historical attempt must stay unresolved: %s", out.String())
					}
				} else if !strings.Contains(out.String(), "| team-reader | reader | "+decisions[decision]+" |") {
					t.Fatalf("missing historical outcome in markdown: %s", out.String())
				}
			}
			b, err := os.ReadFile(path)
			if err != nil || string(b) != ack {
				t.Fatalf("inventory rewrote historical state: %s, %v", b, err)
			}
		})
	}
}

func TestAcknowledgementConflictIdentifiesRepositoryAndFiles(t *testing.T) {
	s := scope{root: t.TempDir(), prefixes: []string{"team-"}}
	note := object{"repo": "team-reader", "path": "20-Repos/System/reader.md", "recorded_sha": "123456789abc", "recorded_branch": "main", "valid": true}
	for _, tc := range []struct {
		decision string
		notes    []object
	}{
		{"no-durable-node", []object{note}},
		{"no-documentation-change", nil},
	} {
		t.Run(tc.decision, func(t *testing.T) {
			_, err := classify(s, tc.notes, []object{{"repository": "team-reader", "decision": tc.decision}}, nil)
			if err == nil {
				t.Fatal("incompatible successful acknowledgement accepted")
			}
			for _, context := range []string{"team-reader", tc.decision, "90-Meta/.sync-acknowledgements.json"} {
				if !strings.Contains(err.Error(), context) {
					t.Fatalf("missing %q in diagnostic: %v", context, err)
				}
			}
			if len(tc.notes) != 0 && !strings.Contains(err.Error(), str(note["path"])) {
				t.Fatalf("missing note path: %v", err)
			}
		})
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
