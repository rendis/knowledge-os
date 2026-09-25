package cases

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(content), 0o644); e != nil {
		t.Fatal(e)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("git %v: %s", args, b)
	}
}

func vault(t *testing.T) string {
	t.Helper()
	v := t.TempDir()
	for _, m := range []string{"AGENTS.md", "00-Home.md", "90-Meta/Convenciones.md", "90-Meta/Auditoria - Framework.md"} {
		write(t, v, m, "x\n")
	}
	write(t, v, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nlocale:\n  notes: es\nsystems:\n  - id: \"s\"\n    name: \"S\"\n")
	write(t, v, "20-Repos/orders.md", "---\ntipo: api\n---\n# orders\n\nEl servicio orders publica órdenes confirmadas en el topic de salida después de validar el cliente, calcular impuestos por país y registrar la auditoría completa de cada transacción en la base de datos operacional del sistema de ventas.\n")
	git(t, v, "init", "-q", "-b", "main")
	git(t, v, "add", "-A")
	git(t, v, "commit", "-q", "-m", "base")
	return v
}

func run(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	var b bytes.Buffer
	e := Run(args, &b)
	m := map[string]any{}
	_ = json.Unmarshal(b.Bytes(), &m)
	return m, e
}

func TestNewCaseFromTemplatePassesCheck(t *testing.T) {
	v := vault(t)
	for _, kind := range []string{"understanding", "development"} {
		res, e := run(t, "new", "--vault", v, "--title", "Órdenes duplicadas en "+kind, "--type", kind)
		if e != nil {
			t.Fatal(e)
		}
		id := res["id"].(string)
		if !idPattern.MatchString(id) || !strings.Contains(id, "ordenes-duplicadas") {
			t.Fatalf("id %q", id)
		}
		r, e := Check(v, res["path"].(string))
		if e != nil || !r.OK {
			t.Fatalf("an untouched %s template must pass: %+v %v", kind, r, e)
		}
		b, _ := os.ReadFile(filepath.Join(v, res["path"].(string)))
		if kind == "development" && !strings.Contains(string(b), "## Cambios por componente") {
			t.Fatal("development cases carry requirements, changes and acceptance criteria")
		}
	}
	res, _ := run(t, "new", "--vault", v, "--title", "Órdenes duplicadas otra vez", "--type", "understanding")
	if len(res["similar_cases"].([]any)) == 0 {
		t.Fatal("a similar title must be reported to avoid duplicate cases")
	}
}

func caseText(extra string) string {
	return "---\nid: 20260925-100000-ordenes\ntitle: \"Órdenes\"\ntype: understanding\nstatus: open\ncreated: 2026-09-25\n---\n\n# Órdenes\n\n## Objetivo y alcance\n\nEntender por qué se duplican.\n\n## Estado actual\n\nSe revisó [[orders]]; ver E-001.\n\n## Evidencia\n\n- **E-001** — El reintento reenvía sin clave de idempotencia. Fuente: `src/retry.go@abc1234` L10-L20.\n" + extra + "\n## Conclusiones\n\n- **F-001** — Demostrada por E-001.\n\n## Preguntas abiertas\n\n- **Q-001** — ¿Ocurre en PE? Se resuelve con el snapshot de PE.\n"
}

func issues(r Result, sev string) string {
	out := []string{}
	for _, i := range r.Issues {
		if i.Severity == sev {
			out = append(out, i.Where+": "+i.Detail)
		}
	}
	return strings.Join(out, "\n")
}

