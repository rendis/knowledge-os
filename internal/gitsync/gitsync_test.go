package gitsync

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"knowledge-os/internal/cases"
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

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("git %v: %s", args, b)
	}
}

func vault(t *testing.T) string {
	t.Helper()
	v := t.TempDir()
	for _, m := range []string{"AGENTS.md", "90-Meta/Convenciones.md", "90-Meta/Auditoria - Framework.md"} {
		write(t, v, m, "x\n")
	}
	write(t, v, "00-Home.md", "---\ntipo: indice\n---\n# Home\n\n[[Sales]]\n")
	write(t, v, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"sales\"\n    name: \"Sales\"\n")
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas.\n")
	run(t, v, "init", "-q", "-b", "main")
	run(t, v, "config", "user.name", "t")
	run(t, v, "config", "user.email", "t@t")
	run(t, v, "config", "core.autocrlf", "false")
	run(t, v, "add", "-A")
	run(t, v, "commit", "-q", "-m", "base")
	return v
}

func call(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	var out bytes.Buffer
	e := Run(args, &out)
	if e != nil && out.Len() == 0 {
		t.Logf("sync failed before emitting a result: %v", e)
	}
	m := map[string]any{}
	_ = json.Unmarshal(out.Bytes(), &m)
	return m, e
}

func TestBranchReviewVerifyFinish(t *testing.T) {
	v := vault(t)
	if _, e := call(t, "start", "--vault", v, "--name", "sales-refresh"); e != nil {
		t.Fatal(e)
	}
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas y devoluciones.\n")
	if res, e := call(t, "verify", "--vault", v); e == nil || res["ok"] != false {
		t.Fatalf("uncommitted content must not verify: %v", res)
	}
	run(t, v, "commit", "-qam", "docs: update sales")
	res, e := call(t, "verify", "--vault", v)
	if e == nil || !strings.Contains(strings.Join(toStrings(res["problems"]), " "), "no review recorded") {
		t.Fatalf("content without review must not verify: %v", res)
	}
	if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "fresh-session"); e != nil {
		t.Fatal(e)
	}
	if res, e := call(t, "verify", "--vault", v); e != nil || res["ok"] != true {
		t.Fatalf("reviewed content must verify: %v %v", e, res)
	}
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas, devoluciones y cambios.\n")
	run(t, v, "commit", "-qam", "docs: late change")
	res, e = call(t, "verify", "--vault", v)
	if e == nil || !strings.Contains(strings.Join(toStrings(res["problems"]), " "), "changed after the accepted review") {
		t.Fatalf("a change after review must invalidate it: %v", res)
	}
	if _, e := call(t, "finish", "--vault", v); e == nil {
		t.Fatal("finish must refuse unreviewed content")
	}
	if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "fresh-session"); e != nil {
		t.Fatal(e)
	}
	res, e = call(t, "finish", "--vault", v)
	if e != nil || res["merged"] != "sync/sales-refresh" {
		t.Fatalf("finish %v %v", e, res)
	}
	b, _ := os.ReadFile(filepath.Join(v, "10-Sistemas/Sales.md"))
	if !strings.Contains(string(b), "cambios") || current(v) != "main" {
		t.Fatal("base must contain the reviewed content")
	}
}

