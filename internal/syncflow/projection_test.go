package syncflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projectionFixture(t *testing.T, patch []byte) (map[string]any, map[string]any) {
	t.Helper()
	g := fixture(t, "gate.json")
	d, _ := digest(g)
	s, e := patchSections(patch)
	if e != nil {
		t.Fatal(e)
	}
	base, result := map[string]any{}, map[string]any{}
	for p, x := range s {
		base[p] = hash(x.Old)
		result[p] = hash(x.New)
	}
	return g, map[string]any{"version": json.Number("1"), "run_id": "run-test", "gate_digest": d, "unit_id": "group-001", "unit_type": "write-group", "patch_digest": hash(patch), "base_files": base, "result_files": result, "grants": obj(arr(g["write_groups"])[0])["grants"]}
}
func TestProjectionFullImageBinding(t *testing.T) {
	patch := []byte("--- a/20-Repos/target-service.md\n+++ b/20-Repos/target-service.md\n@@ -1 +1 @@\n-old\n+new\n")
	g, p := projectionFixture(t, patch)
	if e := validateProjection(g, p, patch); e != nil {
		t.Fatal(e)
	}
	obj(p["result_files"])["20-Repos/target-service.md"] = hash([]byte("tampered"))
	if validateProjection(g, p, patch) == nil {
		t.Fatal("accepted wrong byte digest")
	}
	for _, mutation := range []string{strings.ReplaceAll(string(patch), "target-service", "unknown"), strings.ReplaceAll(string(patch), "+new\n", "+---\n")} {
		g, p = projectionFixture(t, []byte(mutation))
		if strings.Contains(mutation, "unknown") && validateProjection(g, p, []byte(mutation)) == nil {
			t.Fatal("unauthorized node")
		}
	}
}
func TestProjectionRejectsIncompleteAndMalformedPatches(t *testing.T) {
	for _, p := range []string{
		"--- a/20-Repos/a.md\n+++ b/20-Repos/a.md\n@@ -2 +2 @@\n-a\n+b\n",
		"--- a/20-Repos/a.md\n+++ b/20-Repos/a.md\n@@ -1 +1 @@\n-a\n+b\n@@ -4 +4 @@\n-a\n+b\n",
		"--- a/../a\n+++ b/../a\n@@ -1 +1 @@\n-a\n+b\n",
		"--- /dev/null\n+++ b/20-Repos/a.md\n@@ -1 +1 @@\n-a\n+b\n",
		"--- a/20-Repos/a.md\n+++ b/20-Repos/a.md\n@@ -1 +1 @@\n-a\n+la sync\n",
		"--- a/20-Repos/a.md\n+++ b/20-Repos/a.md\n@@ -1 +1 @@\n-a\n+\xff\n",
	} {
		if _, e := patchSections([]byte(p)); e == nil {
			t.Fatalf("accepted malformed patch %q", p)
		}
	}
}
func TestProjectionPresenceAndNoNewline(t *testing.T) {
	p := []byte("--- /dev/null\n+++ b/20-Repos/a.md\n@@ -0,0 +1 @@\n+new\n\\ No newline at end of file\n")
	root := t.TempDir()
	out, del, e := projectedImages(root, p)
	if e != nil || string(out["20-Repos/a.md"]) != "new" || del["20-Repos/a.md"] {
		t.Fatal(out, del, e)
	}
	if e = os.Mkdir(filepath.Join(root, "20-Repos"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "20-Repos/a.md"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e = projectedImages(root, p); e == nil {
		t.Fatal("accepted existing empty base for addition")
	}
}
func TestProjectionCoverageAndAcknowledgement(t *testing.T) {
	patch := []byte("--- /dev/null\n+++ b/20-Repos/target-service.md\n@@ -0,0 +1,3 @@\n+---\n+cobertura-datos: invented\n+---\n")
	g, p := projectionFixture(t, patch)
	if e := validateProjection(g, p, patch); e == nil {
		t.Fatal("accepted unsupported coverage")
	}
	ack := obj(arr(g["acknowledgements"])[0])
	r := map[string]any{"repository": ack["repository"], "branch": ack["branch"], "analyzed_sha": str(ack["new_oid"])[:12], "decision": ack["decision"], "analysis_date": ack["analysis_date"]}
	doc := map[string]any{"version": json.Number("1"), "repositories": []any{r}}
	b, _ := canonical(doc)
	b = append(b, '\n')
	patch = []byte("--- /dev/null\n+++ b/" + ackPath + "\n@@ -0,0 +1 @@\n+" + string(b))
	g, p = projectionFixture(t, patch)
	p["unit_type"] = "acknowledgements"
	p["unit_id"] = "acknowledgements"
	p["grants"] = []any{}
	if e := validateProjection(g, p, patch); e != nil {
		t.Fatal(e)
	}
	p["grants"] = []any{"invalid"}
	if validateProjection(g, p, patch) == nil {
		t.Fatal("accepted ack grants")
	}
	if _, e := acknowledgementDocument([]byte("{\"version\":1,\"version\":1,\"repositories\":[]}"), false); e == nil {
		t.Fatal("accepted duplicate keys")
	}
}
