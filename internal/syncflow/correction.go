package syncflow

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

func artifactValues(paths ...string) ([]map[string]any, error) {
	out := []map[string]any{}
	for _, p := range paths {
		v, e := readJSON(p)
		if e != nil {
			return nil, e
		}
		m := obj(v)
		if m == nil {
			return nil, fail("invalid-json")
		}
		out = append(out, m)
	}
	return out, nil
}
func finalizeReview(repo, mp, sp, ap, rp, output string) (map[string]any, error) {
	values, e := artifactValues(mp, sp, ap, rp)
	if e != nil {
		return nil, e
	}
	m, s, a, r := values[0], values[1], values[2], values[3]
	if e = checkAnalysis(repo, m, s, a, ""); e != nil {
		return nil, fail("analysis-invalid-or-stale")
	}
	head, e := gitRef(repo, "HEAD")
	if e != nil || head != m["new_oid"] {
		return nil, fail("analysis-invalid-or-stale")
	}
	literals, e := credentialLiterals(repo, m)
	if e != nil {
		return nil, e
	}
	raw, e := canonical(r)
	if e != nil {
		return nil, e
	}
	value, e := decode(raw)
	if e != nil {
		return nil, e
	}
	final := obj(value)
	changed := []any{}
	for i, v := range arr(final["findings"]) {
		f := obj(v)
		if f == nil {
			continue
		}
		for _, surface := range []struct {
			m        map[string]any
			k, label string
		}{{f, "reason", "reason"}, {obj(f["evidence"]), "anchor", "evidence.anchor"}} {
			if old, ok := surface.m[surface.k].(string); ok {
				v := redactValue(old, literals)
				if !reflect.DeepEqual(v, old) {
					surface.m[surface.k] = v
					changed = append(changed, "findings["+itoa(i)+"]."+surface.label)
				}
			}
		}
	}
	if exposed(final, literals) {
		return nil, fail("credential-exposure-outside-review-prose")
	}
	if e = validateAnalysisReview(repo, m, s, a, final); e != nil {
		return nil, fail("review-invalid-or-stale")
	}
	dest, e := filepath.Abs(output)
	if e != nil {
		return nil, e
	}
	source, e := safe(repo, true)
	if e != nil {
		return nil, e
	}
	if within(source, dest) {
		return nil, fail("output-conflicts-with-input")
	}
	for _, p := range []string{mp, sp, ap, rp} {
		p, e := safe(p, false)
		if e != nil {
			return nil, e
		}
		if p == dest {
			return nil, fail("output-conflicts-with-input")
		}
	}
	if e = noClobberJSON(dest, final); e != nil {
		return nil, e
	}
	rd, _ := digest(r)
	fd, _ := digest(final)
	md, _ := digest(m)
	sd, _ := digest(s)
	ad, _ := digest(a)
	return map[string]any{"status": "pass", "input_review_digest": rd, "output_review_digest": fd, "manifest_digest": md, "scaffold_digest": sd, "analysis_digest": ad, "redacted_fields": changed}, nil
}
func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
func within(root, p string) bool {
	rel, e := filepath.Rel(root, p)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func correctionValidate(repo string, paths []string, currentRef string, initial bool) ([]map[string]any, error) {
	values, e := artifactValues(paths...)
	if e != nil {
		return nil, e
	}
	m, s, a, r := values[0], values[1], values[2], values[3]
	if e = checkAnalysis(repo, m, s, a, currentRef); e != nil {
		return nil, e
	}
	literals, e := credentialLiterals(repo, m)
	if e != nil {
		return nil, e
	}
	final, e := finalizeAnalysisPayload(a, m, literals)
	if e != nil {
		return nil, e
	}
	fallback := fallbackAnalysisPayload(s, m)
	existing := !emptyNewManifest(m) && reflect.DeepEqual(a, fallback)
	fallbackChecklist := false
	if len(arr(a["claims"])) == 0 {
		for _, v := range obj(a["checklist"]) {
			if obj(v)["reason"] == fallbackReason {
				fallbackChecklist = true
			}
		}
	}
	if initial && fallbackChecklist && !existing || !existing && !reflect.DeepEqual(final, a) {
		return nil, fail("analysis-not-finalized")
	}
	if e = validateAnalysisReview(repo, m, s, a, r); e != nil {
		return nil, e
	}
	return values, nil
}
func correctionReceipt(values []map[string]any) map[string]any {
	inputs := map[string]any{}
	for i, n := range []string{"manifest", "scaffold", "analysis", "review"} {
		d, _ := digest(values[i])
		inputs[n] = d
	}
	return map[string]any{"version": json.Number("1"), "attempt": json.Number("1"), "inputs": inputs}
}
func correctionScope(initial, revised, review map[string]any) error {
	for _, k := range []string{"version", "repository", "old_oid", "new_oid"} {
		if initial[k] != revised[k] {
			return fail("correction-envelope-changed")
		}
	}
	nodes, paths, claims := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, v := range arr(review["findings"]) {
		f := obj(v)
		for _, n := range arr(f["nodes"]) {
			nodes[str(n)] = true
		}
		paths[str(obj(f["evidence"])["path"])] = true
		target := str(f["target"])
		if strings.HasPrefix(target, "claims.") {
			claims[strings.TrimPrefix(target, "claims.")] = true
		}
	}
	dependencies := map[string]bool{}
	for k := range claims {
		dependencies[k] = true
	}
	for _, v := range arr(initial["nodes"]) {
		n := obj(v)
		if nodes[str(n["basename"])] {
			for _, id := range arr(n["claim_ids"]) {
				dependencies[str(id)] = true
			}
		}
	}
	for _, v := range arr(initial["claims"]) {
		c := obj(v)
		if dependencies[str(c["claim_id"])] {
			for _, e := range arr(c["evidence"]) {
				paths[str(obj(e)["path"])] = true
			}
		}
	}
	for _, spec := range []struct {
		k, id   string
		allowed map[string]bool
	}{{"nodes", "basename", nodes}, {"paths", "path", paths}, {"claims", "claim_id", claims}} {
		old, new := map[string]any{}, map[string]any{}
		all := map[string]bool{}
		for _, v := range arr(initial[spec.k]) {
			id := str(obj(v)[spec.id])
			old[id] = v
			all[id] = true
		}
		for _, v := range arr(revised[spec.k]) {
			id := str(obj(v)[spec.id])
			new[id] = v
			all[id] = true
		}
		for id := range all {
			if reflect.DeepEqual(old[id], new[id]) || spec.allowed[id] {
				continue
			}
			allowed := false
			if spec.k == "claims" && old[id] == nil {
				for _, v := range arr(revised["nodes"]) {
					n := obj(v)
					if nodes[str(n["basename"])] && contains(n["claim_ids"], id) {
						allowed = true
					}
				}
			}
			if !allowed {
				return fail("correction-outside-findings")
			}
		}
	}
	for dimension, v := range obj(initial["checklist"]) {
		old, new := obj(v), obj(obj(revised["checklist"])[dimension])
		if reflect.DeepEqual(old, new) {
			continue
		}
		allowed := false
		for _, v := range arr(review["findings"]) {
			if strings.HasPrefix(str(obj(v)["target"]), "checklist."+dimension) {
				allowed = true
			}
		}
		for _, ev := range append(append([]any{}, arr(old["evidence"])...), arr(new["evidence"])...) {
			if paths[str(obj(ev)["path"])] {
				allowed = true
			}
		}
		if !allowed {
			return fail("correction-outside-findings")
		}
	}
	return nil
}
func runCorrection(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fail("usage: sync correction prepare|check")
	}
	cmd := args[0]
	if cmd != "prepare" && cmd != "check" && cmd != "review-finalize" {
		return fail("unknown-correction-command")
	}
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	repo := f.String("repo", "", "")
	ref := f.String("current-ref", "", "")
	mp := f.String("manifest", "", "")
	sp := f.String("scaffold", "", "")
	ap := f.String("analysis", "", "")
	rp := f.String("review", "", "")
	workspace := f.String("workspace", "", "")
	output := f.String("output", "", "")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if f.NArg() > 0 {
		return fail("unexpected-arguments")
	}
	var result map[string]any
	var e error
	if cmd == "review-finalize" {
		result, e = finalizeReview(*repo, *mp, *sp, *ap, *rp, *output)
	} else if cmd == "prepare" {
		paths := []string{*mp, *sp, *ap, *rp}
		for i, p := range paths {
			paths[i], e = safe(p, false)
			if e != nil {
				return e
			}
		}
		for _, part := range strings.Split(filepath.ToSlash(paths[2]), "/") {
			if part == "correction-1" {
				return fail("correction-limit-reached")
			}
		}
		source, e := safe(*repo, true)
		if e != nil {
			return e
		}
		values, e := correctionValidate(source, paths, *ref, true)
		if e != nil {
			return e
		}
		if values[3]["verdict"] != "revise" {
			return fail("correction-requires-revise")
		}
		w := filepath.Join(filepath.Dir(paths[2]), "correction-1")
		if within(source, w) {
			return fail("source-workspace-forbidden")
		}
		for _, part := range strings.Split(filepath.ToSlash(w), "/") {
			if roots[part] {
				return fail("source-workspace-forbidden")
			}
		}
		if e = os.Mkdir(w, 0700); e != nil {
			return e
		}
		for i, n := range []string{"manifest", "scaffold", "analysis", "review"} {
			p := filepath.Join(w, "initial-"+n+".json")
			if e = noClobberJSON(p, values[i]); e != nil {
				return e
			}
			if e = os.Chmod(p, 0444); e != nil {
				return e
			}
		}
		if e = noClobberJSON(filepath.Join(w, "analysis.json"), values[2]); e != nil {
			return e
		}
		if e = noClobberJSON(filepath.Join(w, "receipt.json"), correctionReceipt(values)); e != nil {
			return e
		}
		if e = os.Chmod(filepath.Join(w, "receipt.json"), 0444); e != nil {
			return e
		}
		result = map[string]any{"status": "pass", "code": "correction-prepared", "workspace": w}
	} else {
		w, e := safe(*workspace, true)
		if e != nil {
			return e
		}
		if filepath.Base(w) != "correction-1" {
			return fail("invalid-correction-workspace")
		}
		paths := []string{}
		for _, n := range []string{"manifest", "scaffold", "analysis", "review"} {
			paths = append(paths, filepath.Join(w, "initial-"+n+".json"))
		}
		values, e := correctionValidate(*repo, paths, *ref, true)
		if e != nil {
			return e
		}
		receipt, e := readJSON(filepath.Join(w, "receipt.json"))
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(receipt, correctionReceipt(values)) {
			return fail("initial-artifacts-changed")
		}
		if values[3]["verdict"] != "revise" {
			return fail("correction-requires-revise")
		}
		revised, e := correctionValidate(*repo, []string{paths[0], paths[1], filepath.Join(w, "analysis.json"), *rp}, *ref, false)
		if e != nil {
			return e
		}
		if reflect.DeepEqual(values[2], revised[2]) || reflect.DeepEqual(values[3], revised[3]) {
			return fail("correction-needs-new-analysis-and-review")
		}
		if e = correctionScope(values[2], revised[2], values[3]); e != nil {
			return e
		}
		ad, _ := digest(revised[2])
		rd, _ := digest(revised[3])
		result = map[string]any{"status": "pass", "code": "correction-checked", "analysis_digest": ad, "review_digest": rd}
	}
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(result)
}