func localPublishedCase(t *testing.T, v string) string {
	t.Helper()
	const id = "20260925-100000-local-case"
	var out bytes.Buffer
	if err := cases.Run([]string{"new", "--vault", v, "--id", id, "--date", "2026-09-25", "--title", "Local case", "--type", "understanding", "--objective", "Keep an auditable local case."}, &out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(v, "investigations"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(v, ".investigations", id), filepath.Join(v, "investigations", id)); err != nil {
		t.Fatal(err)
	}
	run(t, v, "add", "-A")
	run(t, v, "commit", "-qm", "published baseline")
	proof := filepath.Join(t.TempDir(), "proof.txt")
	if err := os.WriteFile(proof, []byte("Local proof.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := cases.Run([]string{"add", "--vault", v, "--id", id, "--kind", "evidence", "--text", "The local check returned one result.", "--source", "query: local fixture", "--level", "demonstrated", "--file", proof}, &out); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPublishPendingCaseWithReviewGates(t *testing.T) {
	for _, base := range []string{"main", "develop"} {
		t.Run(base, func(t *testing.T) {
			v := vault(t)
			if base != "main" {
				run(t, v, "branch", "-m", base)
			}
			write(t, v, "scratch.txt", "Original user content.\n")
			id := localPublishedCase(t, v)
			caseDir := filepath.Join("investigations", id)
			run(t, v, "add", "--", caseDir) // retain an already staged case update
			var state bytes.Buffer
			if err := cases.Run([]string{"state", "--vault", v, "--id", id, "--text", "E-001 records the local result. Next: publish the reviewed case."}, &state); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(v, caseDir, "investigation.md"))
			if err != nil {
				t.Fatal(err)
			}
			write(t, v, "scratch.txt", "Unrelated pending work.\n")
			before, _ := git(v, "rev-parse", "HEAD")
			index, _ := git(v, "diff", "--cached", "--binary")
			wt, _ := git(v, "worktree", "list", "--porcelain")
			if _, err := call(t, "start", "--vault", v, "--name", "publish-case"); err == nil {
				t.Fatal("ordinary sync start must still require clean knowledge")
			}
			if _, err := call(t, "start", "--vault", v, "--name", "publish-case", "--case", id); err != nil {
				t.Fatalf("explicit case publication must carry the pending case: %v", err)
			}
			if got, _ := git(v, "rev-parse", "HEAD"); got != before {
				t.Fatal("transition committed work")
			}
			if got, _ := git(v, "diff", "--cached", "--binary"); got != index {
				t.Fatal("transition changed staged content")
			}
			afterBody, err := os.ReadFile(filepath.Join(v, caseDir, "investigation.md"))
			if err != nil || !bytes.Equal(body, afterBody) {
				t.Fatal("transition changed unstaged case content")
			}
			if current(v) != "sync/publish-case" {
				t.Fatal("publication did not enter its authorized branch")
			}
			if got, _ := git(v, "worktree", "list", "--porcelain"); strings.Count(got, "worktree ") != strings.Count(wt, "worktree ") {
				t.Fatal("transition created a worktree")
			}
			if res, err := call(t, "verify", "--vault", v); err == nil || res["ok"] != false {
				t.Fatal("pending content passed verify")
			}
			run(t, v, "add", "--", caseDir)
			run(t, v, "commit", "-qm", "docs: case evidence")
			if res, err := call(t, "verify", "--vault", v); err == nil || !strings.Contains(fmt.Sprint(res), "no review recorded") {
				t.Fatalf("unreviewed publication passed: %v %v", res, err)
			}
			if _, err := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "fresh-reviewer"); err != nil {
				t.Fatal(err)
			}
			if res, err := call(t, "verify", "--vault", v); err != nil || res["ok"] != true {
				t.Fatalf("accepted case did not verify: %v %v", res, err)
			}
			write(t, v, filepath.Join(caseDir, "investigation.md"), "invalid case\n")
			run(t, v, "add", "--", caseDir)
			run(t, v, "commit", "-qm", "invalid late edit")
			if _, err := call(t, "finish", "--vault", v); err == nil {
				t.Fatal("finish accepted invalid/unreviewed content")
			}
			if head, _ := git(v, "rev-parse", base); head != before {
				t.Fatal("failed finish moved the base")
			}
			run(t, v, "revert", "--no-edit", "HEAD")
			var out bytes.Buffer
			if err := cases.Run([]string{"state", "--vault", v, "--id", id, "--text", "E-001 records the local result. Next: validate the release candidate."}, &out); err != nil {
				t.Fatal(err)
			}
			run(t, v, "add", "--", caseDir)
			run(t, v, "commit", "-qm", "docs: corrected case state")
			if _, err := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "fresh-reviewer"); err != nil {
				t.Fatal(err)
			}
			if _, err := call(t, "finish", "--vault", v); err != nil {
				t.Fatal(err)
			}
			pending, err := os.ReadFile(filepath.Join(v, "scratch.txt"))
			if err != nil || string(pending) != "Unrelated pending work.\n" || current(v) != base {
				t.Fatal("publication lost unrelated work or finished on another base")
			}
		})
	}
}

