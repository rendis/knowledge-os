package devhandoff

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	write(t, f.vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+filepath.ToSlash(filepath.Join(dir, "repos"))+"\"\nskills:\n  manage-development-handoff:\n    worktree_root: \""+filepath.ToSlash(f.root)+"\"\n")
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

// Publishing moves the case, but existing worktree copies still record its old path.
func TestRefreshAfterCasePublication(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		for _, changed := range []bool{false, true} {
			t.Run(fmt.Sprintf("absolute=%t/changed=%t", absolute, changed), func(t *testing.T) {
				f := setup(t)
				oldDir := ".investigations/20260925-100000-ordenes"
				newDir := "investigations/20260925-100000-ordenes"
				rel := oldDir + "/handoffs/DH-001.md"
				write(t, f.vault, rel, pkg)
				source := rel
				if absolute {
					source = filepath.Join(f.vault, rel)
				}
				if _, err := run(t, "start", "--vault", f.vault, "--package", source, "--apply"); err != nil {
					t.Fatal(err)
				}
				dest := filepath.Join(f.root, "svc-orders", "idempotent-orders")
				taskPath := filepath.Join(dest, ".handoff", "DH-001.md")
				recorded, err := ReadPackage(taskPath)
				if err != nil {
					t.Fatal(err)
				}
				// Exercise both persisted path forms, regardless of temp-directory symlinks.
				write(t, dest, ".handoff/DH-001.md", strings.Replace(recorded.Text, "package: "+recorded.Fields["package"], "package: "+source, 1))
				before, err := ReadPackage(taskPath)
				if err != nil {
					t.Fatal(err)
				}
				deltas := deltasHeader + "\nRetained implementation evidence.\n"
				write(t, dest, ".handoff/deltas.md", deltas)
				if err := os.MkdirAll(filepath.Join(f.vault, "investigations"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(f.vault, oldDir), filepath.Join(f.vault, newDir)); err != nil {
					t.Fatal(err)
				}
				next := pkg
				if changed {
					next = strings.Replace(pkg, "(test unitario)", "(test unitario y de integración)", 1)
				}
				newRel := newDir + "/handoffs/DH-001.md"
				write(t, f.vault, newRel, next)
				status, err := run(t, "status", "--vault", f.vault, "--worktree", dest)
				if err != nil {
					t.Fatal(err)
				}
				if changed && !strings.Contains(toJSON(status["handoffs"]), `"package_changed":true`) {
					t.Errorf("status must detect edits after publication: %v", status)
				}
				preview, err := run(t, "refresh", "--vault", f.vault, "--worktree", dest, "--handoff", "DH-001")
				if err != nil {
					t.Fatalf("refresh must resolve the published package: %v", err)
				}
				if !strings.Contains(toJSON(preview["effects"]), "replace") {
					t.Fatalf("relocated metadata needs refresh: %v", preview)
				}
				unchanged, _ := os.ReadFile(taskPath)
				if string(unchanged) != before.Text {
					t.Fatal("preview changed the worktree task")
				}
				if _, err := run(t, "refresh", "--vault", f.vault, "--worktree", dest, "--handoff", "DH-001", "--apply"); err != nil {
					t.Fatal(err)
				}
				after, err := ReadPackage(taskPath)
				if err != nil {
					t.Fatal(err)
				}
				if after.Fields["package"] != newRel || after.Fields["package-sha256"] != digest([]byte(next)) || after.Fields["base-commit"] != before.Fields["base-commit"] || after.Fields["started"] != before.Fields["started"] {
					t.Fatalf("refresh must retain execution metadata and point to the published revision: %v", after.Fields)
				}
				gotDeltas, _ := os.ReadFile(filepath.Join(dest, ".handoff/deltas.md"))
				if string(gotDeltas) != deltas {
					t.Fatal("refresh changed the deltas")
				}
				again, err := run(t, "refresh", "--vault", f.vault, "--worktree", dest, "--handoff", "DH-001", "--apply")
				if err != nil || !strings.Contains(toJSON(again["effects"]), "unchanged") {
					t.Fatalf("repeat refresh must be a no-op: %v %v", again, err)
				}
				if err := os.Remove(filepath.Join(f.vault, newRel)); err != nil {
					t.Fatal(err)
				}
				missing, err := run(t, "status", "--vault", f.vault, "--worktree", dest)
				if err != nil || !strings.Contains(toJSON(missing["handoffs"]), `"package_error":`) {
					t.Fatalf("a missing package must be reported explicitly: %v %v", missing, err)
				}
			})
		}
	}
}

