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
		res, e := run(t, "new", "--vault", v, "--title", "Órdenes duplicadas en "+kind, "--type", kind, "--objective", "Entender por qué se duplican órdenes confirmadas.")
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
		if kind == "development" && (!strings.Contains(string(b), "## Requisitos") || strings.Contains(string(b), "## Cambios por componente")) {
			t.Fatal("development cases carry requirements; changes and criteria live in the handoff packages")
		}
	}
	res, _ := run(t, "new", "--vault", v, "--title", "Órdenes duplicadas otra vez", "--type", "understanding", "--objective", "x")
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

func TestCheckGatesHandoffPackages(t *testing.T) {
	v := vault(t)
	p := ".investigations/20260925-100000-ordenes/investigation.md"
	write(t, v, p, caseText(""))
	write(t, v, ".investigations/20260925-100000-ordenes/handoffs/DH-001.md", "---\nhandoff: DH-001\ncase: 20260925-100000-ordenes\nrepository: https://github.com/acme/orders.git\nbase: main\nbranch: issue/x\n---\n\n# Tarea\n\n## Tarea\n\nHacer algo.\n\n## Cambios\n\n## Criterios de aceptación\n\n- Test.\n")
	r, _ := Check(v, p)
	if r.OK || !strings.Contains(issues(r, "error"), "handoffs/DH-001.md: section Cambios") {
		t.Fatalf("an incomplete task package blocks the case: %+v", r.Issues)
	}
	write(t, v, ".investigations/20260925-100000-ordenes/handoffs/DH-001.md", "---\nhandoff: DH-001\ncase: 20260925-100000-ordenes\nrepository: https://github.com/acme/orders.git\nbase: main\nbranch: issue/x\ndepends-on: DH-007\n---\n\n# Tarea\n\n## Tarea\n\nHacer algo.\n\n## Cambios\n\n- `a.go`.\n\n## Criterios de aceptación\n\n- Test.\n")
	if r, _ := Check(v, p); !strings.Contains(issues(r, "error"), "depends-on DH-007, which is not a package of this case") {
		t.Fatalf("a dependency must be a package of the case: %+v", r.Issues)
	}
}

func TestCaseChangesGoThroughTheCLI(t *testing.T) {
	v := vault(t)
	res, e := run(t, "new", "--vault", v, "--title", "Órdenes duplicadas", "--type", "development", "--objective", "Evitar que un reintento publique dos veces la misma orden.", "--date", "2026-09-25")
	if e != nil {
		t.Fatal(e)
	}
	id, path := res["id"].(string), filepath.Join(v, res["path"].(string))
	must := func(args ...string) map[string]any {
		t.Helper()
		m, e := run(t, append([]string{args[0], "--vault", v, "--id", id, "--date", "2026-09-25"}, args[1:]...)...)
		if e != nil {
			t.Fatalf("%v: %v %v", args, e, m)
		}
		return m
	}
	refused := func(want string, args ...string) {
		t.Helper()
		m, e := run(t, append([]string{args[0], "--vault", v, "--id", id}, args[1:]...)...)
		if e == nil || !strings.Contains(e.Error()+toJSON(m), want) {
			t.Fatalf("%v must be refused with %q: %v %v", args, want, e, m)
		}
	}
	refused("--source must cite", "add", "--kind", "evidence", "--text", "El reintento no usa clave.", "--source", "lo vi", "--level", "demonstrated")
	refused("--limits is required", "add", "--kind", "evidence", "--text", "x", "--source", "`src/retry.go@abc1234`", "--level", "observed")
	if m := must("add", "--kind", "question", "--text", "¿Pasa también en PE?", "--resolve-by", "snapshot de PE"); m["record"] != "Q-001" {
		t.Fatalf("ids are assigned: %v", m)
	}
	log := filepath.Join(t.TempDir(), "retry.log")
	os.WriteFile(log, []byte("retry 1 order=42\napi_key=abcd1234\n"), 0o644)
	refused("credential", "add", "--kind", "evidence", "--text", "Log", "--source", "solicitante 2026-09-25", "--level", "demonstrated", "--file", log)
	os.WriteFile(log, []byte("retry 1 order=42\nretry 2 order=42\n"), 0o644)
	if m := must("add", "--kind", "evidence", "--text", "El reintento reenvía sin clave de idempotencia", "--source", "solicitante 2026-09-25, log de UAT", "--level", "observed", "--limits", "una orden, UAT", "--file", log, "--resolves", "Q-001"); m["record"] != "E-001" {
		t.Fatalf("evidence %v", m)
	}
	refused("not defined", "add", "--kind", "finding", "--text", "Causa", "--level", "demonstrated", "--from", "E-009")
	must("add", "--kind", "finding", "--text", "La duplicación viene del reintento", "--level", "inferred", "--from", "E-001", "--for-vault", "[[orders]]")
	refused("does not resolve", "add", "--kind", "finding", "--text", "x", "--level", "inferred", "--from", "E-001", "--for-vault", "[[no-existe]]")
	must("add", "--kind", "requirement", "--text", "Un reintento no publica dos veces", "--origin", "solicitante 2026-09-25")
	must("add", "--kind", "evidence", "--text", "El reintento reenvía con backoff fijo", "--source", "`src/retry.go@abc1234` L30", "--level", "demonstrated", "--supersedes", "E-001")
	refused("must cite the records", "state", "--text", "Todo claro.")
	must("state", "--text", "Causa inferida (F-001); falta implementar R-001.")
	refused("published", "absorb", "--finding", "F-001")
	must("close", "--outcome", "completed", "--reason", "Causa establecida y requisito definido")
	refused("already closed", "close", "--outcome", "completed", "--reason", "x")
	must("reopen", "--reason", "Nuevo reporte en PE")
	b, _ := os.ReadFile(path)
	text := string(b)
	for _, want := range []string{"Evitar que un reintento publique dos veces", "- **Q-001** — ¿Pasa también en PE? Se resuelve con: snapshot de PE. — resuelta por E-001",
		"— reemplazado por E-002", "Nivel: inferida desde E-001. Para el vault: [[orders]].", "Archivos: `artifacts/E-001-retry.log`.",
		"- 2026-09-25 — E-002 agregado; reemplaza a E-001", "cerrado (completed)", "reabierto: Nuevo reporte en PE", "status: open"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "outcome:") {
		t.Fatal("reopening clears the outcome")
	}
	if r, _ := Check(v, res["path"].(string)); !r.OK {
		t.Fatalf("a case written through the CLI passes its gate: %s", issues(r, "error"))
	}
	if _, e := os.Stat(filepath.Join(filepath.Dir(path), "artifacts", "E-001-retry.log")); e != nil {
		t.Fatal("the file is copied into the case, named after its record")
	}
}

