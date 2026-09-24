package investigation

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func bindFixture(t *testing.T) (string, string, string, options, map[string]string) {
	t.Helper()
	root := gitFixture(t)
	openFixture(t, root)
	p := filepath.Join(filepath.Dir(root), ".investigations", sampleID, "investigation.md")
	story := filepath.Join(filepath.Dir(p), "exports", "S-001.md")
	if e := os.MkdirAll(filepath.Dir(story), 0755); e != nil {
		t.Fatal(e)
	}
	b := []byte("---\nstory-id: S-001\nsource-investigation: " + sampleID + "\n---\n\n# Reviewed story\n")
	if e := os.WriteFile(story, b, 0644); e != nil {
		t.Fatal(e)
	}
	_, entry := dhFixture()
	observation := dhBinding(entry)
	delete(observation, "dh")
	observed := filepath.Join(t.TempDir(), "observation.json")
	raw, _ := json.Marshal(observation)
	if e := os.WriteFile(observed, raw, 0600); e != nil {
		t.Fatal(e)
	}
	original, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	o := options{"id": {sampleID}, "observation": {observed}, "expected-public-sha256": {digest(original)}, "expected-story-sha256": {digest(b)}}
	return root, p, story, o, observation
}
func TestBindCASRetryAndRevision(t *testing.T) {
	root, p, story, o, observation := bindFixture(t)
	original, _ := os.ReadFile(p)
	v, e := bindCase(root, o)
	if e != nil {
		t.Fatal(e)
	}
	if v.(map[string]any)["status"] != "bound" {
		t.Fatal(v)
	}
	bound, _ := os.ReadFile(p)
	if _, e = bindCase(root, o); e == nil || !strings.Contains(e.Error(), "stale_public_snapshot") {
		t.Fatalf("stale retry %v", e)
	}
	o["expected-public-sha256"] = []string{digest(bound)}
	v, e = bindCase(root, o)
	if e != nil || v.(map[string]any)["status"] != "unchanged" {
		t.Fatalf("exact retry %v %v", v, e)
	}
	again, _ := os.ReadFile(p)
	if !bytes.Equal(bound, again) {
		t.Fatal("retry changed document")
	}
	observation["revision"] = "v0002"
	observation["materialized-at"] = "2026-09-25T12:00:00-03:00"
	raw, _ := json.Marshal(observation)
	os.WriteFile(o.get("observation"), raw, 0600)
	if _, e = bindCase(root, o); e != nil {
		t.Fatal(e)
	}
	revised, _ := os.ReadFile(p)
	if len(validateHandoffs(string(revised))) != 0 || strings.Count(string(revised), dhMarker) != 2 {
		t.Fatal("invalid revision history")
	}
	if !bytes.Contains(revised, []byte("revision `v0001`")) {
		t.Fatal("old history removed")
	}
	o["expected-public-sha256"] = []string{digest(revised)}
	os.WriteFile(story, []byte("changed"), 0644)
	if _, e = bindCase(root, o); e == nil {
		t.Fatal("stale story accepted")
	}
	final, _ := os.ReadFile(p)
	if !bytes.Equal(final, revised) || bytes.Equal(final, original) {
		t.Fatal("failed operation changed bytes")
	}
}
func TestBindRejectsIdentityAndInvalidObservation(t *testing.T) {
	root, p, _, o, m := bindFixture(t)
	if _, e := bindCase(root, o); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(p)
	o["expected-public-sha256"] = []string{digest(before)}
	m["branch"] = "feature/new"
	m["revision"] = "v0002"
	b, _ := json.Marshal(m)
	os.WriteFile(o.get("observation"), b, 0600)
	if _, e := bindCase(root, o); e == nil || !strings.Contains(e.Error(), "binding_identity_mismatch") {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("identity failure wrote")
	}
	os.WriteFile(o.get("observation"), []byte(strings.Replace(string(b), `{"branch":`, `{"branch":"duplicate","branch":`, 1)), 0600)
	if _, e := bindCase(root, o); e == nil || !strings.Contains(e.Error(), "binding_observation_invalid") {
		t.Fatal(e)
	}
}
func TestBindPythonBytesParity(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("Python reference unavailable")
	}
	script, e := filepath.Abs("../../kernel/.agents/skills/manage-investigation/scripts/investigation-case.py")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(script); e != nil {
		t.Skip("reference unavailable")
	}
	root, p, _, o, _ := bindFixture(t)
	original, _ := os.ReadFile(p)
	cmd := exec.Command(python, script, "--root", root, "bind", "--id", sampleID, "--observation", o.get("observation"), "--expected-public-sha256", o.get("expected-public-sha256"), "--expected-story-sha256", o.get("expected-story-sha256"))
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("python %v %s", e, out)
	}
	want, _ := os.ReadFile(p)
	os.WriteFile(p, original, 0644)
	if _, e = bindCase(root, o); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, want) {
		t.Fatalf("Go/Python bind byte mismatch\ngot %s\nwant %s", got, want)
	}
}