func TestPublishedPackageResolutionBoundaries(t *testing.T) {
	const oldRel = ".investigations/20260925-100000-ordenes/handoffs/DH-001.md"
	const newRel = "investigations/20260925-100000-ordenes/handoffs/DH-001.md"
	for _, field := range []string{"case", "handoff", "repository", "branch"} {
		t.Run("different-"+field, func(t *testing.T) {
			vault := t.TempDir()
			write(t, vault, oldRel, pkg)
			cur, err := ReadPackage(filepath.Join(vault, oldRel))
			if err != nil {
				t.Fatal(err)
			}
			cur.Fields["package"] = oldRel
			if err := os.Remove(cur.Path); err != nil {
				t.Fatal(err)
			}
			write(t, vault, newRel, strings.Replace(pkg, field+": "+cur.Fields[field], field+": different", 1))
			if _, err := resolvePackage(vault, cur); err == nil {
				t.Fatal("must reject a different " + field)
			}
		})
	}
	t.Run("recorded-source-wins", func(t *testing.T) {
		vault := t.TempDir()
		write(t, vault, oldRel, pkg)
		write(t, vault, newRel, strings.Replace(pkg, "(test unitario)", "(new test)", 1))
		cur, _ := ReadPackage(filepath.Join(vault, oldRel))
		cur.Fields["package"] = oldRel
		got, err := resolvePackage(vault, cur)
		if err != nil || got.SHA256 != cur.SHA256 {
			t.Fatalf("an existing recorded source remains authoritative: %v", err)
		}
	})
	for _, mode := range []string{"custom-path", "other-vault", "directory", "missing-published"} {
		t.Run(mode, func(t *testing.T) {
			vault := t.TempDir()
			write(t, vault, "task.md", pkg)
			cur, _ := ReadPackage(filepath.Join(vault, "task.md"))
			cur.Fields["package"] = oldRel
			if mode != "missing-published" {
				write(t, vault, newRel, pkg)
			}
			switch mode {
			case "custom-path":
				cur.Fields["package"] = "custom/DH-001.md"
			case "other-vault":
				cur.Fields["package"] = filepath.Join(t.TempDir(), oldRel)
			case "directory":
				if err := os.MkdirAll(filepath.Join(vault, oldRel), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := resolvePackage(vault, cur); err == nil {
				t.Fatal("must not silently substitute a package: " + mode)
			}
		})
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

func TestSeveralTasksShareOneWorktree(t *testing.T) {
	f := setup(t)
	dir := ".investigations/20260925-100000-ordenes/handoffs/"
	write(t, f.vault, dir+"DH-001.md", pkg)
	second := strings.Replace(strings.Replace(pkg, "handoff: DH-001", "handoff: DH-002\ndepends-on: DH-001", 1), "# Evitar órdenes duplicadas en el reintento", "# Registrar el reintento en la auditoría", 1)
	write(t, f.vault, dir+"DH-002.md", second)
	if _, e := run(t, "start", "--vault", f.vault, "--package", dir+"DH-002.md"); e == nil || !strings.Contains(e.Error(), "start DH-001 first") {
		t.Fatalf("a dependent task starts after the task it builds on: %v", e)
	}
	for _, id := range []string{"DH-001", "DH-002"} {
		if _, e := run(t, "start", "--vault", f.vault, "--package", dir+id+".md", "--apply"); e != nil {
			t.Fatal(e)
		}
	}
	dest := filepath.Join(f.root, "svc-orders", "idempotent-orders")
	st, _ := run(t, "status", "--vault", f.vault, "--worktree", dest)
	if toJSON(st["next_task"]) != `"DH-001"` || !strings.Contains(toJSON(st["handoffs"]), `"state":"blocked"`) {
		t.Fatalf("two tasks in one worktree, the dependent one blocked: %v", st)
	}
	write(t, dest, "src/order.go", "package src\n// idempotent\n")
	tgit(t, dest, "commit", "-qam", "feat: idempotent retry\n\nHandoff: DH-001")
	write(t, dest, ".handoff/deltas.md", deltasHeader+"\n## DELTA-001 — Criterios de DH-001\n- Handoff: DH-001\n- Type: verification\n- Detail: AC ok.\n- Evidence: go test\n")
	st, _ = run(t, "status", "--vault", f.vault, "--worktree", dest)
	hs := toJSON(st["handoffs"])
	if !strings.Contains(hs, `"handoff":"DH-001"`) || !strings.Contains(hs, `"state":"verified"`) || toJSON(st["next_task"]) != `"DH-002"` {
		t.Fatalf("commits and deltas are attributed per task: %v", st)
	}
}
