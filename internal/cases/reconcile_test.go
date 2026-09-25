package cases

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"documentation-vault/internal/devhandoff"
)

func TestReconcileImportsHandoffProgressDeterministically(t *testing.T) {
	v := vault(t)
	dir := t.TempDir()
	repo, root := filepath.Join(dir, "repos", "svc-orders"), filepath.Join(dir, "worktrees")
	write(t, repo, "src/order.go", "package src\n")
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	git(t, repo, "remote", "add", "origin", "https://github.com/acme/svc-orders.git")
	os.MkdirAll(root, 0o755)
	write(t, v, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+filepath.Join(dir, "repos")+"\"\nskills:\n  manage-development-handoff:\n    worktree_root: \""+root+"\"\n")

	res, _ := run(t, "new", "--vault", v, "--title", "Órdenes idempotentes", "--type", "development", "--objective", "Evitar publicaciones duplicadas.", "--date", "2026-09-25")
	id := res["id"].(string)
	caseDir := filepath.Dir(filepath.Join(v, res["path"].(string)))
	if _, e := run(t, "add", "--vault", v, "--id", id, "--kind", "requirement", "--text", "Un reintento no publica dos veces", "--origin", "solicitante 2026-09-25"); e != nil {
		t.Fatal(e)
	}
	pkg := "---\nhandoff: DH-001\ncase: " + id + "\nrepository: https://github.com/acme/svc-orders.git\nbase: main\nbranch: issue/idempotent\n---\n\n# Clave de idempotencia\n\n## Tarea\n\nCumplir R-001.\n\n## Cambios\n\n- `src/order.go`: clave por ID.\n\n## Criterios de aceptación\n\n- Test unitario (R-001).\n"
	write(t, caseDir, "handoffs/DH-001.md", pkg)
	if _, e := run(t, "add", "--vault", v, "--id", id, "--kind", "handoff", "--package", "handoffs/DH-001.md"); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e := devhandoff.Run([]string{"start", "--vault", v, "--package", filepath.Join(caseDir, "handoffs", "DH-001.md"), "--apply"}, &b); e != nil {
		t.Fatal(e, b.String())
	}
	wt := filepath.Join(root, "svc-orders", "idempotent")
	write(t, wt, "src/order.go", "package src\n// idempotent\n")
	git(t, wt, "commit", "-qam", "feat: idempotent retry\n\nHandoff: DH-001")
	deltas := "# Deltas\n\n## DELTA-001 — El consumidor exige orderId\n- Handoff: DH-001\n- Type: finding\n- Detail: el consumidor descarta mensajes sin orderId.\n- Evidence: `consumer/main.go@abc1234` L10\n" +
		"\n## DELTA-002 — La clave viene en un header\n- Handoff: DH-001\n- Type: definition\n- Detail: la clave es el header x-order-id, no el cuerpo.\n- Evidence: commit abc1234\n" +
		"\n## DELTA-003 — Timeout del broker\n- Handoff: DH-001\n- Type: deviation\n- Detail: se subió el timeout.\n- Evidence: conversación\n"
	write(t, wt, ".handoff/deltas.md", deltas)

	preview, e := run(t, "reconcile", "--vault", v, "--worktree", wt)
	if e != nil || preview["applied"] != false || len(preview["records"].([]any)) != 4 {
		t.Fatalf("preview: commits, two sourced deltas and one question: %v %v", e, preview)
	}
	before, _ := os.ReadFile(filepath.Join(v, res["path"].(string)))
	if strings.Contains(string(before), "DELTA-001") {
		t.Fatal("a preview writes nothing")
	}
	if _, e := run(t, "reconcile", "--vault", v, "--worktree", wt, "--apply"); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(v, res["path"].(string)))
	text := string(after)
	for _, want := range []string{"feat: idempotent retry", "Fuente: `consumer/main.go@abc1234` L10 (DELTA-001, DH-001)", "La clave viene en un header", "- **Q-001** — DELTA-003, DH-001: Timeout del broker", "reconciliado hasta"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	if r, _ := Check(v, res["path"].(string)); !r.OK {
		t.Fatalf("a reconciled case passes its gate: %s", issues(r, "error"))
	}
	again, _ := run(t, "reconcile", "--vault", v, "--worktree", wt, "--apply")
	if len(again["records"].([]any)) != 0 {
		t.Fatalf("a second run over the same worktree imports nothing: %v", again)
	}
	write(t, wt, ".handoff/deltas.md", deltas+"\n## DELTA-004 — Criterios verificados\n- Handoff: DH-001\n- Type: verification\n- Detail: el test unitario pasa.\n- Evidence: `src/order_test.go@def5678`\n")
	last, _ := run(t, "reconcile", "--vault", v, "--worktree", wt, "--apply")
	if len(last["records"].([]any)) != 1 {
		t.Fatalf("only the new delta comes in: %v", last)
	}
}
