package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExistingVaults(t *testing.T) {
	for _, root := range filepath.SplitList(os.Getenv("AUDIT_TEST_VAULTS")) {
		if root == "" {
			continue
		}
		counts, issues, e := Audit(root)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("%s counts=%v issues=%v", root, counts, issues)
		if len(issues) > 0 {
			t.Fail()
		}
	}
}
func write(t *testing.T, root, path, body string) {
	t.Helper()
	p := filepath.Join(root, path)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestAuditTopologyAndScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "instance.yaml", `version: 1
cell: {name: Example, purpose: Test vault}
systems:
 - id: demo
   name: Demo
   aliases: []
evidence: {profile: documented-source}
locale: {notes: es}
`)
	write(t, root, "60-Operacion/Operacion.md", "---\ntipo: indice\ntags: [moc]\n---\n")
	write(t, root, ".operations/relations.jsonl", "private")
	write(t, root, "investigations/relations.db", "published research")
	write(t, root, "90-Meta/Unrelated.md", "---\ninvalid: [\n---\n")
	_, issues, e := Audit(root)
	if e != nil || len(issues) > 0 {
		t.Fatalf("%v %v", e, issues)
	}
	write(t, root, "20-Repos/demo/Broken.md", "---\ntipo: api\n---\nla sync\n")
	write(t, root, "relations.db", "bad")
	_, issues, e = Audit(root)
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"external relation ledger", "missing frontmatter field aliases", "missing section Propósito", "synchronization process state"} {
		if !strings.Contains(strings.Join(issues, "\n"), want) {
			t.Errorf("missing %s", want)
		}
	}
}
func TestEveryFamilyRejectsMissingContract(t *testing.T) {
	for _, g := range groupOrder {
		t.Run(g, func(t *testing.T) {
			a := auditor{contracts: map[string][]string{}, areas: map[string]string{}}
			a.check(note{path: "bad.md", f: map[string]any{}}, g)
			if len(a.issues) == 0 {
				t.Fatal("missing structural violations")
			}
		})
	}
}
func TestLearningCycleAndReplacement(t *testing.T) {
	a := auditor{}
	notes := []note{{path: "70-Aprendizajes/A.md", f: map[string]any{"tipo": "aprendizaje", "estado": "superado", "supersede-a": []any{"[[B]]"}}}, {path: "70-Aprendizajes/B.md", f: map[string]any{"tipo": "aprendizaje", "estado": "superado", "supersede-a": []any{"[[A]]"}}}}
	a.learningRelations(notes)
	if !strings.Contains(strings.Join(a.issues, "\n"), "cycles") {
		t.Fatal(a.issues)
	}
	a.issues = nil
	notes[0].f["supersede-a"] = []any{}
	a.learningRelations(notes)
	if !strings.Contains(strings.Join(a.issues, "\n"), "by a replacement") {
		t.Fatal(a.issues)
	}
}
func TestLearningEvidenceChecks(t *testing.T) {
	a := auditor{}
	n := note{path: "70-Aprendizajes/Aprendizaje - X.md", body: "## Evidencia acumulada\n### EV-002\n- Investigación: `20260924-123456-demo`\n- Fuentes durables: .investigations/foo\n", f: map[string]any{"investigaciones-origen": []any{"20260924-123456-other"}}}
	a.learning(n)
	for _, want := range []string{"sequential", "appear in investigaciones-origen", "ignored workspaces", "referenced by at least one EV", "closed order"} {
		if !strings.Contains(strings.Join(a.issues, "\n"), want) {
			t.Error(want, a.issues)
		}
	}
}
func TestBranchContract(t *testing.T) {
	for _, v := range []string{"HEAD", "refs/heads/main", "-x", "x..y", "x.lock", "x/", "a b"} {
		if branch(v) {
			t.Errorf("accepted %q", v)
		}
	}
	if !branch("feature/test") {
		t.Fatal("valid branch rejected")
	}
}
func TestDatesAndListParsing(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.md", "---\nultima-auditoria: 2026-09-24\ntags: [moc, sistema/demo]\n---\n## Body\n")
	n, e := read(root, "a.md")
	if e != nil || !date(n.f["ultima-auditoria"]) || !contains(n.f["tags"], "moc") {
		t.Fatalf("%v %#v", e, n.f)
	}
	if date("2026-02-30") {
		t.Fatal("invalid date accepted")
	}
}
func TestOperationalClosedContract(t *testing.T) {
	a := auditor{areas: map[string]string{"Data": "data"}}
	n := note{path: "60-Operacion/Data/Query.md", f: map[string]any{"tipo": "operacional", "clase": "procedimiento", "estado": "vigente", "owner": "por-definir", "ultima-verificacion": "2026-02-30", "area": "[[Wrong]]", "canales": []any{}, "tags": []any{"operacion", "operacion/procedimiento", "rogue"}, "relacionado-con": []any{"[[Unknown]]"}, "report-id": "wrong"}}
	a.operational(n)
	for _, want := range []string{"real owner", "valid YYYY-MM-DD", "operacion/area/data", "unsupported operational tags", "area must be [[Data]]", "target operational area MOCs", "only allowed for clase reporte", "closed table header"} {
		if !strings.Contains(strings.Join(a.issues, "\n"), want) {
			t.Errorf("missing %s", want)
		}
	}
}
func TestArchitectureSubtypes(t *testing.T) {
	for _, kind := range []string{"servicio", "componente", "recurso-runtime"} {
		a := auditor{}
		n := note{path: "15-Arquitectura/X.md", f: map[string]any{"tipo": kind, "compuesto-por": []any{"[[One]]"}, "implementado-por": []any{"[[One]]", "[[Two]]"}, "plataforma": "x"}}
		a.check(n, "ARCHITECTURE_NOTES")
		issues := strings.Join(a.issues, "\n")
		if !strings.Contains(issues, "is not allowed for tipo") {
			t.Error(kind, issues)
		}
		if kind == "servicio" && !strings.Contains(issues, "at least two") {
			t.Error(issues)
		}
		if kind == "componente" && !strings.Contains(issues, "exactly one") {
			t.Error(issues)
		}
		if kind == "recurso-runtime" && !strings.Contains(issues, "missing runtime field") {
			t.Error(issues)
		}
	}
}
func TestFrontmatterEmptyListCompatibility(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.md", "---\naliases:\ntags: []\nowner: null\n---\n")
	n, e := read(root, "a.md")
	if e != nil {
		t.Fatal(e)
	}
	if x, ok := arr(n.f["aliases"]); !ok || len(x) != 0 {
		t.Fatalf("empty aliases changed semantics: %#v", n.f)
	}
	if n.f["owner"] != nil {
		t.Fatal("explicit null changed")
	}
}
