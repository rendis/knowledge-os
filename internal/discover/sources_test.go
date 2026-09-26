package discover

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourcesStateAndLines(t *testing.T) {
	dir := t.TempDir()
	repos := filepath.Join(dir, "repos")
	repo := gitRepo(t, filepath.Join(repos, "SVC-orders"), map[string]string{"svc/save.go": "package svc\n// one\n// two\n// three\nfunc Save() {}\n", "svc/other.go": "package svc\n"})
	old, _ := resolveCommit(repo, "HEAD")
	write(t, repo, "svc/save.go", "package svc\n// one\n// two\n// three\nfunc Save(x int) {}\n")
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "change"}} {
		if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
			t.Fatal(string(b))
		}
	}
	vault := t.TempDir()
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\nsources:\n  repo_prefixes: [\"SVC\"]\n")
	write(t, vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+repos+"\"\n")
	s := NewSources(vault)
	at := func(path string, from, to int) Anchor {
		return Anchor{Repo: "acme/SVC-orders", Commit: old[:12], Path: path, From: from, To: to}
	}
	for _, c := range []struct {
		a    Anchor
		want string
	}{
		{at("svc/save.go", 2, 3), "current"}, // the file changed below the cited lines
		{at("svc/save.go", 5, 5), "changed"},
		{at("svc/save.go", 0, 0), "changed"}, // a whole-file citation sees any change
		{at("svc/other.go", 1, 1), "current"},
		{Anchor{Repo: "acme/SVC-missing", Commit: old, Path: "x.go"}, "unknown"},
	} {
		if got := s.State(c.a); got.Status != c.want {
			t.Errorf("%+v = %+v, want %s", c.a, got, c.want)
		}
	}
	if got, label, ok := s.Excerpt(at("svc/save.go", 4, 5), nil, nil, 12); !ok || got != "   4│ // three\n   5│ func Save() {}" || label != "svc/save.go#L4" {
		t.Fatalf("lines at the cited commit = %q %q, %v", got, label, ok)
	}
	if got, _, _ := s.Excerpt(at("svc/save.go", 2, 5), nil, nil, 2); got != "   2│ // one\n   3│ // two" {
		t.Fatalf("lines are capped: %q", got)
	}
	if got, _, _ := s.Excerpt(at("svc/save.go", 1, 5), nil, []string{"Save"}, 2); got != "   1│ package svc\n    │ …\n   4│ // three\n   5│ func Save() {}" {
		t.Fatalf("a long range shows the literal the claim is about: %q", got)
	}
	if got, label, ok := s.Excerpt(at("svc/save.go", 0, 0), []string{"Save"}, nil, 12); !ok || label != "svc/save.go#L5 · Save" || !strings.HasPrefix(got, "   5│ func Save() {}") {
		t.Fatalf("a whole-file link shows the symbol it names: %q %q", got, label)
	}
	if got := Anchors("[^e1]: [a](https://github.com/acme/SVC-orders/blob/" + old + "/svc/save.go#L2-L3) y [b](https://github.com/acme/SVC-orders/blob/" + old + "/svc/other.go)"); len(got) != 2 || got[0].From != 2 || got[0].To != 3 || got[1].From != 0 {
		t.Fatalf("anchors %+v", got)
	}
}

func TestSourcesGrepAndSymbol(t *testing.T) {
	dir := t.TempDir()
	repos := filepath.Join(dir, "repos")
	gitRepo(t, filepath.Join(repos, "SVC-orders"), map[string]string{
		"a/config.go":         "package a\n\nvar MAX_RETRIES = env(\"MAX_RETRIES\")\n",
		"a/send.go":           "package a\n\n// Send posts.\nfunc (s *S) Send() {\n\tfor i := 0; i < MAX_RETRIES; i++ {\n\t}\n}\n",
		"a/send_test.go":      "package a\n// MAX_RETRIES in a test\n",
		"a/main.go":           "package a\n\nfunc main() { (&S{}).Send() }\n",
		"k8s/production/env":  "MAX_RETRIES=6\n",
		"k8s/development/env": "MAX_RETRIES=2\n",
	})
	vault := t.TempDir()
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\nsources:\n  repo_prefixes: [\"SVC\"]\n")
	write(t, vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+repos+"\"\n")
	s := NewSources(vault)
	hits, all, ref := s.Grep("acme/SVC-orders", []string{"MAX_RETRIES", "Missing", "for"}, 3)
	if len(hits["for"]) != 1 || hits["for"][0].Path != "a/send.go" {
		t.Fatalf("a line naming two searched names counts for both: %v", hits["for"])
	}
	if len(all["MAX_RETRIES"]) != 4 {
		t.Fatalf("all hits %v", all)
	}
	if ref != "main" || len(hits["Missing"]) != 0 {
		t.Fatalf("ref %q, hits %v", ref, hits)
	}
	got := []string{}
	for _, h := range hits["MAX_RETRIES"] {
		got = append(got, h.Path)
	}
	// Code first, one line per file, production configuration before development; tests left out.
	if strings.Join(got, ",") != "a/config.go,a/send.go,k8s/production/env" {
		t.Fatalf("grep order %v", got)
	}
	fns, _ := s.Functions("acme/SVC-orders", []string{"retr", "for i"}, 1, 5, nil)
	if len(fns) != 1 || fns[0].Path != "a/send.go" || fns[0].From != 4 || !strings.HasPrefix(fns[0].Code, "   4│ func (s *S) Send() {") || strings.Join(fns[0].Words, ",") != "for i,retr" || len(fns[0].Callers) != 1 || fns[0].Callers[0].Path != "a/main.go" {
		t.Fatalf("functions %+v", fns)
	}
	head, _ := resolveCommit(filepath.Join(repos, "SVC-orders"), "HEAD")
	code, line, name, ok := s.Symbol(Anchor{Repo: "acme/SVC-orders", Commit: head, Path: "a/send.go"}, []string{"Nope", "Send"}, 2)
	if !ok || line != 4 || name != "Send" || code != "func (s *S) Send() {\n\tfor i := 0; i < MAX_RETRIES; i++ {" {
		t.Fatalf("symbol %q %d %q %v", code, line, name, ok)
	}
}

