package retrieval

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func codeFixture(t *testing.T) (Options, string) {
	t.Helper()
	opt, write := fixture(t)
	repos := t.TempDir()
	repo := filepath.Join(repos, "SVC-orders")
	files := map[string]string{
		"svc/send.go":      "package svc\n\n// Send posts with retries.\nfunc Send(n int) error {\n\tfor i := 0; i < n; i++ {\n\t}\n\treturn nil\n}\n",
		"svc/page.go":      "package svc\n\nfunc Page() {\n\tif err := Send(3); err != nil {\n\t\tlog(err)\n\t}\n\trun(Send)\n}\n",
		"svc/send_test.go": "package svc\n// Send in a test\n",
		"k8s/prod/env":     "RETRIES=6\n",
		"svc/const.go":     "package svc\n\nconst Limit = 3\n\nfunc Use() int { return Limit }\n",
	}
	for p, c := range files {
		full := filepath.Join(repo, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if e := os.WriteFile(full, []byte(c), 0o644); e != nil {
			t.Fatal(e)
		}
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "init"}} {
		if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
			t.Fatalf("git %v: %s", args, b)
		}
	}
	write("instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\nsources:\n  repo_prefixes: [\"SVC\"]\n")
	write(".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+repos+"\"\n")
	write("20-Repos/orders.md", "---\naliases: [\"SVC-orders\"]\n---\n# orders\n\nSends.\n")
	return opt, repo
}

func runCode(t *testing.T, opt Options, o CodeOptions) (string, error) {
	t.Helper()
	ctx := context.Background()
	idx, e := Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	defer idx.Close()
	var b bytes.Buffer
	o.Budget = ReadBudget
	e = idx.Code(ctx, o, &b)
	return b.String(), e
}

func TestCodeFuncGrepAndShow(t *testing.T) {
	opt, _ := codeFixture(t)
	out, e := runCode(t, opt, CodeOptions{Repo: "orders", Func: "Send"})
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"# Send in SVC-orders at main", "## svc/send.go#L4-L8", "   4│ func Send(n int) error {", "svc/page.go:4 in Page", "   5│ \t\tlog(err)", "svc/page.go:7 in Page"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "send_test.go") {
		t.Fatalf("tests are not callers:\n%s", out)
	}
	out, _ = runCode(t, opt, CodeOptions{Repo: "SVC-orders", Grep: "retr|RETRIES", IgnoreCase: true})
	for _, want := range []string{"2 lines in 2 files", "svc/send.go\n  L3: // Send posts with retries.", "k8s/prod/env\n  L1: RETRIES=6"} {
		if !strings.Contains(out, want) {
			t.Fatalf("grep: missing %q in\n%s", want, out)
		}
	}
	out, _ = runCode(t, opt, CodeOptions{Repo: "orders", Grep: "renewToken"})
	if !strings.Contains(out, "No line matches in: SVC-orders at main (") || !strings.Contains(out, "tests left out. An absence covers that pattern, branch and those files only") {
		t.Fatalf("an absence states its scope:\n%s", out)
	}
	out, _ = runCode(t, opt, CodeOptions{Repo: "orders", Show: "svc/page.go:3-4"})
	if !strings.Contains(out, "   3│ func Page() {\n   4│ \tif err := Send(3); err != nil {") || strings.Contains(out, "   5│") {
		t.Fatalf("show a range:\n%s", out)
	}
	out, _ = runCode(t, opt, CodeOptions{Repo: "orders", Func: "Limit"})
	if !strings.Contains(out, "not a function") || !strings.Contains(out, "Declared at svc/const.go:3") || !strings.Contains(out, "svc/const.go (in Use):5") {
		t.Fatalf("a constant gets its declaration and uses:\n%s", out)
	}
	if _, e = runCode(t, opt, CodeOptions{Repo: "nope", Func: "X"}); e == nil || !strings.Contains(e.Error(), "repositories with a checkout: SVC-orders") {
		t.Fatalf("an unknown repository lists the known ones: %v", e)
	}
}
