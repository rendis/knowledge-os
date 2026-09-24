package syncflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewedLifecycleThroughPublicCLI(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "state")
	vault := filepath.Join(dir, "vault")
	candidate := filepath.Join(dir, "candidate")
	evidence := filepath.Join(dir, "evidence")
	note := "20-Repos/target-service.md"
	write(t, filepath.Join(vault, note), "old\n")
	write(t, filepath.Join(candidate, note), "new\n")
	write(t, filepath.Join(evidence, "source.txt"), "source\n")
	start := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", "APP00000-source-adapter", strings.Repeat("a", 40), "--package", "APP00000-target-service", strings.Repeat("b", 40))
	id := str(start["run_id"])
	for _, name := range []string{"APP00000-source-adapter", "APP00000-target-service"} {
		call(t, "checkpoint-package", "--state-root", root, "--run-id", id, "--repository", name, "--artifact", filepath.Join("testdata", name+".json"))
	}
	call(t, "seal-gate", "--state-root", root, "--run-id", id, "--gate", "testdata/gate.json")
	g := fixture(t, "gate.json")
	gd, _ := digest(g)
	ack := obj(arr(g["acknowledgements"])[0])
	ackDoc := map[string]any{"version": json.Number("1"), "repositories": []any{map[string]any{"repository": ack["repository"], "branch": ack["branch"], "analyzed_sha": str(ack["new_oid"])[:12], "decision": ack["decision"], "analysis_date": ack["analysis_date"]}}}
	ackBytes, _ := canonical(ackDoc)
	ackPatch := []byte("--- /dev/null\n+++ b/" + ackPath + "\n@@ -0,0 +1 @@\n+" + string(ackBytes) + "\n")
	docPatch := []byte("--- a/" + note + "\n+++ b/" + note + "\n@@ -1 +1 @@\n-old\n+new\n")
	for _, pair := range []struct {
		id    string
		patch []byte
	}{{"acknowledgements", ackPatch}, {"group-001", docPatch}} {
		_, p := projectionFixture(t, pair.patch)
		p["run_id"] = id
		p["gate_digest"] = gd
		p["unit_id"] = pair.id
		if pair.id == "acknowledgements" {
			p["unit_type"] = "acknowledgements"
			p["grants"] = []any{}
		}
		pp := filepath.Join(dir, pair.id+".json")
		patchPath := filepath.Join(dir, pair.id+".patch")
		saveJSON(t, pp, p)
		write(t, patchPath, string(pair.patch))
		call(t, "validate-unit", "--state-root", root, "--run-id", id, "--unit-id", pair.id, "--projection", pp, "--patch", patchPath)
		if pair.id == "group-001" {
			mp := filepath.Join(dir, "note-manifest.json")
			rp := filepath.Join(dir, "note-review.json")
			frozen := call(t, "review", "freeze", "--vault", vault, "--candidate", candidate, "--evidence-root", evidence, "--evidence", "source.txt", "--projection", pp, "--output", mp)
			saveJSON(t, rp, map[string]any{"version": json.Number("1"), "manifest_digest": frozen["manifest_digest"], "verdict": "accept", "findings": []any{}, "connection_decisions": map[string]any{}})
			call(t, "review-unit", "--state-root", root, "--run-id", id, "--unit-id", pair.id, "--vault", vault, "--candidate", candidate, "--evidence-root", evidence, "--manifest", mp, "--review", rp)
		}
		call(t, "apply-unit", "--state-root", root, "--run-id", id, "--unit-id", pair.id, "--vault", vault)
	}
	closed := call(t, "close", "--state-root", root, "--run-id", id)
	if closed["status"] != "complete" {
		t.Fatal(closed)
	}
	again := call(t, "close", "--state-root", root, "--run-id", id)
	if again["receipt_digest"] != closed["receipt_digest"] {
		t.Fatal("closed replay changed")
	}
	if call(t, "status", "--state-root", root, "--run-id", id)["code"] != "run-closed" {
		t.Fatal("closed status unavailable")
	}
	b, e := os.ReadFile(filepath.Join(vault, note))
	if e != nil || string(b) != "new\n" {
		t.Fatal("note not published")
	}
	reused := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", "APP00000-source-adapter", strings.Repeat("a", 40), "--package", "APP00000-target-service", strings.Repeat("b", 40))
	if reused["reused"] != true || reused["code"] != "run-closed" {
		t.Fatal("closed fingerprint not reused")
	}
}
func TestGateBatchReproducesLegacyFixture(t *testing.T) {
	g, e := gateBatch([]map[string]any{fixture(t, "APP00000-target-service.json"), fixture(t, "APP00000-source-adapter.json")}, []string{"APP00000-source-adapter", "APP00000-target-service"})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := digest(g)
	b, _ := digest(fixture(t, "gate.json"))
	if a != b {
		t.Fatal("combined gate differs from independently produced legacy gate")
	}
}
