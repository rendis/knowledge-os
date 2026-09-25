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