func TestTopicWiring(t *testing.T) {
	vault := t.TempDir()
	write(t, vault, "90-Meta/discovery/platform/gcp-p.json", `{"provider":"gcp","scope":"p","status":"ok","captured_at":"2026-09-25","topics":["projects/p/topics/a.b.orders-acked","projects/p/topics/a.x.b.orders-acked","projects/p/topics/other"],"subscriptions":[{"name":"projects/p/subscriptions/orders-acked-sub","topic":"projects/p/topics/a.b.orders-acked","filter":"attributes.c=\"CL\""}]}`)
	write(t, vault, stateRel+"/facts/SVC-pub.json", `{"repo":"SVC-pub","resources":[{"type":"message_topic","name":"a.b.orders-acked","direction":"publish","evidence":[{"file":"k8s/env","key":"TOPIC"}]}]}`)
	write(t, vault, stateRel+"/facts/SVC-sub.json", `{"repo":"SVC-sub","resources":[{"type":"message_subscription","name":"orders-acked-sub","direction":"consume","evidence":[{"file":"k8s/env","key":"SUB"}]}]}`)
	w, ok := TopicWiring(vault, "a.b.orders-acked")
	if !ok || len(w.Repos) != 2 || len(w.Subscriptions) != 1 || strings.Join(w.Subscriptions[0].ConfiguredBy, ",") != "SVC-sub" || strings.Join(w.Similar, ",") != "a.x.b.orders-acked" {
		t.Fatalf("wiring %+v", w)
	}
	if _, ok := TopicWiring(vault, "nobody.knows"); ok {
		t.Fatal("an unknown topic has no wiring")
	}
}

func TestClaimsChecksBareNames(t *testing.T) {
	vault := t.TempDir()
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\n")
	write(t, vault, "25-Topics/acme.sales.orders-acked.md", "# acme.sales.orders-acked\n")
	write(t, vault, stateRel+"/facts/SVC-x.json", `{"repo":"SVC-x"}`)
	write(t, vault, stateRel+"/comparison.json", `[]`)
	res, e := checkClaims(vault, "El servicio publica en acme.sales.orders-acked y en acme.sales.inventado-topic; es fire-and-forget.")
	if e != nil {
		t.Fatal(e)
	}
	if res["names_checked"] != 2 || res["ok"] != false {
		t.Fatalf("bare names: %v", res)
	}
	res, _ = checkClaims(vault, "Solo prosa.")
	if res["note"] == nil {
		t.Fatalf("nothing checked must say so: %v", res)
	}
}

func TestRedactHidesCredentials(t *testing.T) {
	for in, want := range map[string]string{
		"ACCESS_TOKEN_SECRET=s3cr3tValue99":            "ACCESS_TOKEN_SECRET=‹redacted›",
		"  password: \"hunter2hunter2\"":               "  password: ‹redacted›",
		"url=https://x.blob.core/c?sv=1&sig=abcdef123": "url=‹redacted›",
		"MAX_RETRIES=6":                                "MAX_RETRIES=6",
		"TOKEN_URL=https://auth/validate":              "TOKEN_URL=https://auth/validate",
	} {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFunctionStartsAcrossLanguages(t *testing.T) {
	for line, want := range map[string]string{
		"func (s *S) Send() {":                              "Send",
		"  async saveData(input: Order): Promise<number> {": "saveData",
		"  async saveDataAcme (":                       "saveDataAcme",
		"  public async close(): Promise<void> {":           "close",
		"def run(self):":                                    "run",
		"export const handler = async (event) => {":         "handler",
		"  if (x) {":               "",
		"  } catch (e) {":          "",
		"    for (const a of b) {": "",
	} {
		if got := definedName(line); got != want || isFuncStart(line) != (want != "") {
			t.Errorf("%q: name %q start %v, want %q", line, got, isFuncStart(line), want)
		}
	}
}
