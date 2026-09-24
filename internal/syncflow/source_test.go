package syncflow

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sourceRepo(t *testing.T) string {
	t.Helper()
	r := t.TempDir()
	for _, a := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		if _, e := sourceGit(r, nil, a...); e != nil {
			t.Fatal(e)
		}
	}
	return r
}
func sourceCommit(t *testing.T, r string) string {
	t.Helper()
	if _, e := sourceGit(r, nil, "add", "--all"); e != nil {
		t.Fatal(e)
	}
	if _, e := sourceGit(r, nil, "commit", "-qm", "fixture", "--allow-empty"); e != nil {
		t.Fatal(e)
	}
	s, e := sourceResolve(r, "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestSourceGitManifestAndScaffold(t *testing.T) {
	r := sourceRepo(t)
	write(t, filepath.Join(r, "cloudrun/prod/.env.yaml"), "PASSWORD: actual-secret\nNAME: demo\n")
	write(t, filepath.Join(r, "removed.txt"), "old\n")
	old := sourceCommit(t, r)
	write(t, filepath.Join(r, "cloudrun/prod/.env.yaml"), "PASSWORD: ${SECRET}\nNAME: demo\n")
	if e := os.Remove(filepath.Join(r, "removed.txt")); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(r, "binary.dat"), "a\x00b")
	write(t, filepath.Join(r, "kustomization/prod/app.yaml"), "kind: Deployment\n")
	new := sourceCommit(t, r)
	m, e := buildManifest(r, old, new, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = validateSourceManifest(m); e != nil {
		t.Fatal(e)
	}
	if len(arr(m["paths"])) != 4 || len(arr(m["environment_configs"])) != 2 || len(arr(m["credential_suspects"])) != 1 {
		t.Fatal(m)
	}
	if obj(arr(m["paths"])[0])["content_kind"] != "binary" {
		t.Fatal(m)
	}
	s, e := initializeAnalysis(m, "APP00001-example", []string{"other", "example"})
	if e != nil {
		t.Fatal(e)
	}
	if s["version"] != json.Number("3") || len(arr(s["nodes"])) != 2 || len(obj(s["checklist"])) != 6 {
		t.Fatal(s)
	}
	if len(arr(obj(obj(s["checklist"])["deployment"])["evidence"])) != 1 {
		t.Fatal(s)
	}
	var out bytes.Buffer
	if e = runSource([]string{"build", "--repo", r, "--old", old, "--new", new}, &out); e != nil {
		t.Fatal(e)
	}
	v, e := decode(out.Bytes())
	if e != nil || !reflect.DeepEqual(v, m) {
		t.Fatal(e, out.String())
	}
	if _, e = buildManifest(r, new, old, false); e == nil {
		t.Fatal("accepted nonancestor")
	}
	// Differential reference check is optional development verification only;
	// native execution and all other tests do not require Python.
	py, e := exec.LookPath("python3")
	if e == nil {
		script := filepath.Join("..", "..", "kernel", "90-Meta", "git-change-manifest.py")
		if _, e = os.Stat(script); e == nil {
			b, e := exec.Command(py, "-B", script, "build", "--repo", r, "--old", old, "--new", new).Output()
			if e != nil {
				t.Fatal(e)
			}
			v, e = decode(b)
			if e != nil || !reflect.DeepEqual(v, m) {
				t.Fatalf("Python differential mismatch: %s vs %#v (%v)", b, m, e)
			}
			mp := filepath.Join(t.TempDir(), "manifest.json")
			saveJSON(t, mp, m)
			b, e = exec.Command(py, "-B", script, "init-analysis", "--manifest", mp, "--repository", "APP00001-example", "--node", "other").Output()
			if e != nil {
				t.Fatal(e)
			}
			v, e = decode(b)
			if e != nil || !reflect.DeepEqual(v, s) {
				t.Fatalf("scaffold differential mismatch: %s vs %#v (%v)", b, s, e)
			}
		}
	}
}
func TestSourceEmptyNewManifest(t *testing.T) {
	r := sourceRepo(t)
	n := sourceCommit(t, r)
	m, e := buildManifest(r, "", n, true)
	if e != nil {
		t.Fatal(e)
	}
	s, e := initializeAnalysis(m, "APP00001-empty", nil)
	if e != nil {
		t.Fatal(e)
	}
	if s["result"] != "no-change" || len(arr(s["claims"])) != 0 {
		t.Fatal(s)
	}
	for _, v := range obj(s["checklist"]) {
		if obj(v)["status"] != "not-applicable" || len(arr(obj(v)["evidence"])) != 0 {
			t.Fatal(s)
		}
	}
	m["version"] = json.Number("99")
	if _, e = initializeAnalysis(m, "example", nil); e == nil {
		t.Fatal("unsupported manifest version")
	}
}
func TestSourceCredentialScanner(t *testing.T) {
	cases := []struct {
		p, text string
		lines   []int
	}{
		{"settings.yaml", "PASSWORD: actual-value\nPASSWORD: ${SECRET}\nid-token: write\n", []int{1}},
		{"app.py", "password = os.getenv('DB_PASSWORD')\npassword = os.getenv('DB_PASSWORD', 'actual-value')\npassword = ref\n", []int{2}},
		{"deploy.yaml", "env:\n  - name: DB_PASSWORD\n    value: actual-value\n  - name: API_TOKEN\n    valueFrom:\n      secretKeyRef:\n        name: store\n        key: password\n", []int{3}},
		{"password.txt", "# comment\nactual-value\n", []int{2}},
		{"app.js", "const password = process.env.PASSWORD || 'actual-value';\n", []int{1}},
	}
	for _, c := range cases {
		if got := scanCredentialText(c.p, c.text); !reflect.DeepEqual(got, c.lines) {
			t.Fatalf("%s: got %v want %v", c.p, got, c.lines)
		}
	}
	if len(literalValues("'fixture-secret'", false)) != 0 || len(literalValues("{{ secret }}", false)) != 0 {
		t.Fatal("placeholder or reference")
	}
}
func TestSourceNewManifestBinaryAndSpaces(t *testing.T) {
	r := sourceRepo(t)
	write(t, filepath.Join(r, "note with space.txt"), "hello\n")
	write(t, filepath.Join(r, "bad.dat"), "\xff\n")
	n := sourceCommit(t, r)
	m, e := buildManifest(r, "", n, true)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(str(m["old_oid"]), "4b825d") || obj(arr(m["paths"])[0])["content_kind"] != "binary" {
		t.Fatal(m)
	}
	for _, v := range arr(m["paths"]) {
		if obj(v)["status"] != "added" {
			t.Fatal(m)
		}
	}
}
