package syncflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func validateClosedReceipt(root, path, id, expected string) (map[string]any, error) {
	rel, e := filepath.Rel(root, path)
	if e != nil || relative(filepath.ToSlash(rel), false) != nil {
		return nil, fail("state-path-invalid")
	}
	p, e := statePath(root, filepath.ToSlash(rel))
	if e != nil {
		return nil, e
	}
	v, e := readJSON(p)
	if e != nil {
		return nil, e
	}
	r := obj(v)
	bad := func() (map[string]any, error) { return nil, fail("run-receipt-invalid") }
	if !exact(r, "version code status run_id fingerprint tool_digest inventory_digest gate_digest packages units receipt_digest") || r["version"] != json.Number("1") || r["code"] != "run-closed" || r["status"] != "complete" || r["run_id"] != id || !nameRE.MatchString(id) {
		return bad()
	}
	for _, k := range strings.Fields("fingerprint tool_digest inventory_digest gate_digest receipt_digest") {
		if !digestRE.MatchString(str(r[k])) {
			return bad()
		}
	}
	if expected != "" && r["fingerprint"] != expected {
		return bad()
	}
	ps, ok := r["packages"].([]any)
	if !ok {
		return bad()
	}
	last := ""
	for _, v := range ps {
		p := obj(v)
		n := str(p["repository"])
		if !exact(p, "repository oid status artifact_digest") || !nameRE.MatchString(n) || n <= last || !oidRE.MatchString(str(p["oid"])) || p["status"] != "checkpointed" || !digestRE.MatchString(str(p["artifact_digest"])) {
			return bad()
		}
		last = n
	}
	fp, e := fingerprint(str(r["tool_digest"]), str(r["inventory_digest"]), ps)
	if e != nil || fp != r["fingerprint"] {
		return bad()
	}
	us, ok := r["units"].([]any)
	if !ok {
		return bad()
	}
	seen := map[string]bool{}
	fields := "unit_id unit_type status gate_digest projection_digest patch_digest receipt_digest"
	for _, v := range us {
		u := obj(v)
		n := str(u["unit_id"])
		if (!exact(u, fields) && !exact(u, fields+" note_review_digest")) || !nameRE.MatchString(n) || seen[n] || !one(u["unit_type"], "acknowledgements", "write-group") || u["status"] != "applied" {
			return bad()
		}
		seen[n] = true
		for _, k := range strings.Fields("gate_digest projection_digest patch_digest receipt_digest") {
			if !digestRE.MatchString(str(u[k])) {
				return bad()
			}
		}
		if x, present := u["note_review_digest"]; present {
			s, ok := x.(string)
			if !ok || s != "" && !digestRE.MatchString(s) || u["unit_type"] == "acknowledgements" && s != "" {
				return bad()
			}
		}
	}
	unsigned := clone(r)
	delete(unsigned, "receipt_digest")
	d, e := digest(unsigned)
	if e != nil || r["receipt_digest"] != d {
		return bad()
	}
	return r, nil
}
func retireClosedActive(root string, r map[string]any) error {
	id := str(r["run_id"])
	p, e := statePath(root, "active", id)
	if e != nil {
		return e
	}
	if _, e = os.Lstat(p); os.IsNotExist(e) {
		return nil
	} else if e != nil {
		return e
	}
	active, e := readState(root, "active", id, "run.json")
	if e == nil && (active["run_id"] != id || active["fingerprint"] != r["fingerprint"]) {
		return fail("active-receipt-conflict")
	}
	retired, e := detachActive(root, id, "closed-orphan")
	if e != nil {
		return e
	}
	_ = os.RemoveAll(retired)
	return nil
}
