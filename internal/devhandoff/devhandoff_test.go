package devhandoff

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

func tgit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s", args, b)
	}
	return strings.TrimSpace(string(b))
}

type fixture struct{ vault, repo, root string }

func setup(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	f := fixture{vault: filepath.Join(dir, "vault"), repo: filepath.Join(dir, "repos", "svc-orders"), root: filepath.Join(dir, "worktrees")}
	write(t, f.repo, "src/order.go", "package src\n")
	write(t, f.repo, "AGENTS.md", "# Repository rules\n\nRun make test.\n")
	tgit(t, f.repo, "init", "-q", "-b", "main")
	tgit(t, f.repo, "add", "-A")
	tgit(t, f.repo, "commit", "-q", "-m", "init")
	tgit(t, f.repo, "remote", "add", "origin", "https://github.com/acme/svc-orders.git")
	for _, m := range []string{"AGENTS.md", "00-Home.md", "90-Meta/Convenciones.md", "90-Meta/Auditoria - Framework.md"} {
		write(t, f.vault, m, "x\n")
	}
	write(t, f.vault, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\n")
	write(t, f.vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+filepath.Join(dir, "repos")+"\"\nskills:\n  manage-development-handoff:\n    worktree_root: \""+f.root+"\"\n")
	if e := os.MkdirAll(f.root, 0o755); e != nil {
		t.Fatal(e)
	}
	tgit(t, f.vault, "init", "-q", "-b", "main")
	return f
}

const pkg = "---\nhandoff: DH-001\ncase: 20260925-100000-ordenes\nrepository: https://github.com/acme/svc-orders.git\nbase: main\nbranch: issue/idempotent-orders\n---\n\n# Evitar órdenes duplicadas en el reintento\n\n## Tarea\n\nHacer idempotente el reintento de publicación.\n\n## Cambios\n\n- `src/order.go`: usar el ID de la orden como clave de idempotencia.\n\n## Criterios de aceptación\n\n- Un reintento con el mismo ID no publica dos veces (test unitario).\n\n## Contexto necesario\n\nEl consumidor descarta mensajes sin `orderId` (permalink https://github.com/acme/svc-consumer/blob/abc1234/main.go#L10).\n"

func run(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	var b bytes.Buffer
	e := Run(args, &b)
	m := map[string]any{}
	_ = json.Unmarshal(b.Bytes(), &m)
	return m, e
}

func TestPackageGate(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "ok.md", pkg)
	if _, issues, _ := CheckPackage(filepath.Join(dir, "ok.md")); blocking(issues) != nil {
		t.Fatalf("a complete package passes: %v", issues)
	}
	bad := strings.Replace(pkg, "## Cambios\n\n- `src/order.go`: usar el ID de la orden como clave de idempotencia.\n", "## Cambios\n\n", 1)
	bad = strings.Replace(bad, "El consumidor", "Ver [[svc-consumer]]. token: abc123 en /Users/x/tmp/log. El consumidor", 1)
	write(t, dir, "bad.md", bad)
	_, issues, _ := CheckPackage(filepath.Join(dir, "bad.md"))
	msg := blocking(issues).Error()
	for _, want := range []string{"Cambios / Changes is required", "[[svc-consumer]] points into the vault", "credential", "local path"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in %s", want, msg)
		}
	}
}

