package gitsync

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