func TestPendingCasePublicationRefusesOutOfScopeChanges(t *testing.T) {
	for _, mode := range []string{"other-knowledge", "staged-unrelated", "another-base", "rename-other-case", "rename-user-file", "missing-case", "unpublished-case", "existing-sync", "no-case-changes", "other-command", "empty-case"} {
		t.Run(mode, func(t *testing.T) {
			v := vault(t)
			write(t, v, "investigations/another-case/artifacts/proof.txt", "Other case evidence.\n")
			write(t, v, "user-file.txt", "User content.\n")
			id := localPublishedCase(t, v)
			args := []string{"start", "--vault", v, "--name", "publish-case", "--case", id}
			switch mode {
			case "other-knowledge":
				write(t, v, "10-Sistemas/Sales.md", "Unrelated note edit.\n")
			case "staged-unrelated":
				write(t, v, "scratch.txt", "Staged user file.\n")
				run(t, v, "add", "scratch.txt")
			case "another-base":
				run(t, v, "branch", "other")
				args = append(args, "--base", "other")
			case "rename-other-case":
				run(t, v, "mv", "investigations/another-case/artifacts/proof.txt", "investigations/"+id+"/artifacts/renamed.txt")
			case "rename-user-file":
				run(t, v, "mv", "user-file.txt", "investigations/"+id+"/artifacts/renamed.txt")
			case "missing-case":
				args[len(args)-1] = "20260925-100000-missing"
			case "unpublished-case":
				var out bytes.Buffer
				if err := cases.Run([]string{"new", "--vault", v, "--id", "20260925-100000-private-case", "--date", "2026-09-25", "--title", "Private case", "--type", "understanding", "--objective", "Keep this case private."}, &out); err != nil {
					t.Fatal(err)
				}
				args[len(args)-1] = "20260925-100000-private-case"
			case "existing-sync":
				run(t, v, "switch", "-c", "sync/existing")
			case "no-case-changes":
				run(t, v, "add", "--", "investigations/"+id)
				run(t, v, "commit", "-qm", "local case checkpoint")
			case "other-command":
				args[0] = "status"
			case "empty-case":
				args[len(args)-1] = ""
			}
			refs, _ := git(v, "show-ref")
			branch := current(v)
			index, _ := git(v, "diff", "--cached", "--binary")
			body, err := os.ReadFile(filepath.Join(v, "investigations", id, "investigation.md"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := call(t, args...); err == nil {
				t.Fatal("transition accepted an invalid scope")
			}
			afterRefs, _ := git(v, "show-ref")
			afterIndex, _ := git(v, "diff", "--cached", "--binary")
			afterBody, _ := os.ReadFile(filepath.Join(v, "investigations", id, "investigation.md"))
			if refs != afterRefs || branch != current(v) || index != afterIndex || !bytes.Equal(body, afterBody) {
				t.Fatal("a refused transition changed branches, index or case content")
			}
		})
	}
}

func TestStartRejectsSinglePendingKnowledgeFile(t *testing.T) {
	for _, path := range []string{"10-Sistemas/Sales.md", "investigations/local-case/artifacts/proof with spaces.txt"} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			v := vault(t)
			write(t, v, path, "Original content.\n")
			run(t, v, "add", "--", path)
			run(t, v, "commit", "-qm", "baseline proof")
			write(t, v, path, "Pending content.\n")
			if _, err := call(t, "start", "--vault", v, "--name", "unexpected"); err == nil {
				t.Fatal("the first tracked status entry must not escape the clean-knowledge gate")
			}
			if current(v) != "main" {
				t.Fatal("a refused start switched branches")
			}
		})
	}
}

func TestVerifyRejectsIntroducedStructuralIssues(t *testing.T) {
	v := vault(t)
	if _, e := call(t, "start", "--vault", v, "--name", "broken"); e != nil {
		t.Fatal(e)
	}
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nVer [[Nota inexistente]].\n")
	run(t, v, "commit", "-qam", "docs: broken link")
	if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "r"); e != nil {
		t.Fatal(e)
	}
	res, e := call(t, "verify", "--vault", v)
	if e == nil || len(toStrings(res["new_structural_issues"])) == 0 {
		t.Fatalf("a new broken link must fail verify: %v", res)
	}
}

func TestAcknowledgementKeepsSchema(t *testing.T) {
	v := vault(t)
	if _, e := call(t, "acknowledge", "--vault", v, "--repo", "SVC-b", "--commit", "0123456789abcdef", "--decision", "no-durable-node", "--date", "2026-09-25"); e != nil {
		t.Fatal(e)
	}
	if _, e := call(t, "acknowledge", "--vault", v, "--repo", "SVC-a", "--commit", "abcdefabcdef", "--decision", "no-documentation-change", "--date", "2026-09-25"); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(v, ackRel))
	want := `{"repositories":[{"analysis_date":"2026-09-25","analyzed_sha":"abcdefabcdef","branch":"main","decision":"no-documentation-change","repository":"SVC-a"},{"analysis_date":"2026-09-25","analyzed_sha":"0123456789ab","branch":"main","decision":"no-durable-node","repository":"SVC-b"}],"version":1}` + "\n"
	if string(b) != want {
		t.Fatalf("acknowledgement file\n%s\nwant\n%s", b, want)
	}
}