func TestStartStatusRefresh(t *testing.T) {
	f := setup(t)
	write(t, f.vault, ".investigations/20260925-100000-ordenes/handoffs/DH-001.md", pkg)
	p := ".investigations/20260925-100000-ordenes/handoffs/DH-001.md"
	preview, e := run(t, "start", "--vault", f.vault, "--package", p)
	if e != nil || preview["applied"] != false {
		t.Fatalf("preview %v %v", e, preview)
	}
	dest := filepath.Join(f.root, "svc-orders", "idempotent-orders")
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("a preview must not create the worktree")
	}
	if _, e := run(t, "start", "--vault", f.vault, "--package", p, "--apply"); e != nil {
		t.Fatal(e)
	}
	if branch := tgit(t, dest, "rev-parse", "--abbrev-ref", "HEAD"); branch != "issue/idempotent-orders" {
		t.Fatalf("branch %s", branch)
	}
	agents, _ := os.ReadFile(filepath.Join(dest, "AGENTS.md"))
	if !strings.Contains(string(agents), "Run make test.") || !strings.Contains(string(agents), "## Development handoff") {
		t.Fatal("the segment is added and the repository's own rules are kept")
	}
	if st := tgit(t, dest, "status", "--porcelain"); strings.Contains(st, ".handoff") || !strings.Contains(st, "AGENTS.md") {
		t.Fatalf(".handoff/ stays out of Git; only the stable segment is a tracked change: %q", st)
	}
	// Reusing the worktree with the same package is a no-op; the segment is idempotent.
	again, e := run(t, "start", "--vault", f.vault, "--package", p, "--apply")
	if e != nil || !strings.Contains(toJSON(again["effects"]), "reuse-worktree") || strings.Contains(toJSON(again["effects"]), "managed-segment") {
		t.Fatalf("reuse %v %v", e, again)
	}
	// Work: a commit and a delta.
	write(t, dest, "src/order.go", "package src\n// idempotent\n")
	tgit(t, dest, "commit", "-qam", "feat: idempotent retry")
	write(t, dest, ".handoff/deltas.md", deltasHeader+"\n## DELTA-001 — El ID viene en el header\n- Handoff: DH-001\n- Type: definition\n- Detail: la clave es el header x-order-id.\n- Evidence: commit feat: idempotent retry\n")
	st, e := run(t, "status", "--vault", f.vault, "--worktree", dest)
	if e != nil || len(st["commits_since_base"].([]any)) != 1 || len(st["deltas"].([]any)) != 1 {
		t.Fatalf("status reads progress from Git and deltas: %v %v", e, st)
	}
	all, _ := run(t, "status", "--vault", f.vault)
	if all["count"].(float64) != 1 {
		t.Fatalf("status finds worktrees under the root: %v", all)
	}
	// The package changes: status reports it, refresh replaces the task and keeps deltas.
	write(t, f.vault, p, strings.Replace(pkg, "(test unitario)", "(test unitario y de integración)", 1))
	st, _ = run(t, "status", "--vault", f.vault, "--worktree", dest)
	if !strings.Contains(toJSON(st["handoffs"]), `"package_changed":true`) {
		t.Fatalf("a changed package is reported: %v", st["handoffs"])
	}
	r, e := run(t, "refresh", "--vault", f.vault, "--worktree", dest, "--handoff", "DH-001", "--apply")
	if e != nil || !strings.Contains(toJSON(r["effects"]), "Criterios de aceptación") {
		t.Fatalf("refresh %v %v", e, r)
	}
	task, _ := os.ReadFile(filepath.Join(dest, ".handoff", "DH-001.md"))
	deltas, _ := os.ReadFile(filepath.Join(dest, ".handoff", "deltas.md"))
	if !strings.Contains(string(task), "integración") || !strings.Contains(string(deltas), "DELTA-001") {
		t.Fatal("refresh updates the task and keeps the deltas")
	}
}

func TestSegmentReachesClaudeWhenItDoesNotImportAgents(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "CLAUDE.md", "# Claude rules\n")
	if files := instructionFiles(dir); len(files) != 2 {
		t.Fatal("a CLAUDE.md that does not import AGENTS.md gets the segment too")
	}
	write(t, dir, "CLAUDE.md", "@AGENTS.md\n")
	if files := instructionFiles(dir); len(files) != 1 {
		t.Fatal("a CLAUDE.md importing AGENTS.md needs no copy")
	}
	os.Remove(filepath.Join(dir, "CLAUDE.md"))
	write(t, dir, "CLAUDE.local.md", "# personal\n")
	if files := instructionFiles(dir); len(files) != 2 || !strings.HasSuffix(files[1], "CLAUDE.local.md") {
		t.Fatal("a personal CLAUDE.local.md hides AGENTS.md from Claude Code, so it gets the segment")
	}
	write(t, dir, "AGENTS.md", "# Rules\n\n"+strings.Replace(Segment(), "Development handoff", "Old handoff policy", 1))
	next, changed, _ := withSegment(filepath.Join(dir, "AGENTS.md"))
	if !changed || strings.Count(string(next), segmentStart) != 1 || !strings.Contains(string(next), "# Rules") {
		t.Fatal("an outdated segment is replaced in place")
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
