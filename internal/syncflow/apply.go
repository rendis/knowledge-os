package syncflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type fileRecord struct{ kind, digest string }

func applyTarget(vault, path string) (string, error) {
	if relative(path, false) != nil {
		return "", fail("projection-path-invalid")
	}
	if _, e := safe(vault, true); e != nil {
		return "", e
	}
	p := vault
	for _, segment := range splitPath(path) {
		p = filepath.Join(p, segment)
		st, e := os.Lstat(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return "", fail("projection-path-invalid")
		}
	}
	return p, nil
}
func splitPath(path string) []string { return strings.Split(path, "/") }
func actualRecord(path string) (fileRecord, error) {
	s, e := os.Lstat(path)
	if os.IsNotExist(e) {
		return fileRecord{"missing", hash(nil)}, nil
	}
	if e != nil {
		return fileRecord{}, e
	}
	if !s.Mode().IsRegular() {
		return fileRecord{}, fail("vault-path-invalid")
	}
	b, e := os.ReadFile(path)
	return fileRecord{"file", hash(b)}, e
}
func unitRecord(u map[string]any, path, prefix string) fileRecord {
	return fileRecord{str(obj(u[prefix+"_kinds"])[path]), str(obj(u[prefix+"_files"])[path])}
}
func writeImage(vault, path, kind string, b []byte) error {
	target, e := applyTarget(vault, path)
	if e != nil {
		return e
	}
	if kind == "missing" {
		e = os.Remove(target)
		if os.IsNotExist(e) {
			return nil
		}
		return e
	}
	if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
		return e
	}
	if _, e = applyTarget(vault, path); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(target), ".sync-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0644); e == nil {
		_, e = f.Write(b)
	}
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
	return os.Rename(tmp, target)
}
func unitByID(r map[string]any, id string) (map[string]any, error) {
	for _, v := range arr(r["units"]) {
		if obj(v)["unit_id"] == id {
			return obj(v), nil
		}
	}
	return nil, fail("unit-not-found")
}
func removeJournal(root, id, unit string) error {
	p, e := statePath(root, "active", id, "units", unit, "apply-journal.json")
	if e != nil {
		return e
	}
	e = os.Remove(p)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
func writeUnitReceipt(root string, r, u map[string]any) error {
	v := map[string]any{"version": json.Number("1"), "run_id": r["run_id"], "status": "applied"}
	for _, k := range []string{"unit_id", "gate_digest", "projection_digest", "patch_digest", "base_files", "result_files", "base_kinds", "result_kinds", "note_review_digest"} {
		v[k] = u[k]
	}
	d, e := digest(v)
	if e != nil {
		return e
	}
	if e = atomicState(root, []string{"active", str(r["run_id"]), "units", str(u["unit_id"]), "receipt.json"}, v); e != nil {
		return e
	}
	u["receipt_digest"] = d
	return nil
}
func checkpointImages(root, id string, u map[string]any) (map[string]patchSection, error) {
	p, e := statePath(root, "active", id, "units", str(u["unit_id"]), "unit.patch")
	if e != nil {
		return nil, e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, e
	}
	if hash(b) != u["patch_digest"] {
		return nil, fail("unit-checkpoint-invalid")
	}
	sections, e := patchSections(b)
	if e != nil {
		return nil, e
	}
	paths := map[string]any{}
	for path, s := range sections {
		paths[path] = nil
		oldKind, newKind := "file", "file"
		if s.OldMissing {
			oldKind = "missing"
		}
		if s.NewMissing {
			newKind = "missing"
		}
		if unitRecord(u, path, "base") != (fileRecord{oldKind, hash(s.Old)}) || unitRecord(u, path, "result") != (fileRecord{newKind, hash(s.New)}) {
			return nil, fail("unit-checkpoint-invalid")
		}
	}
	if !reflect.DeepEqual(sortedKeys(paths), sortedKeys(obj(u["base_files"]))) || !reflect.DeepEqual(sortedKeys(paths), sortedKeys(obj(u["result_files"]))) {
		return nil, fail("unit-checkpoint-invalid")
	}
	return sections, nil
}
func allApplied(r map[string]any) bool {
	if len(arr(r["units"])) == 0 {
		return false
	}
	for _, v := range arr(r["units"]) {
		if obj(v)["status"] != "applied" {
			return false
		}
	}
	return true
}
func applyUnit(root string, r map[string]any, id, vault string, failAfterWrites, crashAfterWrites int, crashAfterBytes bool) (map[string]any, error) {
	u, e := unitByID(r, id)
	if e != nil {
		return nil, e
	}
	vault, e = safe(vault, true)
	if e != nil {
		return nil, e
	}
	loc, e := filepath.Rel(root, vault)
	if e != nil {
		return nil, e
	}
	if r["vault_locator"] != "" && r["vault_locator"] != loc {
		return nil, fail("vault-mismatch")
	}
	response := func(reused bool) map[string]any {
		return map[string]any{"version": json.Number("1"), "code": "unit-applied", "status": "pass", "run_id": r["run_id"], "unit_id": id, "receipt_digest": u["receipt_digest"], "reused": reused}
	}
	if u["status"] == "applied" {
		for _, pv := range sortedKeys(obj(u["result_files"])) {
			p := str(pv)
			target, e := applyTarget(vault, p)
			if e != nil {
				return nil, e
			}
			rec, e := actualRecord(target)
			if e != nil {
				return nil, e
			}
			if rec != unitRecord(u, p, "result") {
				return nil, fail("applied-unit-stale")
			}
		}
		return response(true), nil
	}
	if !one(u["status"], "validated", "apply-failed") {
		return nil, fail("unit-not-validated")
	}
	if u["unit_type"] == "write-group" && u["note_review_digest"] == "" {
		return nil, fail("final-note-review-required")
	}
	if e = unitProof(root, str(r["run_id"]), u); e != nil {
		return nil, e
	}
	images, e := checkpointImages(root, str(r["run_id"]), u)
	if e != nil {
		return nil, e
	}
	r["vault_locator"] = loc
	if e = saveRun(root, r); e != nil {
		return nil, e
	}
	staged := []string{}
	entries := []any{}
	pre, post := false, false
	failState := func(err error) (map[string]any, error) {
		u["status"] = "apply-failed"
		if se := saveRun(root, r); se != nil {
			return nil, se
		}
		return nil, err
	}
	for _, pv := range sortedKeys(obj(u["base_files"])) {
		p := str(pv)
		target, e := applyTarget(vault, p)
		if e != nil {
			return nil, e
		}
		observed, e := actualRecord(target)
		if e != nil {
			return nil, e
		}
		before, after := unitRecord(u, p, "base"), unitRecord(u, p, "result")
		state := "already-applied"
		if observed != after {
			if observed != before {
				return failState(fail("apply-failed"))
			}
			staged = append(staged, p)
			state = "pending"
		}
		if before != after {
			pre = pre || observed == before
			post = post || observed == after
		}
		entries = append(entries, map[string]any{"path": p, "before_digest": before.digest, "before_kind": before.kind, "after_digest": after.digest, "after_kind": after.kind, "status": state})
	}
	if pre && post {
		return failState(fail("atomic-unit-interrupted"))
	}
	journal := map[string]any{"version": json.Number("1"), "run_id": r["run_id"], "unit_id": id, "patch_digest": u["patch_digest"], "status": "prepared", "entries": entries}
	saveJournal := func() error {
		return atomicState(root, []string{"active", str(r["run_id"]), "units", id, "apply-journal.json"}, journal)
	}
	if e = saveJournal(); e != nil {
		return nil, e
	}
	written := []string{}
	rollback := func(err error) (map[string]any, error) {
		failed := false
		for i := len(written) - 1; i >= 0; i-- {
			p := written[i]
			if writeImage(vault, p, unitRecord(u, p, "base").kind, images[p].Old) != nil {
				failed = true
			}
		}
		if !failed {
			if removeJournal(root, str(r["run_id"]), id) != nil {
				failed = true
			}
		}
		if failed {
			err = fail("rollback-failed")
		}
		return failState(err)
	}
	for _, p := range staged {
		if e = writeImage(vault, p, unitRecord(u, p, "result").kind, images[p].New); e != nil {
			return rollback(e)
		}
		written = append(written, p)
		for _, entry := range entries {
			if obj(entry)["path"] == p {
				obj(entry)["status"] = "applied"
			}
		}
		journal["status"] = "applying"
		if e = saveJournal(); e != nil {
			return rollback(e)
		}
		if crashAfterWrites == len(written) {
			os.Exit(99)
		}
		if failAfterWrites == len(written) {
			return rollback(fail("simulated-write-failure"))
		}
	}
	for _, pv := range sortedKeys(obj(u["result_files"])) {
		p := str(pv)
		target, e := applyTarget(vault, p)
		if e != nil {
			return rollback(e)
		}
		rec, e := actualRecord(target)
		if e != nil {
			return rollback(e)
		}
		if rec != unitRecord(u, p, "result") {
			return rollback(fail("apply-failed"))
		}
	}
	if crashAfterBytes {
		return nil, fail("simulated-crash")
	}
	u["status"] = "applied"
	if e = writeUnitReceipt(root, r, u); e != nil {
		return nil, e
	}
	if allApplied(r) {
		r["status"] = "closing"
	}
	if e = saveRun(root, r); e != nil {
		return nil, e
	}
	if e = removeJournal(root, str(r["run_id"]), id); e != nil {
		return nil, e
	}
	return response(len(staged) == 0), nil
}
