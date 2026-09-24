package syncflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"unicode/utf8"
)

func findUnit(r map[string]any, id string) (map[string]any, error) {
	for _, v := range arr(r["units"]) {
		u := obj(v)
		if u["unit_id"] == id {
			return u, nil
		}
	}
	return nil, fail("unit-not-found")
}
func saveRun(root string, r map[string]any) error {
	return atomicState(root, []string{"active", str(r["run_id"]), "run.json"}, r)
}
func atomicStateBytes(root string, parts []string, b []byte) error {
	p, e := statePath(root, parts...)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	if _, e = statePath(root, parts...); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".state-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), p)
}
func removeState(root string, parts ...string) error {
	p, e := statePath(root, parts...)
	if e != nil {
		return e
	}
	e = os.Remove(p)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
func validateUnit(root string, r map[string]any, id, projectionPath, patchPath string) (map[string]any, error) {
	if r["gate_digest"] == "" || r["gate_stale"] == true {
		return nil, fail("gate-not-sealed")
	}
	u, e := findUnit(r, id)
	if e != nil {
		return nil, e
	}
	if u["status"] == "applied" {
		return nil, fail("unit-already-applied")
	}
	invalid := func(code, field, attempt string) (map[string]any, error) {
		u["status"] = "projection-invalid"
		if attempt != "" && !contains(u["failed_patch_digests"], attempt) {
			a := append(arr(u["failed_patch_digests"]), attempt)
			sort.Slice(a, func(i, j int) bool { return str(a[i]) < str(a[j]) })
			u["failed_patch_digests"] = a
		}
		r["status"] = "projecting"
		if e := saveRun(root, r); e != nil {
			return nil, e
		}
		return map[string]any{"version": json.Number("1"), "code": "projection-invalid", "status": "blocked", "retryable": true, "resume_from": "projection", "run_id": r["run_id"], "unit_id": id, "issues": []any{map[string]any{"code": code, "field": field}}}, nil
	}
	pv, e := readJSON(projectionPath)
	p := obj(pv)
	if e != nil || p == nil {
		return invalid("projection-contract-invalid", "projection", "")
	}
	patch, e := os.ReadFile(patchPath)
	if e != nil {
		patch = nil
	}
	patchDigest := hash(patch)
	if !utf8.Valid(patch) {
		return invalid("patch-encoding-invalid", "patch", patchDigest)
	}
	if sensitive(string(patch)) {
		return nil, fail("checkpoint-sensitive-content")
	}
	pd, e := digest(p)
	if e != nil {
		return nil, e
	}
	attempt, _ := digest(map[string]any{"projection_digest": pd, "patch_digest": patchDigest})
	if contains(u["failed_patch_digests"], attempt) {
		return nil, fail("failed-patch-digest-reused")
	}
	if p["unit_id"] != id {
		return invalid("projection-unit-mismatch", "projection.unit_id", attempt)
	}
	if p["run_id"] != r["run_id"] || p["gate_digest"] != r["gate_digest"] || p["patch_digest"] != patchDigest {
		return invalid("projection-binding-invalid", "projection", attempt)
	}
	if u["status"] == "stale" && u["projection_digest"] != "" && u["projection_digest"] == pd {
		return nil, fail("stale-projection-digest-reused")
	}
	result := map[string]any{"version": json.Number("1"), "code": "projection-valid", "status": "pass", "issues": []any{}, "run_id": r["run_id"], "unit_id": id, "reused": false}
	if u["status"] == "validated" && u["projection_digest"] == pd && u["patch_digest"] == patchDigest {
		result["reused"] = true
		return result, nil
	}
	g, e := readState(root, "active", str(r["run_id"]), "gate.json")
	if e != nil {
		return nil, e
	}
	if e = validateProjection(g, p, patch); e != nil {
		return invalid(e.Error(), "projection", attempt)
	}
	sections, e := patchSections(patch)
	if e != nil {
		return invalid(e.Error(), "patch", attempt)
	}
	dir := []string{"active", str(r["run_id"]), "units", id}
	if e = atomicState(root, append(dir, "projection.json"), p); e != nil {
		return nil, e
	}
	if e = atomicStateBytes(root, append(dir, "unit.patch"), patch); e != nil {
		return nil, e
	}
	if e = removeState(root, append(dir, "note-review.json")...); e != nil {
		return nil, e
	}
	baseKinds, resultKinds := map[string]any{}, map[string]any{}
	for path, s := range sections {
		baseKinds[path] = "file"
		resultKinds[path] = "file"
		if s.OldMissing {
			baseKinds[path] = "missing"
		}
		if s.NewMissing {
			resultKinds[path] = "missing"
		}
	}
	u["status"] = "validated"
	u["stale_reason"] = ""
	u["gate_digest"] = r["gate_digest"]
	u["projection_digest"] = pd
	u["patch_digest"] = patchDigest
	u["base_files"] = p["base_files"]
	u["result_files"] = p["result_files"]
	u["base_kinds"] = baseKinds
	u["result_kinds"] = resultKinds
	u["note_review_digest"] = ""
	u["receipt_digest"] = ""
	r["status"] = "projecting"
	if e = saveRun(root, r); e != nil {
		return nil, e
	}
	return result, nil
}
func reviewUnit(root string, r map[string]any, id, vault, candidate, evidence, manifestPath, reviewPath string) (map[string]any, error) {
	u, e := findUnit(r, id)
	if e != nil {
		return nil, e
	}
	if u["unit_type"] != "write-group" {
		return nil, fail("note-review-not-required")
	}
	if !one(u["status"], "validated", "apply-failed") {
		return nil, fail("projection-validation-required")
	}
	mv, e := readJSON(manifestPath)
	if e != nil {
		return nil, e
	}
	m, e := manifest(mv)
	if e != nil {
		return nil, e
	}
	rv, e := readJSON(reviewPath)
	if e != nil {
		return nil, e
	}
	if e = review(rv, m); e != nil {
		return nil, e
	}
	dir := []string{"active", str(r["run_id"]), "units", id}
	projection, e := statePath(root, append(dir, "projection.json")...)
	if e != nil {
		return nil, e
	}
	ev := []string{}
	for n := range obj(m["evidence_files"]) {
		ev = append(ev, n)
	}
	del, _ := stringsFrom(m["deleted_files"])
	current, e := freeze(vault, candidate, evidence, ev, del, projection)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(current, m) {
		return nil, fail("candidate-base-or-evidence-stale")
	}
	ma, e := readJSON(manifestPath)
	if e != nil {
		return nil, e
	}
	ra, e := readJSON(reviewPath)
	if e != nil {
		return nil, e
	}
	md, _ := digest(m)
	if !reflect.DeepEqual(ma, m) || !reflect.DeepEqual(rv, ra) || m["projection_digest"] != u["projection_digest"] {
		return nil, fail("final-note-review-stale")
	}
	bk, rk := map[string]any{}, map[string]any{}
	for n := range obj(m["base_files"]) {
		bk[n] = "missing"
		if contains(m["base_present"], n) {
			bk[n] = "file"
		}
	}
	for n := range obj(m["candidate_files"]) {
		rk[n] = "file"
		if contains(m["deleted_files"], n) {
			rk[n] = "missing"
		}
	}
	projectedBase, projectedResult := clone(obj(u["base_kinds"])), clone(obj(u["result_kinds"]))
	delete(projectedBase, "90-Meta/.sync-acknowledgements.json")
	delete(projectedResult, "90-Meta/.sync-acknowledgements.json")
	if !reflect.DeepEqual(bk, projectedBase) || !reflect.DeepEqual(rk, projectedResult) {
		return nil, fail("final-note-review-kind-mismatch")
	}
	rd, _ := digest(rv)
	receipt := map[string]any{"version": json.Number("1"), "code": "note-unit-reviewed", "status": "pass", "run_id": r["run_id"], "unit_id": id, "gate_digest": u["gate_digest"], "projection_digest": u["projection_digest"], "manifest_digest": md, "review_digest": rd}
	receiptDigest, _ := digest(receipt)
	reused := u["note_review_digest"] != ""
	if reused && u["note_review_digest"] != receiptDigest {
		return nil, fail("final-note-review-conflict")
	}
	if !reused {
		if e = atomicState(root, append(dir, "note-review.json"), receipt); e != nil {
			return nil, e
		}
		u["note_review_digest"] = receiptDigest
		if e = saveRun(root, r); e != nil {
			return nil, e
		}
	}
	receipt["receipt_digest"] = receiptDigest
	receipt["reused"] = reused
	return receipt, nil
}