func TestCheckGatesEvidenceReferencesLinksAndLeaks(t *testing.T) {
	v := vault(t)
	p := ".investigations/20260925-100000-ordenes/investigation.md"
	write(t, v, p, caseText(""))
	if r, _ := Check(v, p); !r.OK {
		t.Fatalf("a sourced case must pass: %s", issues(r, "error"))
	}
	bad := caseText("- **E-002** — Además falla en CO según D-009.\n- **E-001** — repetido. Fuente: [[orders]].\nVer [[missing-note]]. token: abc123 en /Users/someone/tmp/x.log\n")
	write(t, v, p, bad)
	r, _ := Check(v, p)
	errs := issues(r, "error")
	for _, want := range []string{"E-002: evidence without a source", "D-009: referenced but not defined", "E-001: defined more than once", "[[missing-note]]: link does not resolve", "credential", "local path"} {
		if !strings.Contains(errs, want) {
			t.Fatalf("missing %q in:\n%s", want, errs)
		}
	}
	copied := caseText("\nEl servicio orders publica órdenes confirmadas en el topic de salida después de validar el cliente, calcular impuestos por país y registrar la auditoría completa de cada transacción en la base de datos operacional del sistema de ventas.\n")
	write(t, v, p, copied)
	if r, _ := Check(v, p); !strings.Contains(issues(r, "error"), "repeats") {
		t.Fatalf("a paragraph copied from a note must be replaced by a reference: %+v", r.Issues)
	}
	closed := strings.Replace(caseText(""), "status: open", "status: closed", 1)
	write(t, v, p, closed)
	if r, _ := Check(v, p); !strings.Contains(issues(r, "error"), "outcome") {
		t.Fatal("a closed case needs an outcome")
	}
	write(t, v, p, strings.Replace(closed, "status: closed", "status: closed\noutcome: completed", 1))
	if r, _ := Check(v, p); !r.OK {
		t.Fatalf("closed with outcome and evidence: %s", issues(r, "error"))
	}
}

func TestLegacyCasesLoadWithWarnings(t *testing.T) {
	v := vault(t)
	legacy := "---\nid: 20260701-120000-legacy-case\ntitle: Legacy\ndedupe-key: legacy\nstatus: investigating\npurpose: development\ncreated-at: 2026-07-01T12:00:00Z\n---\n\n# Legacy\n\n## Evidence\n\n### Facts\n\n- E-001 hecho sin fuente\n\n## History\n\n- 2026-07-01 — creado; registrado por alguien\n"
	write(t, v, "investigations/20260701-120000-legacy-case/investigation.md", legacy)
	cs, _ := List(v)
	if len(cs) != 1 || !cs[0].Legacy || cs[0].Status != "open" || cs[0].Type != "development" || cs[0].Visibility != "published" {
		t.Fatalf("legacy case must map onto the new states: %+v", cs)
	}
	r, _ := Check(v, cs[0].Path)
	if !r.OK || !strings.Contains(issues(r, "warning"), "evidence without a source") {
		t.Fatalf("legacy schema findings are warnings, not blockers: %+v", r)
	}
	ok, _, pre, _ := CheckIntroduced(v, cs[0].Path, []byte(legacy))
	if !ok || pre != 0 {
		t.Fatal("an unchanged legacy case passes")
	}
}

func TestListFindsRetiredCasesInHistory(t *testing.T) {
	v := vault(t)
	write(t, v, "investigations/20260801-090000-old/investigation.md", caseText(""))
	git(t, v, "add", "-A")
	git(t, v, "commit", "-q", "-m", "docs: publish case")
	git(t, v, "rm", "-q", "-r", "investigations/20260801-090000-old")
	git(t, v, "commit", "-q", "-m", "docs: retire case\n\nRetired-Case: 20260801-090000-old")
	cs, _ := List(v)
	if len(cs) != 1 || cs[0].Status != "retired" || cs[0].ID != "20260801-090000-old" {
		t.Fatalf("retired cases stay resolvable by id: %+v", cs)
	}
}

func TestIntroducedErrorsOnlyBlockNewDefects(t *testing.T) {
	v := vault(t)
	p := "investigations/20260925-100000-ordenes/investigation.md"
	base := caseText("- **E-002** — sin fuente.\n")
	write(t, v, p, base+"\nNueva línea.\n")
	if ok, _, pre, _ := CheckIntroduced(v, p, []byte(base)); !ok || pre == 0 {
		t.Fatal("a pre-existing error does not block an unrelated edit")
	}
	write(t, v, p, base+"\n- **E-003** — tampoco tiene fuente.\n")
	if ok, introduced, _, _ := CheckIntroduced(v, p, []byte(base)); ok || len(introduced) != 1 {
		t.Fatalf("a new unsourced record blocks: %v", introduced)
	}
}
