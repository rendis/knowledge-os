package investigation

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

const sampleID = "20260924-120000-native-case"

func makeCase(t *testing.T, root, visibility string) string {
	t.Helper()
	store := root
	if visibility == "unpublished" {
		store = filepath.Join(filepath.Dir(root), ".investigations")
	}
	dir := filepath.Join(store, sampleID)
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	s := "---\n"
	for _, k := range requiredFields {
		v := "sample"
		switch k {
		case "id":
			v = sampleID
		case "dedupe-key":
			v = "native-case"
		case "status":
			v = "investigating"
		case "created-at", "updated-at":
			v = "2026-09-24T12:00:00Z"
		case "purpose":
			v = "knowledge"
		case "vault-outcome", "learning-outcome":
			v = "not-evaluated"
		}
		s += k + ": " + v + "\n"
	}
	s += "---\n"
	for _, a := range requiredSections {
		s += "\n## " + a[0] + "\n\nExample content.\n"
	}
	p := filepath.Join(dir, "investigation.md")
	if e := os.WriteFile(p, []byte(s), 0644); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestReadParityWithExistingTool(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("Python reference unavailable")
	}
	script, e := filepath.Abs("../../kernel/.agents/skills/manage-investigation/scripts/investigation-case.py")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(script); e != nil {
		t.Skip("reference script unavailable")
	}
	for _, visibility := range []string{"published", "unpublished"} {
		t.Run(visibility, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "investigations")
			if e := os.MkdirAll(root, 0755); e != nil {
				t.Fatal(e)
			}
			makeCase(t, root, visibility)
			for _, verb := range []string{"list", "load"} {
				args := []string{"--root", root, verb}
				if verb == "load" {
					args = append(args, "--id", sampleID)
				}
				expected, e := exec.Command(python, append([]string{"-B", script}, args...)...).Output()
				if e != nil {
					t.Fatalf("Python reference %s: %v", verb, e)
				}
				var out bytes.Buffer
				if e = Run(args, &out); e != nil {
					t.Fatal(e)
				}
				var a, b any
				if e = json.Unmarshal(expected, &a); e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(out.Bytes(), &b); e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(a, b) {
					t.Fatalf("%s parity mismatch\nPython=%s\nGo=%s", verb, expected, &out)
				}
			}
		})
	}
}
func TestLoadRejectsUnsafeAndPrivateAuthority(t *testing.T) {
	root := filepath.Join(t.TempDir(), "investigations")
	if e := os.MkdirAll(root, 0755); e != nil {
		t.Fatal(e)
	}
	p := makeCase(t, root, "published")
	old, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p, append(old, []byte("\npassword: example-value\n")...), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = loadCase(root, sampleID); e == nil {
		t.Fatal("credential-bearing case accepted")
	}
	if e = os.WriteFile(p, old, 0644); e != nil {
		t.Fatal(e)
	}
	priv := filepath.Join(filepath.Dir(root), ".investigations-private", sampleID)
	if e = os.MkdirAll(priv, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(priv, "private.md"), []byte("---\nid: wrong\nauthority: private-overlay\nupdated-at: 2026-09-24T12:00:00Z\n---\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = loadCase(root, sampleID); e == nil {
		t.Fatal("wrong private identity accepted")
	}
}
func TestVisibilityConflictAndMutationGate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "investigations")
	if e := os.MkdirAll(root, 0755); e != nil {
		t.Fatal(e)
	}
	makeCase(t, root, "published")
	makeCase(t, root, "unpublished")
	if _, _, e := records(root); e == nil {
		t.Fatal("visibility conflict accepted")
	}
	gate := filepath.Join(filepath.Dir(root), ".investigations", ".open.lock")
	if e := os.Mkdir(gate, 0700); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	e := Run([]string{"list", "--root", root}, &out)
	if e == nil || !strings.Contains(e.Error(), "lock") {
		t.Fatalf("lock not detected: %v", e)
	}
}

func TestLegacyWorkbenchDoesNotPolluteReadFingerprint(t *testing.T) {
	root := filepath.Join(t.TempDir(), "investigations")
	if e := os.MkdirAll(root, 0755); e != nil {
		t.Fatal(e)
	}
	makeCase(t, root, "published")
	legacy := filepath.Join(filepath.Dir(root), ".investigations", "legacy-workbench")
	if e := os.MkdirAll(legacy, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(legacy, "investigation.md"), []byte("# Historical investigation\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(t.TempDir(), filepath.Join(legacy, "node_modules")); e != nil {
		t.Skipf("symlinks unavailable: %v", e)
	}
	var out bytes.Buffer
	if e := Run([]string{"list", "--root", root}, &out); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out.Bytes(), []byte("legacy-workbench")) {
		t.Fatal(out.String())
	}
}
