package syncflow

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func reviewedAnalysisFixture(t *testing.T) (string, map[string]any, map[string]any, map[string]any, map[string]any, string) {
	t.Helper()
	repo, m, s, a, ref := analysisFixture(t)
	a["claims"] = []any{map[string]any{"claim_id": "claim-1", "statement": "New behavior", "evidence": []any{map[string]any{"path": "app.txt", "anchor": "new"}}}}
	obj(arr(a["paths"])[0])["disposition"] = "relevant"
	obj(arr(a["paths"])[0])["claim_ids"] = []any{"claim-1"}
	obj(arr(a["nodes"])[0])["claim_ids"] = []any{"claim-1"}
	a["result"] = "documentation-change"
	var e error
	a, e = finalizeAnalysisPayload(a, m, nil)
	if e != nil {
		t.Fatal(e)
	}
	r := map[string]any{"version": json.Number("3"), "repository": a["repository"], "verdict": "accept", "findings": []any{}}
	bindReview(r, m, s, a)
	return repo, m, s, a, r, ref
}
func bindReview(r, m, s, a map[string]any) {
	for k, v := range map[string]any{"manifest": m, "scaffold": s, "analysis": a} {
		d, _ := digest(v)
		r[k+"_digest"] = d
	}
}
func TestClosePackageReferenceDifferential(t *testing.T) {
	repo, m, s, a, r, ref := reviewedAnalysisFixture(t)
	p, e := closePackage(repo, m, s, a, r, ref, "2026-09-24", "")
	if e != nil {
		t.Fatal(e)
	}
	if len(arr(obj(p["gate"])["write_groups"])) != 1 {
		t.Fatal("valid reviewed claim lost write grant")
	}
	dir := t.TempDir()
	for k, v := range map[string]any{"manifest": m, "scaffold": s, "analysis": a, "review": r} {
		saveJSON(t, filepath.Join(dir, k+".json"), v)
	}
	if py, e := exec.LookPath("python3"); e == nil {
		script := filepath.Join("..", "..", "kernel", "90-Meta", "git-change-manifest.py")
		if _, e = os.Stat(script); e == nil {
			output := filepath.Join(dir, "closed.json")
			b, e := exec.Command(py, "-B", script, "close-package", "--repo", repo, "--manifest", filepath.Join(dir, "manifest.json"), "--scaffold", filepath.Join(dir, "scaffold.json"), "--analysis", filepath.Join(dir, "analysis.json"), "--review", filepath.Join(dir, "review.json"), "--production-ref", ref, "--analysis-date", "2026-09-24", "--output", output).CombinedOutput()
			if e != nil {
				t.Fatal(string(b), e)
			}
			reference, e := readJSON(output)
			if e != nil || !reflect.DeepEqual(reference, p) {
				t.Fatalf("package differs: reference=%v native=%v error=%v", reference, p, e)
			}
		}
	}
	r["analysis_digest"] = emptyHash
	p, e = closePackage(repo, m, s, a, r, ref, "2026-09-24", "")
	if e == nil && len(arr(obj(p["gate"])["write_groups"])) != 0 {
		t.Fatal("invalid review retained authority")
	}
}
func TestFinalizeReviewAndCorrectionWorkflow(t *testing.T) {
	repo, m, s, a, r, ref := reviewedAnalysisFixture(t)
	dir := t.TempDir()
	mp, sp, ap, rp := filepath.Join(dir, "manifest.json"), filepath.Join(dir, "scaffold.json"), filepath.Join(dir, "analysis.json"), filepath.Join(dir, "review.json")
	for p, v := range map[string]any{mp: m, sp: s, ap: a, rp: r} {
		saveJSON(t, p, v)
	}
	out := filepath.Join(dir, "final-review.json")
	if _, e := finalizeReview(repo, mp, sp, ap, rp, out); e != nil {
		t.Fatal(e)
	}
	if _, e := finalizeReview(repo, mp, sp, ap, rp, out); e != nil {
		t.Fatal("idempotent finalization", e)
	}
	r["verdict"] = "revise"
	r["findings"] = []any{map[string]any{"target": "claims.claim-1", "category": "precision", "reason": "Clarify wording", "nodes": []any{"example"}, "evidence": map[string]any{"path": "app.txt", "anchor": "new"}}}
	saveJSON(t, rp, r)
	prepared := call(t, "correction", "prepare", "--repo", repo, "--current-ref", ref, "--manifest", mp, "--scaffold", sp, "--analysis", ap, "--review", rp)
	w := str(prepared["workspace"])
	v, e := readJSON(filepath.Join(w, "analysis.json"))
	if e != nil {
		t.Fatal(e)
	}
	revised := obj(v)
	obj(arr(revised["claims"])[0])["statement"] = "Precisely documented behavior"
	saveJSON(t, filepath.Join(w, "analysis.json"), revised)
	r2 := cloneMap(r)
	r2["verdict"] = "accept"
	r2["findings"] = []any{}
	bindReview(r2, m, s, revised)
	rp2 := filepath.Join(dir, "revised-review.json")
	saveJSON(t, rp2, r2)
	result := call(t, "correction", "check", "--repo", repo, "--current-ref", ref, "--workspace", w, "--review", rp2)
	if result["code"] != "correction-checked" {
		t.Fatal(result)
	}
	obj(arr(revised["nodes"])[0])["reason"] = "allowed finding node"
	if e = correctionScope(a, revised, r); e != nil {
		t.Fatal(e)
	}
	revised["repository"] = "different"
	if e = correctionScope(a, revised, r); e == nil {
		t.Fatal("accepted changed envelope")
	}
}