func TestAcknowledgementRejectsUnsuccessfulAttemptsWithoutWriting(t *testing.T) {
	for _, decision := range []string{"review-rejected", "inspection-limited"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing=%t", decision, existing), func(t *testing.T) {
				v := vault(t)
				const prior = `{"repositories":[{"analysis_date":"2026-08-11","analyzed_sha":"123456789abc","branch":"main","decision":"review-rejected","repository":"SVC-a"}],"version":1}` + "\n"
				if existing {
					write(t, v, ackRel, prior)
				}
				_, err := call(t, "acknowledge", "--vault", v, "--repo", "SVC-a", "--commit", "fedcba987654", "--decision", decision)
				if err == nil || !strings.Contains(err.Error(), "unsuccessful") {
					t.Fatalf("unsuccessful attempt must not create an acknowledgement: %v", err)
				}
				b, err := os.ReadFile(filepath.Join(v, ackRel))
				if existing {
					if err != nil || string(b) != prior {
						t.Fatalf("failed write changed historical state: %s, %v", b, err)
					}
				} else if !os.IsNotExist(err) {
					t.Fatalf("failed write created state: %s, %v", b, err)
				}
			})
		}
	}
}

func TestAcknowledgementPreservesOtherHistoricalAttempts(t *testing.T) {
	v := vault(t)
	legacy := ackEntry{"2026-08-11", "123456789abc", "main", "review-rejected", "SVC-a"}
	b, err := json.Marshal(map[string]any{"version": 1, "repositories": []ackEntry{legacy}})
	if err != nil {
		t.Fatal(err)
	}
	write(t, v, ackRel, string(b)+"\n")
	if _, err := call(t, "acknowledge", "--vault", v, "--repo", "SVC-b", "--commit", "fedcba987654", "--decision", "no-durable-node"); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(filepath.Join(v, ackRel))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Version      int        `json:"version"`
		Repositories []ackEntry `json:"repositories"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != 1 || len(doc.Repositories) != 2 || doc.Repositories[0] != legacy || doc.Repositories[1].Decision != "no-durable-node" {
		t.Fatalf("accepted acknowledgement lost historical records: %s", b)
	}
}

func toStrings(v any) []string {
	out := []string{}
	for _, x := range asList(v) {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func asList(v any) []any { l, _ := v.([]any); return l }

func TestPullFastForwardsAndReportsOverlap(t *testing.T) {
	origin := vault(t)
	clone := t.TempDir()
	if b, e := exec.Command("git", "clone", "-q", origin, clone).CombinedOutput(); e != nil {
		t.Fatal(string(b))
	}
	write(t, origin, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nUpstream.\n")
	run(t, origin, "commit", "-qam", "upstream")
	if res, e := call(t, "pull", "--vault", clone); e != nil || res["status"] != "fast-forwarded" {
		t.Fatalf("pull %v %v", e, res)
	}
	if _, e := call(t, "start", "--vault", clone, "--name", "local"); e != nil {
		t.Fatal(e)
	}
	write(t, clone, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nLocal.\n")
	run(t, clone, "commit", "-qam", "local")
	write(t, origin, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nUpstream again.\n")
	run(t, origin, "commit", "-qam", "upstream 2")
	res, e := call(t, "pull", "--vault", clone)
	if e != nil || res["status"] != "diverged" || len(toStrings(res["changed_on_both_sides"])) != 1 {
		t.Fatalf("divergence must list overlapping notes: %v %v", e, res)
	}
}

func TestChangedKeepsNonASCIIPaths(t *testing.T) {
	v := vault(t)
	if _, e := call(t, "start", "--vault", v, "--name", "flows"); e != nil {
		t.Fatal(e)
	}
	write(t, v, "30-Flujos/Flujo - Sincronización.md", "---\ntipo: flujo\nsistema: \"[[Sales]]\"\n---\n# Flujo\n\nSincroniza ventas.\n")
	run(t, v, "add", "-A")
	run(t, v, "commit", "-qm", "docs: add flow")
	res, e := call(t, "status", "--vault", v)
	if e != nil || !strings.Contains(strings.Join(toStrings(res["changed"]), " "), "30-Flujos/Flujo - Sincronización.md") {
		t.Fatalf("an accented path must be listed as changed: %v %v", e, res)
	}
}

func TestDiscoveryStateOnlyPublishesWhenValid(t *testing.T) {
	v := vault(t)
	if _, e := call(t, "start", "--vault", v, "--name", "classify"); e != nil {
		t.Fatal(e)
	}
	store := "90-Meta/discovery/classifications.json"
	write(t, v, store, `{"schema":1,"dependencies":{"example.org/bus":{"choice":"not-an-option","confidence":0.9,"source":"agent"}}}`+"\n")
	run(t, v, "add", "-A")
	run(t, v, "commit", "-qm", "chore: classify")
	if res, e := call(t, "verify", "--vault", v); e == nil || res["ok"] != false {
		t.Fatalf("an invalid judgment must not verify: %v", res)
	}
	write(t, v, store, `{"schema":1,"dependencies":{"example.org/bus":{"choice":"messaging","confidence":0.9,"source":"agent"}}}`+"\n")
	run(t, v, "commit", "-qam", "chore: fix classification")
	if res, e := call(t, "verify", "--vault", v); e != nil || res["ok"] != true || res["discovery_state_only"] != true {
		t.Fatalf("valid discovery state must verify without a knowledge review: %v %v", e, res)
	}
	if res, e := call(t, "finish", "--vault", v); e != nil || res["merged"] != "sync/classify" {
		t.Fatalf("finish %v %v", e, res)
	}
}

func TestPublishingACaseRunsTheCaseGate(t *testing.T) {
	v := vault(t)
	if _, e := call(t, "start", "--vault", v, "--name", "publish-case"); e != nil {
		t.Fatal(e)
	}
	c := "---\nid: 20260925-100000-caso\ntitle: \"Caso\"\ntype: understanding\nstatus: open\ncreated: 2026-09-25\n---\n\n# Caso\n\n## Objetivo y alcance\n\nx\n\n## Estado actual\n\nx\n\n## Evidencia\n\n- **E-001** — hecho sin fuente.\n\n## Conclusiones\n\n## Preguntas abiertas\n"
	write(t, v, "investigations/20260925-100000-caso/investigation.md", c)
	run(t, v, "add", "-A")
	run(t, v, "commit", "-qm", "docs: publish case")
	res, _ := call(t, "verify", "--vault", v)
	if !strings.Contains(strings.Join(toStrings(res["problems"]), " "), "case gate failed") {
		t.Fatalf("an unsourced evidence record must block publication: %v", res)
	}
	write(t, v, "investigations/20260925-100000-caso/investigation.md", strings.Replace(c, "hecho sin fuente.", "hecho. Fuente: `src/a.go@abc1234`.", 1))
	run(t, v, "commit", "-qam", "docs: cite source")
	if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "fresh-session"); e != nil {
		t.Fatal(e)
	}
	if res, e := call(t, "verify", "--vault", v); e != nil || res["ok"] != true {
		t.Fatalf("a sourced, reviewed case publishes: %v %v", e, res)
	}
}

func TestSyncKeepsTheBranchItStartedFrom(t *testing.T) {
	v := vault(t)
	run(t, v, "switch", "-qc", "develop")
	// A stale origin/HEAD (the remote's default changed since the clone) must not decide the base.
	run(t, v, "update-ref", "refs/remotes/origin/main", "HEAD")
	run(t, v, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	if res, e := call(t, "start", "--vault", v, "--name", "refresh"); e != nil || res["base"] != "develop" {
		t.Fatalf("start from the checked-out branch: %v %v", e, res)
	}
	if res, e := call(t, "status", "--vault", v); e != nil || res["base"] != "develop" {
		t.Fatalf("the sync branch remembers its base: %v %v", e, res)
	}
}

func TestVerifyRejectsCopiedRepositoryParagraphs(t *testing.T) {
	v := vault(t)
	fact := "El manejador elimina los ceros iniciales de tienda, terminal y folio antes de componer el identificador, y usa el id del evento cuando falta la tienda o el terminal."
	write(t, v, "20-Repos/sales/orders-command.md", "---\ntipo: suscriptor\n---\n# orders-command\n\n"+fact+"\n")
	run(t, v, "add", "-A")
	run(t, v, "commit", "-qm", "docs: repository note")
	if _, e := call(t, "start", "--vault", v, "--name", "copy"); e != nil {
		t.Fatal(e)
	}
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas.\n\n"+fact+"\n")
	run(t, v, "commit", "-qam", "docs: copy a repository paragraph")
	if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "r"); e != nil {
		t.Fatal(e)
	}
	res, e := call(t, "verify", "--vault", v)
	if e == nil || len(asList(res["copied_paragraphs"])) != 1 || !strings.Contains(strings.Join(toStrings(res["problems"]), " "), "copies 1 paragraph") {
		t.Fatalf("a knowledge note that copies a repository note must fail verify: %v", res)
	}
}

func TestVerifyAllowNoChangeForPushes(t *testing.T) {
	v := vault(t)
	write(t, v, "AGENTS.md", "kernel update\n")
	run(t, v, "commit", "-qam", "chore: update the kernel")
	if res, e := call(t, "verify", "--vault", v, "--base", "HEAD~1"); e == nil || res["ok"] != false {
		t.Fatalf("without the flag a range with no knowledge change is refused: %v", res)
	}
	if res, e := call(t, "verify", "--vault", v, "--base", "HEAD~1", "--allow-no-change"); e != nil || res["ok"] != true {
		t.Fatalf("a kernel-only push passes the CI gate: %v %v", e, res)
	}
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nEditado sin revisión.\n")
	run(t, v, "commit", "-qam", "docs: direct edit")
	if res, e := call(t, "verify", "--vault", v, "--base", "HEAD~1", "--allow-no-change"); e == nil || !strings.Contains(strings.Join(toStrings(res["problems"]), " "), "no accepted review covers") {
		t.Fatalf("a knowledge commit without review fails the CI gate: %v", res)
	}
}

func TestVerifyPushRangeWithSeveralSyncs(t *testing.T) {
	v := vault(t)
	pushed, _ := git(v, "rev-parse", "HEAD")
	sync := func(name, content string) {
		if _, e := call(t, "start", "--vault", v, "--name", name); e != nil {
			t.Fatal(e)
		}
		write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\n"+content+"\n")
		run(t, v, "commit", "-qam", "docs: "+name)
		if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "r"); e != nil {
			t.Fatal(e)
		}
		if _, e := call(t, "finish", "--vault", v); e != nil {
			t.Fatal(e)
		}
	}
	sync("first", "Sistema de ventas y devoluciones.")
	write(t, v, "AGENTS.md", "kernel update\n")
	run(t, v, "commit", "-qam", "chore: update the kernel")
	sync("second", "Sistema de ventas, devoluciones y cambios.")
	if res, e := call(t, "verify", "--vault", v, "--base", pushed, "--allow-no-change"); e != nil || res["ok"] != true {
		t.Fatalf("two reviewed syncs and a kernel update pass the push gate: %v %v", e, res)
	}
	write(t, v, "00-Home.md", "---\ntipo: indice\n---\n# Home\n\n[[Sales]] editado a mano.\n")
	run(t, v, "commit", "-qam", "docs: direct edit")
	sync("third", "Sistema de ventas.")
	res, e := call(t, "verify", "--vault", v, "--base", pushed, "--allow-no-change")
	if e == nil || len(toStrings(res["problems"])) != 1 || !strings.Contains(toStrings(res["problems"])[0], "no accepted review covers") {
		t.Fatalf("only the direct commit between syncs is unreviewed: %v", res)
	}
}

func TestPullRequestVerifiesTheBranchHeadNotTheTemporaryMerge(t *testing.T) {
	v := vault(t)
	if _, e := call(t, "start", "--vault", v, "--name", "pr"); e != nil {
		t.Fatal(e)
	}
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas y devoluciones.\n")
	run(t, v, "commit", "-qam", "docs: pr")
	if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "r"); e != nil {
		t.Fatal(e)
	}
	head, _ := git(v, "rev-parse", "HEAD")
	// The base moved while the PR was open, so GitHub's refs/pull/N/merge is a true merge commit.
	run(t, v, "switch", "-q", "main")
	write(t, v, "AGENTS.md", "kernel update\n")
	run(t, v, "commit", "-qam", "chore: update the kernel")
	run(t, v, "update-ref", "refs/remotes/origin/main", "HEAD")
	run(t, v, "switch", "-q", "--detach", "HEAD")
	run(t, v, "merge", "-q", "--no-ff", "-m", "Merge "+head+" into main", head)
	if res, e := call(t, "verify", "--vault", v, "--base", "origin/main", "--allow-no-change"); e != nil || res["ok"] != true {
		t.Fatalf("the review record in the tree covers the temporary merge too: %v %v", e, res)
	}
	run(t, v, "switch", "-q", "--detach", head)
	if res, e := call(t, "verify", "--vault", v, "--base", "origin/main", "--allow-no-change"); e != nil || res["ok"] != true {
		t.Fatalf("the PR's own head, as the workflow checks it out, passes: %v %v", e, res)
	}
}

// reviewedBranch leaves a sync/ branch with one reviewed knowledge commit and main moved by a kernel update, as a
// pull request finds it; it returns main's commit before the merge.
func reviewedBranch(t *testing.T, v string) string {
	t.Helper()
	if _, e := call(t, "start", "--vault", v, "--name", "pr"); e != nil {
		t.Fatal(e)
	}
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas y devoluciones.\n")
	run(t, v, "commit", "-qam", "docs: pr")
	if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "r"); e != nil {
		t.Fatal(e)
	}
	run(t, v, "switch", "-q", "main")
	write(t, v, "AGENTS.md", "kernel update\n")
	run(t, v, "commit", "-qam", "chore: update the kernel")
	before, _ := git(v, "rev-parse", "HEAD")
	return before
}

func TestReviewSurvivesRebaseAndSquashMerges(t *testing.T) {
	// GitHub's rebase merge replays the branch's non-empty commits; a squash keeps only the PR title.
	for name, merge := range map[string][]string{
		"rebase": {"rebase", "-q", "--no-keep-empty", "main", "sync/pr"},
		"squash": {"merge", "-q", "--squash", "sync/pr"},
	} {
		t.Run(name, func(t *testing.T) {
			v := vault(t)
			before := reviewedBranch(t, v)
			run(t, v, merge...)
			if name == "squash" {
				run(t, v, "commit", "-qm", "Sales refresh (#1)")
			} else {
				run(t, v, "switch", "-q", "main")
				run(t, v, "merge", "-q", "--ff-only", "sync/pr")
			}
			if res, e := call(t, "verify", "--vault", v, "--base", before, "--allow-no-change"); e != nil || res["ok"] != true {
				t.Fatalf("the published content keeps its review: %v %v", e, res)
			}
			write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nEditado sin revisión.\n")
			run(t, v, "commit", "-qam", "docs: direct edit")
			if res, e := call(t, "verify", "--vault", v, "--base", before, "--allow-no-change"); e == nil || !strings.Contains(strings.Join(toStrings(res["problems"]), " "), "10-Sistemas/Sales.md changes knowledge that no accepted review covers") {
				t.Fatalf("an edit after the review stays unreviewed: %v", res)
			}
		})
	}
}

func TestTrailerOnlyReviewsStillCoverAFastForward(t *testing.T) {
	v := vault(t)
	pushed, _ := git(v, "rev-parse", "HEAD")
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas y devoluciones.\n")
	run(t, v, "commit", "-qam", "docs: pr")
	d := digestAt(v, "HEAD", []string{"10-Sistemas/Sales.md"})
	// The review as kos 0.22.4 and earlier recorded it: an empty commit with trailers.
	run(t, v, "commit", "-q", "--allow-empty", "-m", "chore(sync): record review accept\n\nKnowledge-Review: accept\nKnowledge-Reviewer: r\nKnowledge-Digest: "+d+"\nKnowledge-Base: "+pushed)
	if res, e := call(t, "verify", "--vault", v, "--base", pushed, "--allow-no-change"); e != nil || res["ok"] != true {
		t.Fatalf("a branch reviewed before the records existed still publishes: %v %v", e, res)
	}
}

func TestCumulativeReviewsSurviveMerge(t *testing.T) {
	v := vault(t)
	base, _ := git(v, "rev-parse", "HEAD")
	file := "10-Sistemas/Sales.md"
	initial := blobAt(v, base, file)
	run(t, v, "switch", "-qc", "sync/checkpoints")
	write(t, v, file, "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas y devoluciones.\n")
	run(t, v, "commit", "-qam", "docs: first checkpoint")
	checkpoint, _ := git(v, "rev-parse", "HEAD")
	write(t, v, file, "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas, devoluciones y cambios.\n")
	run(t, v, "commit", "-qam", "docs: final checkpoint")
	final, _ := git(v, "rev-parse", "HEAD")
	// Both accepted checkpoints cover the same base, as repeated `sync review`
	// calls do. The first record is a valid path that does not reach the final blob.
	for i, revision := range []string{checkpoint, final} {
		record := reviewFile{
			Verdict: "accept", Reviewer: "fresh-session", Base: base,
			Digest: digestAt(v, revision, []string{file}),
			Files:  map[string]blobChange{file: {From: initial, To: blobAt(v, revision, file)}},
		}
		body, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		write(t, v, fmt.Sprintf("%s/checkpoint-%d.json", reviewsDir, i), string(body))
	}
	run(t, v, "add", "-A")
	run(t, v, "commit", "-qm", "chore(sync): keep accepted checkpoints")
	run(t, v, "switch", "-q", "main")
	run(t, v, "merge", "-q", "--no-ff", "-m", "Merge reviewed checkpoints", "sync/checkpoints")
	if res, err := call(t, "verify", "--vault", v, "--base", base, "--allow-no-change"); err != nil || res["ok"] != true {
		t.Fatalf("the accepted final checkpoint must cover the merge despite an earlier dead end: %v %v", err, res)
	}
	write(t, v, file, "---\ntipo: sistema\n---\n# Sales\n\nCambio posterior sin revisión.\n")
	run(t, v, "commit", "-qam", "docs: unreviewed edit")
	if res, err := call(t, "verify", "--vault", v, "--base", base, "--allow-no-change"); err == nil || !strings.Contains(strings.Join(toStrings(res["problems"]), " "), "no accepted review covers") {
		t.Fatalf("searching accepted checkpoints must still reject an unreviewed final blob: %v %v", err, res)
	}
}

func TestRecoverAReviewLostByARebaseMerge(t *testing.T) {
	v := vault(t)
	pushed, _ := git(v, "rev-parse", "HEAD")
	// A review recorded before the records existed lives only in an empty commit, which a rebase merge drops.
	write(t, v, "10-Sistemas/Sales.md", "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas y devoluciones.\n")
	run(t, v, "commit", "-qam", "docs: pr")
	res, e := call(t, "verify", "--vault", v, "--base", pushed, "--allow-no-change")
	if e == nil || !strings.Contains(strings.Join(toStrings(res["problems"]), " "), "no accepted review covers") {
		t.Fatalf("the rebased content without its review fails the push gate: %v", res)
	}
	// Recovery: the reviewer records the published range on a sync/ branch against main before the merge.
	if _, e := call(t, "start", "--vault", v, "--name", "restore-review"); e != nil {
		t.Fatal(e)
	}
	if _, e := call(t, "review", "--vault", v, "--base", pushed, "--verdict", "accept", "--reviewer", "r", "--summary", "restores the review a rebase merge dropped"); e != nil {
		t.Fatal(e)
	}
	if res, e := call(t, "verify", "--vault", v, "--base", pushed, "--allow-no-change"); e != nil || res["ok"] != true {
		t.Fatalf("the restored review covers the published range: %v %v", e, res)
	}
	if res, e := call(t, "verify", "--vault", v, "--base", "main", "--allow-no-change"); e != nil || res["ok"] != true {
		t.Fatalf("the recovery branch adds no knowledge change: %v %v", e, res)
	}
}

func TestPushRangeUsesTheReviewedBase(t *testing.T) {
	v := vault(t)
	pushed, _ := git(v, "rev-parse", "HEAD")
	write(t, v, "00-Home.md", "---\ntipo: indice\n---\n# Home\n\n[[Sales]] editado a mano.\n")
	run(t, v, "commit", "-qam", "docs: direct edit")
	// A later sync that touches the same file does not cover the direct edit made before it started.
	if _, e := call(t, "start", "--vault", v, "--name", "home"); e != nil {
		t.Fatal(e)
	}
	write(t, v, "00-Home.md", "---\ntipo: indice\n---\n# Home\n\n[[Sales]] revisado.\n")
	run(t, v, "commit", "-qam", "docs: home")
	if _, e := call(t, "review", "--vault", v, "--verdict", "accept", "--reviewer", "r"); e != nil {
		t.Fatal(e)
	}
	if res, e := call(t, "verify", "--vault", v, "--base", pushed, "--allow-no-change"); e == nil || len(toStrings(res["problems"])) != 1 {
		t.Fatalf("the direct edit before the sync stays unreviewed: %v", res)
	}
	// A review recorded against the pushed base covers the whole range, the direct edit included.
	if _, e := call(t, "review", "--vault", v, "--base", pushed, "--verdict", "accept", "--reviewer", "r"); e != nil {
		t.Fatal(e)
	}
	if res, e := call(t, "verify", "--vault", v, "--base", pushed, "--allow-no-change"); e != nil || res["ok"] != true {
		t.Fatalf("a review of the whole range covers it: %v %v", e, res)
	}
}

func TestBaseTreeReportsAnInvalidRevision(t *testing.T) {
	v := vault(t)
	dir, cleanup, err := baseTree(v, "missing-revision")
	if err == nil || !strings.Contains(err.Error(), "git archive:") || !strings.Contains(err.Error(), "missing-revision") {
		t.Fatalf("an invalid revision must report the archive failure: %v", err)
	}
	if dir != "" || cleanup != nil {
		t.Fatal("a failed snapshot must not expose a partial tree")
	}
}

func TestExtractTreeRejectsEscapingArchivePaths(t *testing.T) {
	for _, link := range []bool{false, true} {
		t.Run(fmt.Sprint(link), func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "tree")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(base, "archive.tar")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			archive := tar.NewWriter(file)
			name := "../escaped"
			if link {
				if err := archive.WriteHeader(&tar.Header{Name: "outside", Typeflag: tar.TypeSymlink, Linkname: ".."}); err != nil {
					t.Fatal(err)
				}
				name = "outside/escaped"
			}
			if err := archive.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}); err != nil {
				t.Fatal(err)
			}
			if _, err := archive.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if err := extractTree(path, dir); err == nil {
				t.Fatal("archive extraction must reject writing outside its tree")
			}
			if _, err := os.Stat(filepath.Join(base, "escaped")); !os.IsNotExist(err) {
				t.Fatalf("archive escaped its tree: %v", err)
			}
		})
	}
}
