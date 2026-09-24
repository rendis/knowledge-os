package check

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, path, body string) {
	t.Helper()
	p := filepath.Join(root, path)
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestLinksScopeAndCanonical(t *testing.T) {
	r := t.TempDir()
	put(t, r, "00-Home.md", "[[Target]] [[Alias]] [[Missing]] [[Missing]]\n[bad](.agents/secret.md)\n`[[Code]]`\n```\n[[Fenced]]\n```\n[[Overview.base]]")
	put(t, r, "Target.md", "---\naliases: [Alias]\n---\n")
	put(t, r, "Overview.base", "views: []")
	for _, p := range []string{".private/p.md", "investigations/p.md", "AGENTS.md", "CLAUDE.md", "AGENTS.personal.md"} {
		put(t, r, p, "[[ShouldIgnore]]")
	}
	got, e := Links(r)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Broken) != 1 || !strings.Contains(got.Broken[0], "Missing") {
		t.Fatalf("broken=%v", got.Broken)
	}
	if len(got.AliasTargets) != 1 || len(got.Hidden) != 1 || len(got.Orphans) != 0 || len(got.ExpectedOrphans) != 1 {
		t.Fatalf("%+v", got)
	}
}
func TestLinksDuplicatesAndOrphans(t *testing.T) {
	r := t.TempDir()
	put(t, r, "a/Note.md", "")
	put(t, r, "b/Note.md", "")
	got, e := Links(r)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Duplicates) != 1 || len(got.Orphans) != 1 {
		t.Fatalf("%+v", got)
	}
	var out bytes.Buffer
	if !errors.Is(Run([]string{"links", "--vault", r}, &out), ErrIssues) {
		t.Fatal("must fail issues")
	}
	if !strings.Contains(out.String(), `"duplicates"`) {
		t.Fatal(out.String())
	}
}
func TestBasesReferencesAndScopes(t *testing.T) {
	r := t.TempDir()
	put(t, r, "A.md", "---\ncustom: yes\n---\n")
	put(t, r, "plan/ignored.md", "---\nexcluded: true\n---\n")
	put(t, r, "all.base", `formulas:
  display: 'file.name'
properties:
  custom:
    displayName: Custom
views:
  - type: table
    name: All
    filters:
      and:
        - 'tipo == "system"'
    order: [file.name, custom, formula.display]
    groupBy:
      property: tipo
      direction: ASC
    limit: 10
`)
	got, e := Bases(r)
	if e != nil || len(got.Issues) > 0 {
		t.Fatalf("%+v %v", got, e)
	}
	put(t, r, "bad.base", "views:\n - type: table\n   name: All\n   order: [excluded, file.unknown, formula.missing, tipo, tipo]\n   limit: true\n")
	got, e = Bases(r)
	if e != nil || len(got.Issues) != 5 {
		t.Fatalf("%+v %v", got, e)
	}
}
func TestBasesInvalidYAMLAndAliases(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{{"duplicate", "views: []\nviews: []", false}, {"cycle", "filters: &self\n  and: [*self]\nviews: []", false}, {"alias", "filters:\n  and:\n   - &filter 'tipo == 1'\n   - *filter\nviews: [{type: table, name: All}]", true}, {"null", "null", false}, {"multi", "views: [{type: table, name: All}]\n---\nviews: []", false}} {
		t.Run(tc.name, func(t *testing.T) {
			r := t.TempDir()
			put(t, r, "all.base", tc.body)
			got, e := Bases(r)
			if e != nil {
				t.Fatal(e)
			}
			if (len(got.Issues) == 0) != tc.valid {
				t.Fatalf("%+v", got)
			}
		})
	}
}
func TestNoBasesAndBadCommand(t *testing.T) {
	r := t.TempDir()
	var out bytes.Buffer
	if !errors.Is(Run([]string{"--vault", r, "bases"}, &out), ErrIssues) {
		t.Fatal("missing Bases must fail")
	}
	if Run([]string{"--vault"}, &out) == nil {
		t.Fatal("missing path accepted")
	}
}
