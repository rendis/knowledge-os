package retrieval

import (
	"bytes"
	"context"
	"knowledge-os/internal/discover"
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
		"svc/routes.go":    "package svc\n\nfunc Auth(h int) int { return h }\n\nfunc Routes() {\n\troute(\"/a\", Auth, 1)\n\troute(\"/b\", 2)\n}\n",
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
	for _, want := range []string{"# Send in SVC-orders at main", "## svc/send.go#L4-L8", "   4│ func Send(n int) error {", "svc/page.go:4 in Page", "   5│ \t\tlog(err)", "svc/page.go:7 in Page", "Exits of Send (the whole function, L4-L8 at main):\n- L7 returns nil"} {
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
	out, _ = runCode(t, opt, CodeOptions{Repo: "orders", Show: "svc/page.go:3,7-8"})
	if !strings.Contains(out, "   3│ func Page() {\n```") || !strings.Contains(out, "   7│ \trun(Send)\n   8│ }") || strings.Contains(out, "   4│") {
		t.Fatalf("show several ranges:\n%s", out)
	}
	out, _ = runCode(t, opt, CodeOptions{Repo: "orders", Func: "Auth"})
	if !strings.Contains(out, "Same-shaped lines in svc/routes.go that do not pass Auth (route(), 1:\n- L7 `route(\"/b\", 2)`") {
		t.Fatalf("the lines a passed function is missing from:\n%s", out)
	}
	if _, e = runCode(t, opt, CodeOptions{Repo: "orders", Show: "svc/page.go:9-3"}); e == nil {
		t.Fatal("a reversed range is an error, not a narrower read")
	}
	out, _ = runCode(t, opt, CodeOptions{Repo: "orders", Func: "Page,Send", Down: true})
	if !strings.Contains(out, "# Page in SVC-orders") || !strings.Contains(out, "# Send in SVC-orders") || !strings.Contains(out, "Calls down from Page to this repository's functions") || !strings.Contains(out, "L4 Send → svc/send.go#L4-L8\n- L7 returns nil") {
		t.Fatalf("several functions, with the calls below:\n%s", out)
	}
	out, _ = runCode(t, opt, CodeOptions{Repo: "orders", Grep: `Send\(\d\)`})
	if discover.GrepDialect() == "Perl-compatible" && !strings.Contains(out, "svc/page.go\n  L4") {
		t.Fatalf("\\d works where git has PCRE:\n%s", out)
	}
	if _, e = runCode(t, opt, CodeOptions{Repo: "nope", Func: "X"}); e == nil || !strings.Contains(e.Error(), "repositories with a checkout: SVC-orders") {
		t.Fatalf("an unknown repository lists the known ones: %v", e)
	}
}

func TestExitLinesCheckTheNote(t *testing.T) {
	f := discover.Function{Path: "src/h.ts", Name: "process = async (id: string): Promise<boolean> => {", Start: 2, End: 30, Exits: []discover.Exit{
		{Line: 10, Kind: "returns true", When: "if (duplicateResult === Result.DUPLICATE) {", At: 9, Names: []string{"duplicateResult", "DUPLICATE"}},
		{Line: 14, Kind: "returns true", When: "} catch (e) {", At: 12, Names: []string{"Error parse payload"}},
		{Line: 20, Kind: "returns false", When: "} catch (e) {", At: 18, Names: []string{"Error save data", "rollback"}},
		{Line: 22, Kind: "propagates", Text: "throw e"},
	}, Callers: []discover.Hit{{Path: "src/l.ts", Line: 5, Text: "await h.process(m.id) ? m.ack() : m.nack()"}}}
	note := "---\n---\n# h\n\nSi el resultado es duplicado se confirma con ACK sin guardar.\n\n[^e1]: [src/h.ts](https://github.com/o/r/blob/0123456789abcdef0123456789abcdef01234567/src/h.ts#L12-L16) — parse\n"
	out := exitLines(f, []byte(note), "origin/main", 8)
	for _, want := range []string{
		"L10 returns true — under L9",
		"note L5 (dupli",
		"L14 returns true — under L12 `} catch (e) {` — note L7 cites L12-L16",
		"L20 returns false — under L18 `} catch (e) {` — ✗ (save, rollb)",
		"L22 pass on the error",
		"its result at src/l.ts:5: true → `m.ack()`, false → `m.nack()`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(exitLines(f, nil, "origin/main", 8), "✗") {
		t.Fatal("without a note there is nothing to mark")
	}
}

func TestCodeFollowsAURLSettingToItsRoute(t *testing.T) {
	opt, write := fixture(t)
	repos := t.TempDir()
	for name, files := range map[string]map[string]string{
		"SVC-front": {
			"src/check.ts": "export function check(t: string) {\n  return post(process.env.AUTH_URL, t)\n}\n",
			"k8s/prod/env": "AUTH_URL=https://gw.example.com/svc-auth/v1/auth/validate\n",
		},
		"SVC-auth": {
			"src/controllers/auth/router.ts": "router\n  .post('/validate', validate)\n",
			"src/client.ts":                  "const url = `${base}/v1/auth/validate`\n",
		},
	} {
		repo := filepath.Join(repos, name)
		for p, c := range files {
			_ = os.MkdirAll(filepath.Dir(filepath.Join(repo, p)), 0o755)
			if e := os.WriteFile(filepath.Join(repo, p), []byte(c), 0o644); e != nil {
				t.Fatal(e)
			}
		}
		for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "init"}} {
			if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
				t.Fatalf("git %v: %s", args, b)
			}
		}
	}
	write("instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\nsources:\n  repo_prefixes: [\"SVC\"]\n")
	write(".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+repos+"\"\n")
	out, err := runCode(t, opt, CodeOptions{Repo: "SVC-front", Func: "check"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "`AUTH_URL` calls …/auth/validate, declared in: SVC-auth src/controllers/auth/router.ts:2") || strings.Contains(out, "client.ts") {
		t.Fatalf("a URL setting is followed to the route that declares it:\n%s", out)
	}
}
