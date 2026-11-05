package gitsync

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func withOrigin(t *testing.T, v string) string {
	t.Helper()
	origin := t.TempDir()
	run(t, origin, "init", "-q", "--bare", "-b", "main")
	run(t, v, "remote", "add", "origin", origin)
	run(t, v, "push", "-q", "origin", "main")
	return origin
}

func TestPreparePRLeavesCheckoutIntact(t *testing.T) {
	v := vault(t)
	const firstCase = "investigations/case-one/investigation.md"
	const secondCase = "investigations/case-two/investigation.md"
	const untracked = "investigations/case-two/artifacts/proof.txt"
	const note = "10-Sistemas/Sales.md"
	for _, path := range []string{firstCase, secondCase} {
		write(t, v, path, "# Published case\n")
	}
	run(t, v, "add", "--", firstCase, secondCase)
	run(t, v, "commit", "-qm", "publish cases")
	origin := withOrigin(t, v)
	published, _ := git(v, "rev-parse", "refs/remotes/origin/main")

	// Local main is ahead. The PR must exclude its commits as well as the live index and drafts.
	write(t, v, "90-Meta/local-only.md", "local commit\n")
	run(t, v, "add", "--", "90-Meta/local-only.md")
	run(t, v, "commit", "-qm", "local only")
	write(t, v, firstCase, "staged case draft\n")
	run(t, v, "add", "--", firstCase)
	write(t, v, secondCase, "unstaged case draft\n")
	write(t, v, untracked, "untracked case artifact\n")

	snapshot := func() map[string]string {
		state := map[string]string{}
		for _, args := range [][]string{{"symbolic-ref", "HEAD"}, {"rev-parse", "main"}, {"--no-optional-locks", "status", "--porcelain=v1", "-z", "-uall"}} {
			value, err := gitOutput(v, args...)
			if err != nil {
				t.Fatal(err)
			}
			state[strings.Join(args, " ")] = value
		}
		for _, path := range []string{firstCase, secondCase, untracked, note, ".git/index"} {
			b, err := os.ReadFile(filepath.Join(v, path))
			if err != nil {
				t.Fatal(err)
			}
			state[path] = string(b)
		}
		return state
	}
	before := snapshot()
	named := func(command string, extra ...string) (map[string]any, error) {
		return call(t, append([]string{command, "--vault", v, "--name", "sales"}, extra...)...)
	}
	must := func(command string, extra ...string) map[string]any {
		t.Helper()
		res, err := named(command, extra...)
		if err != nil {
			t.Fatalf("%s: %v (%v)", command, err, res)
		}
		return res
	}

	res := must("start")
	if res["base_commit"] != published {
		t.Fatalf("wrong published base: %v", res)
	}
	draft := res["draft"].(string)
	if _, err := os.Stat(filepath.Join(draft, "90-Meta/local-only.md")); !os.IsNotExist(err) {
		t.Fatalf("draft must not include local commits: %v", err)
	}
	write(t, draft, note, "---\ntipo: sistema\n---\n# Sales\n\nSistema de ventas y cambios.\n")
	write(t, draft, secondCase, "unselected draft change\n")
	if _, err := named("commit", "--path", "../Sales.md", "--message", "docs: outside scope"); err == nil {
		t.Fatal("path traversal accepted")
	}
	head := must("commit", "--path", note, "--message", "docs: update sales")["head"].(string)
	if _, err := named("review", "--head", published, "--verdict", "accept", "--reviewer", "tester"); err == nil {
		t.Fatal("stale review accepted")
	}
	if _, err := named("finish"); err == nil {
		t.Fatal("unreviewed PR marked ready")
	}
	head = must("review", "--head", head, "--verdict", "accept", "--reviewer", "tester")["head"].(string)
	if res := must("verify"); res["ok"] != true {
		t.Fatalf("candidate did not pass verify: %v", res)
	}
	finished := must("finish")
	if finished["head"] != head || finished["ready_for_pr"] != true || finished["push_refspec"] != head+":refs/heads/sync/sales" {
		t.Fatalf("finish did not identify the validated commit: %v", finished)
	}
	paths, err := diffPaths(v, published, head)
	if err != nil || len(paths) != 2 || paths[0] != note || !strings.HasPrefix(paths[1], reviewsDir+"/") {
		t.Fatalf("wrong PR diff: %v, %v", paths, err)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("preparing the PR changed the checkout or index")
	}
	// A push of the reported refspec transfers that exact candidate to a test remote.
	run(t, v, "push", "-q", "origin", finished["push_refspec"].(string))
	remote, _ := git(v, "ls-remote", "origin", "refs/heads/sync/sales")
	if !strings.HasPrefix(remote, head+"\t") {
		t.Fatalf("wrong remote branch: %s", remote)
	}

	otherWriter := t.TempDir()
	run(t, otherWriter, "clone", "-q", origin, ".")
	write(t, otherWriter, secondCase, "new published evidence from another case\n")
	run(t, otherWriter, "commit", "-qam", "docs: publish unrelated case evidence")
	run(t, otherWriter, "push", "-q", "origin", "main")
	upstreamHead, _ := git(otherWriter, "rev-parse", "HEAD")
	if res := must("finish"); res["verified_against"] != upstreamHead || res["head"] != head {
		t.Fatalf("unrelated remote edits must revalidate the same candidate: %v", res)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("checking a newer upstream changed the live checkout")
	}

	write(t, v, note, "edit to the same live note\n")
	if _, err := named("finish"); err == nil || !strings.Contains(err.Error(), note) {
		t.Fatalf("overlapping edit must be reported: %v", err)
	}
	write(t, v, note, before[note])
	write(t, otherWriter, note, "---\ntipo: sistema\n---\n# Sales\n\nNueva edición publicada.\n")
	run(t, otherWriter, "commit", "-qam", "docs: update same note upstream")
	run(t, otherWriter, "push", "-q", "origin", "main")
	if _, err := named("finish"); err == nil || !strings.Contains(err.Error(), "changed PR path "+note) {
		t.Fatalf("upstream overlap must be reported: %v", err)
	}

}

func TestPreparePublishedCaseImportsOnlyItsPendingFiles(t *testing.T) {
	v := vault(t)
	id := localPublishedCase(t, v)
	withOrigin(t, v)
	const other = "investigations/other/artifacts/new.txt"
	write(t, v, other, "do not import\n")
	run(t, v, "add", "--", other)
	before, _ := os.ReadFile(filepath.Join(v, ".git", "index"))

	res, err := call(t, "start", "--vault", v, "--name", "case-edit", "--case", id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(res["draft"].(string), other)); !os.IsNotExist(err) {
		t.Fatal("draft contains another case's staged files")
	}
	args := []string{"commit", "--vault", v, "--name", "case-edit", "--message", "docs: update case"}
	for _, p := range toStrings(res["imported"]) {
		args = append(args, "--path", p)
	}
	res, err = call(t, args...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, "review", "--vault", v, "--name", "case-edit", "--head", res["head"].(string), "--verdict", "accept", "--reviewer", "tester"); err != nil {
		t.Fatal(err)
	}
	if res, err := call(t, "finish", "--vault", v, "--name", "case-edit"); err != nil || res["ready_for_pr"] != true {
		t.Fatalf("selected case could not finish: %v %v", res, err)
	}
	after, _ := os.ReadFile(filepath.Join(v, ".git", "index"))
	if current(v) != "main" || !bytes.Equal(before, after) {
		t.Fatal("case PR changed the live checkout or index")
	}
}