func TestPublishedCasesChangeOnlyOnSyncBranches(t *testing.T) {
	v := vault(t)
	write(t, v, "investigations/20260925-100000-ordenes/investigation.md", caseText("")+"\n## Bitácora\n")
	if _, e := run(t, "add", "--vault", v, "--id", "20260925-100000-ordenes", "--kind", "question", "--text", "¿Y en CO?", "--resolve-by", "snapshot"); e == nil || !strings.Contains(e.Error(), "sync branch") {
		t.Fatalf("a published case must not change outside a sync branch: %v", e)
	}
	git(t, v, "checkout", "-q", "-b", "sync/case-ordenes")
	if _, e := run(t, "add", "--vault", v, "--id", "20260925-100000-ordenes", "--kind", "question", "--text", "¿Y en CO?", "--resolve-by", "snapshot"); e != nil {
		t.Fatal(e)
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestDocumentedComponentsAreReferenced(t *testing.T) {
	v := vault(t)
	write(t, v, "20-Repos/svc-orders-api.md", "---\ntipo: api\n---\n# svc-orders-api\n\nExpone órdenes.\n")
	res, _ := run(t, "new", "--vault", v, "--title", "Órdenes", "--type", "understanding", "--objective", "Entender la API de órdenes.")
	id := res["id"].(string)
	m, e := run(t, "add", "--vault", v, "--id", id, "--kind", "evidence", "--text", "APP01234-svc-orders-api responde 404 sin clave", "--source", "`src/api.go@abc1234` L5", "--level", "demonstrated")
	if e != nil || !strings.Contains(toJSON(m["gate_review"]), "[[svc-orders-api]]") {
		t.Fatalf("a write that names a documented component points to its note: %v %v", e, m)
	}
	m, _ = run(t, "add", "--vault", v, "--id", id, "--kind", "finding", "--text", "La API [[svc-orders-api]] exige la clave", "--level", "demonstrated", "--from", "E-001")
	if strings.Contains(toJSON(m["gate_review"]), "svc-orders-api") {
		t.Fatalf("once the case links the note the reminder goes: %v", m)
	}
}

func TestCredentialReferencesAreNotCredentials(t *testing.T) {
	for _, ref := range []string{"password: process.env.DB_PASSWORD", "token: ${GITHUB_TOKEN}", "api_key = os.getenv('KEY')", "password: <from vault>", "secret: $DB_SECRET"} {
		if len(credentials(ref, 1)) != 0 {
			t.Errorf("a reference to where a secret lives is not a credential: %s", ref)
		}
	}
	for _, value := range []string{"password: hunter2", "token: abc123", "api_key=sk-live-123"} {
		if len(credentials(value, 1)) == 0 {
			t.Errorf("a credential value must be caught: %s", value)
		}
	}
}
