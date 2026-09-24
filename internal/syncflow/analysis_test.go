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

func analysisFixture(t *testing.T) (string, map[string]any, map[string]any, map[string]any, string) {
	t.Helper()
	repo := sourceRepo(t)
	if _, e := sourceGit(repo, nil, "remote", "add", "origin", "https://example.invalid/APP00001-example.git"); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(repo, "app.txt"), "old\n")
	old := sourceCommit(t, repo)
	write(t, filepath.Join(repo, "app.txt"), "new\n")
	new := sourceCommit(t, repo)
	m, e := buildManifest(repo, old, new, false)
	if e != nil {
		t.Fatal(e)
	}
	s, e := initializeAnalysis(m, "APP00001-example", nil)
	if e != nil {
		t.Fatal(e)
	}
	a := fallbackAnalysisPayload(s, m)
	ref, e := sourceGit(repo, nil, "symbolic-ref", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	return repo, m, s, a, strings.TrimSpace(string(ref))
}
func TestAnalysisCheckLiveBindings(t *testing.T) {
	repo, m, s, a, ref := analysisFixture(t)
	if e := checkAnalysis(repo, m, s, a, ref); e != nil {
		t.Fatal(e)
	}
	bad := cloneMap(a)
	obj(arr(bad["paths"])[0])["path"] = "other.txt"
	if checkAnalysis(repo, m, s, bad, ref) == nil {
		t.Fatal("accepted uncovered path")
	}
	bad = cloneMap(a)
	obj(obj(bad["checklist"])["inputs"])["evidence"] = []any{map[string]any{"path": "nonexistent.txt", "anchor": "pretend"}}
	if e := checkAnalysis(repo, m, s, bad, ref); e == nil || !strings.Contains(e.Error(), "evidence-path-unavailable") {
		t.Fatal(e)
	}
	write(t, filepath.Join(repo, "app.txt"), "later\n")
	sourceCommit(t, repo)
	if e := checkAnalysis(repo, m, s, a, ref); e == nil || !strings.Contains(e.Error(), "stale-head") {
		t.Fatal(e)
	}
	if checkAnalysis(repo, m, s, a, str(m["new_oid"])) == nil {
		t.Fatal("accepted hash current-ref")
	}
}
func TestAnalysisReviewBoundAndSourceEvidence(t *testing.T) {
	repo, m, s, a, _ := analysisFixture(t)
	md, _ := digest(m)
	sd, _ := digest(s)
	ad, _ := digest(a)
	r := map[string]any{"version": json.Number("3"), "repository": a["repository"], "manifest_digest": md, "scaffold_digest": sd, "analysis_digest": ad, "verdict": "accept", "findings": []any{}}
	if e := validateAnalysisReview(repo, m, s, a, r); e != nil {
		t.Fatal(e)
	}
	r["analysis_digest"] = strings.Repeat("0", 64)
	if validateAnalysisReview(repo, m, s, a, r) == nil {
		t.Fatal("accepted unbound review")
	}
	r["analysis_digest"] = ad
	r["verdict"] = "revise"
	r["findings"] = []any{map[string]any{"target": "claims", "category": "missing", "reason": "not supported", "nodes": []any{"example"}, "evidence": map[string]any{"path": "missing.txt", "anchor": "line 1"}}}
	if e := validateAnalysisReview(repo, m, s, a, r); e == nil || !strings.Contains(e.Error(), "evidence-path-unavailable") {
		t.Fatal(e)
	}
}
func TestAnalysisFinalizeDifferential(t *testing.T) {
	repo, m, s, a, _ := analysisFixture(t)
	a["claims"] = []any{map[string]any{"claim_id": "claim-1", "statement": "New behavior", "evidence": []any{map[string]any{"path": "app.txt", "anchor": "new"}}}}
	obj(arr(a["paths"])[0])["disposition"] = "relevant"
	obj(arr(a["paths"])[0])["claim_ids"] = []any{"claim-1"}
	obj(arr(a["nodes"])[0])["claim_ids"] = []any{"claim-1"}
	a["result"] = "documentation-change"
	dir := t.TempDir()
	mp, sp, ap := filepath.Join(dir, "manifest.json"), filepath.Join(dir, "scaffold.json"), filepath.Join(dir, "analysis.json")
	saveJSON(t, mp, m)
	saveJSON(t, sp, s)
	saveJSON(t, ap, a)
	var out bytes.Buffer
	if e := runAnalysis([]string{"finalize-analysis", "--repo", repo, "--manifest", mp, "--scaffold", sp, "--analysis", ap}, &out); e != nil {
		t.Fatal(e)
	}
	native, e := readJSON(ap)
	if e != nil {
		t.Fatal(e)
	}
	if e = checkAnalysis(repo, m, s, obj(native), ""); e != nil {
		t.Fatal(e)
	}
	py, e := exec.LookPath("python3")
	if e == nil {
		script := filepath.Join("..", "..", "kernel", "90-Meta", "git-change-manifest.py")
		if _, e = os.Stat(script); e == nil {
			saveJSON(t, ap, a)
			b, e := exec.Command(py, "-B", script, "finalize-analysis", "--repo", repo, "--manifest", mp, "--scaffold", sp, "--analysis", ap).CombinedOutput()
			if e != nil {
				t.Fatal(string(b), e)
			}
			reference, e := readJSON(ap)
			if e != nil || !reflect.DeepEqual(reference, native) {
				t.Fatal("finalizer mismatch", reference, native, e)
			}
		}
	}
	saveJSON(t, ap, map[string]any{"claims": []any{}})
	out.Reset()
	if e = runAnalysis([]string{"finalize-analysis", "--repo", repo, "--manifest", mp, "--scaffold", sp, "--analysis", ap}, &out); e != nil {
		t.Fatal(e)
	}
	v, e := readJSON(ap)
	if e != nil || !reflect.DeepEqual(v, fallbackAnalysisPayload(s, m)) {
		t.Fatal(v, e)
	}
}
func TestAnalysisCredentialRedactionAndNoMetadataRewrite(t *testing.T) {
	_, m, s, a, _ := analysisFixture(t)
	m["credential_suspects"] = []any{map[string]any{"path": "app.txt", "line": json.Number("1")}}
	a["claims"] = []any{map[string]any{"claim_id": "claim-1", "statement": "secret-value", "evidence": []any{map[string]any{"path": "app.txt", "anchor": "line 1"}}}}
	obj(arr(a["nodes"])[0])["claim_ids"] = []any{"claim-1"}
	obj(arr(a["paths"])[0])["reason"] = "secret-value"
	out, e := finalizeAnalysisPayload(a, m, []string{"secret-value"})
	if e != nil || len(arr(out["claims"])) != 0 || out["result"] != "traceability-only" {
		t.Fatal(out, e)
	}
	if out["repository"] != s["repository"] {
		t.Fatal("identity altered")
	}
	if redactLiteral("compass pass password pass.", "pass") != "compass [redacted] password [redacted]." {
		t.Fatal("short literal boundaries")
	}
}
func TestAnalysisFinalizerReferenceCorpus(t *testing.T) {
	py, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("optional legacy differential")
	}
	script := filepath.Join("..", "..", "kernel", "90-Meta", "git-change-manifest.py")
	if _, e = os.Stat(script); e != nil {
		t.Skip(e)
	}
	repo, m, s, base, _ := analysisFixture(t)
	cases := map[string]map[string]any{"fallback": cloneMap(base), "sparse": {"repository": base["repository"]}, "scaffold": cloneMap(s), "wrong-binding": cloneMap(base), "blocked": cloneMap(base), "missing-evidence": cloneMap(base), "duplicate-path": cloneMap(base)}
	cases["wrong-binding"]["repository"] = "wrong"
	cases["blocked"]["result"] = "blocked"
	cases["blocked"]["blockers"] = []any{"Cannot read"}
	obj(arr(cases["blocked"]["paths"])[0])["disposition"] = "blocked"
	obj(obj(cases["missing-evidence"]["checklist"])["inputs"])["evidence"] = []any{map[string]any{"path": "absent.txt", "anchor": "missing"}}
	cases["duplicate-path"]["paths"] = append(arr(cases["duplicate-path"]["paths"]), cloneMap(obj(arr(base["paths"])[0])))
	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			d := t.TempDir()
			mp, sp, ap := filepath.Join(d, "m.json"), filepath.Join(d, "s.json"), filepath.Join(d, "a.json")
			saveJSON(t, mp, m)
			saveJSON(t, sp, s)
			saveJSON(t, ap, a)
			var out bytes.Buffer
			if e := runAnalysis([]string{"finalize-analysis", "--repo", repo, "--manifest", mp, "--scaffold", sp, "--analysis", ap}, &out); e != nil {
				t.Fatal(e)
			}
			native, e := readJSON(ap)
			if e != nil {
				t.Fatal(e)
			}
			saveJSON(t, ap, a)
			b, e := exec.Command(py, "-B", script, "finalize-analysis", "--repo", repo, "--manifest", mp, "--scaffold", sp, "--analysis", ap).CombinedOutput()
			if e != nil {
				t.Fatal(string(b), e)
			}
			reference, e := readJSON(ap)
			if e != nil || !reflect.DeepEqual(reference, native) {
				t.Fatalf("finalized mismatch\nnative=%#v\nreference=%#v\nerr=%v", native, reference, e)
			}
		})
	}
}
func TestAnalysisReviewReferenceCorpus(t *testing.T) {
	py, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("optional legacy differential")
	}
	script, _ := filepath.Abs(filepath.Join("..", "..", "kernel", "90-Meta", "git-change-manifest.py"))
	if _, e = os.Stat(script); e != nil {
		t.Skip(e)
	}
	repo, m, s, a, _ := analysisFixture(t)
	md, _ := digest(m)
	sd, _ := digest(s)
	ad, _ := digest(a)
	base := map[string]any{"version": json.Number("3"), "repository": a["repository"], "manifest_digest": md, "scaffold_digest": sd, "analysis_digest": ad, "verdict": "accept", "findings": []any{}}
	cases := map[string]map[string]any{"accept": cloneMap(base), "v1": cloneMap(base), "revise-empty": cloneMap(base), "revise": cloneMap(base)}
	cases["v1"]["version"] = json.Number("1")
	cases["revise-empty"]["verdict"] = "revise"
	cases["revise"]["verdict"] = "revise"
	cases["revise"]["findings"] = []any{map[string]any{"target": "claims.any", "category": "unsupported", "reason": "Check", "nodes": []any{"example"}, "evidence": map[string]any{"path": "app.txt", "anchor": "new"}}}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			d := filepath.Join(t.TempDir(), "fixture.json")
			saveJSON(t, d, map[string]any{"m": m, "s": s, "a": a, "r": r})
			code := `import importlib.util,json,sys
sys.path.insert(0,sys.argv[1].rsplit('/',1)[0])
spec=importlib.util.spec_from_file_location('legacy',sys.argv[1]);mod=importlib.util.module_from_spec(spec);spec.loader.exec_module(mod)
f=json.load(open(sys.argv[2])); print(json.dumps(mod.validate_review(f['r'],f['a']['repository'],f['m'],f['s'],f['a'],set())))`
			b, e := exec.Command(py, "-B", "-c", code, script, d).CombinedOutput()
			if e != nil {
				t.Fatal(string(b), e)
			}
			v, e := decode(b)
			if e != nil {
				t.Fatal(e)
			}
			native := validateAnalysisReview(repo, m, s, a, r)
			if (native == nil) != (len(arr(v)) == 0) {
				t.Fatal("review mismatch", native, string(b))
			}
		})
	}
}
