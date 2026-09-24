package syncflow

import (
	"os"
	"path/filepath"
	"strings"
)

func detachActive(root, id, reason string) (string, error) {
	src, e := statePath(root, "active", id)
	if e != nil {
		return "", e
	}
	dst, e := statePath(root, "retired", id+"-"+reason)
	if e != nil {
		return "", e
	}
	if _, e = os.Lstat(dst); e == nil {
		return "", fail("state-path-invalid")
	} else if !os.IsNotExist(e) {
		return "", e
	}
	if e = os.MkdirAll(filepath.Dir(dst), 0700); e != nil {
		return "", e
	}
	if e = os.Rename(src, dst); e != nil {
		return "", e
	}
	return dst, nil
}
func runVault(root string, r map[string]any) (string, error) {
	loc := str(r["vault_locator"])
	if loc == "" {
		return "", nil
	}
	return safe(filepath.Clean(filepath.Join(root, loc)), true)
}
func retractStaleUnit(root string, r, u map[string]any, vault string) error {
	images, e := checkpointImages(root, str(r["run_id"]), u)
	if e != nil {
		return e
	}
	staged := []string{}
	for _, pv := range sortedKeys(obj(u["result_files"])) {
		p := str(pv)
		target, e := applyTarget(vault, p)
		if e != nil {
			return e
		}
		rec, e := actualRecord(target)
		if e != nil {
			return e
		}
		before, after := unitRecord(u, p, "base"), unitRecord(u, p, "result")
		if rec != before && rec != after {
			return fail("stale-publication-conflict")
		}
		if rec == after && before != after {
			staged = append(staged, p)
		}
	}
	written := []string{}
	for _, p := range staged {
		if e = writeImage(vault, p, unitRecord(u, p, "base").kind, images[p].Old); e != nil {
			for i := len(written) - 1; i >= 0; i-- {
				q := written[i]
				if writeImage(vault, q, unitRecord(u, q, "result").kind, images[q].New) != nil {
					return fail("rollback-failed")
				}
			}
			return e
		}
		written = append(written, p)
	}
	for _, pv := range sortedKeys(obj(u["base_files"])) {
		p := str(pv)
		target, e := applyTarget(vault, p)
		if e != nil {
			return e
		}
		rec, e := actualRecord(target)
		if e != nil {
			return e
		}
		if rec != unitRecord(u, p, "base") {
			return fail("publication-retraction-failed")
		}
	}
	return nil
}
func resumeRun(root string, r map[string]any, sources, destinations map[string]string) (map[string]any, error) {
	invalidP, invalidU := map[string]any{}, map[string]any{}
	for _, v := range arr(r["units"]) {
		u := obj(v)
		if u["status"] == "stale" {
			invalidU[str(u["unit_id"])] = nil
		}
	}
	for repository, oid := range sources {
		if !nameRE.MatchString(repository) || !oidRE.MatchString(oid) {
			return nil, fail("package-contract-invalid")
		}
		var pk map[string]any
		for _, v := range arr(r["packages"]) {
			if obj(v)["repository"] == repository {
				pk = obj(v)
			}
		}
		if pk == nil {
			return nil, fail("package-not-found")
		}
		if pk["oid"] == oid {
			continue
		}
		pk["oid"] = oid
		pk["status"] = "stale"
		pk["artifact_digest"] = ""
		invalidP[repository] = nil
		for _, v := range arr(r["units"]) {
			u := obj(v)
			for _, repo := range arr(u["repositories"]) {
				if repo == repository {
					u["status"] = "stale"
					u["stale_reason"] = "source"
					invalidU[str(u["unit_id"])] = nil
				}
			}
		}
		r["gate_stale"] = true
		r["status"] = "packages"
		fp, e := fingerprint(str(r["tool_digest"]), str(r["inventory_digest"]), arr(r["packages"]))
		if e != nil {
			return nil, e
		}
		r["fingerprint"] = fp
		if e = saveRun(root, r); e != nil {
			return nil, e
		}
		p, e := statePath(root, "active", str(r["run_id"]), "packages", repository, "artifact.json")
		if e != nil {
			return nil, e
		}
		if e = os.Remove(p); e != nil && !os.IsNotExist(e) {
			return nil, e
		}
	}
	vault, e := runVault(root, r)
	if e != nil {
		return nil, e
	}
	retracted := []any{}
	for _, v := range arr(r["units"]) {
		u := obj(v)
		if u["status"] != "stale" || u["stale_reason"] != "source" || u["receipt_digest"] == "" {
			continue
		}
		if vault == "" {
			return nil, fail("stale-publication-unavailable")
		}
		if e = retractStaleUnit(root, r, u, vault); e != nil {
			return nil, e
		}
		u["stale_reason"] = "source-retracted"
		retracted = append(retracted, u["unit_id"])
		if e = saveRun(root, r); e != nil {
			return nil, e
		}
	}
	if len(invalidP) > 0 {
		p, e := statePath(root, "receipts", "fingerprints", str(r["fingerprint"])+".json")
		if e != nil {
			return nil, e
		}
		if _, e = os.Lstat(p); e == nil {
			v, e := readJSON(p)
			if e != nil {
				return nil, e
			}
			rid := str(obj(v)["run_id"])
			receipt, e := validateClosedReceipt(root, p, rid, str(r["fingerprint"]))
			if e != nil {
				return nil, e
			}
			retired, e := detachActive(root, str(r["run_id"]), "superseded-"+str(r["fingerprint"])[:12])
			if e != nil {
				return nil, e
			}
			_ = os.RemoveAll(retired)
			receipt["reused"] = true
			receipt["retracted_units"] = retracted
			return receipt, nil
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	for path, d := range destinations {
		if relative(path, false) != nil || !digestRE.MatchString(d) {
			return nil, fail("destination-contract-invalid")
		}
		for _, v := range arr(r["units"]) {
			u := obj(v)
			matches := obj(u["base_files"])[path] != nil || (u["unit_type"] == "acknowledgements" && path == "90-Meta/.sync-acknowledgements.json")
			for _, n := range arr(u["nodes"]) {
				matches = matches || strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == n
			}
			if !matches {
				continue
			}
			prefix := "base"
			if u["status"] == "applied" {
				prefix = "result"
			}
			if obj(u[prefix+"_files"])[path] != d {
				u["status"] = "stale"
				u["stale_reason"] = "destination"
				invalidU[str(u["unit_id"])] = nil
			}
		}
	}
	reconciled := []any{}
	if vault != "" {
		for _, v := range arr(r["units"]) {
			u := obj(v)
			if _, ok := invalidU[str(u["unit_id"])]; ok {
				continue
			}
			if !one(u["status"], "validated", "apply-failed") || len(obj(u["result_files"])) == 0 {
				continue
			}
			all, anyChanged, allChanged := true, false, true
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
				after := unitRecord(u, p, "result")
				match := rec == after
				all = all && match
				if unitRecord(u, p, "base") != after {
					anyChanged = anyChanged || match
					allChanged = allChanged && match
				}
			}
			if anyChanged && !allChanged {
				u["status"] = "apply-failed"
				if e = saveRun(root, r); e != nil {
					return nil, e
				}
				return nil, fail("atomic-unit-interrupted")
			}
			if all {
				if u["unit_type"] == "write-group" && u["note_review_digest"] == "" {
					return nil, fail("final-note-review-required")
				}
				u["status"] = "applied"
				if e = writeUnitReceipt(root, r, u); e != nil {
					return nil, e
				}
				reconciled = append(reconciled, u["unit_id"])
			}
		}
	}
	if allApplied(r) {
		r["status"] = "closing"
	} else if r["gate_digest"] != "" && r["gate_stale"] == false {
		r["status"] = "projecting"
	}
	if e = saveRun(root, r); e != nil {
		return nil, e
	}
	for _, id := range reconciled {
		if e = removeJournal(root, str(r["run_id"]), str(id)); e != nil {
			return nil, e
		}
	}
	reused := map[string]any{}
	for _, v := range arr(r["units"]) {
		id := str(obj(v)["unit_id"])
		if _, ok := invalidU[id]; !ok {
			reused[id] = nil
		}
	}
	out := publicStatus(root, r)
	code := "run-resumable"
	if len(invalidP) > 0 {
		code = "source-stale"
	} else if len(invalidU) > 0 {
		code = "vault-baseline-stale"
	}
	out["code"] = code
	out["invalidated_packages"] = sortedKeys(invalidP)
	out["invalidated_units"] = sortedKeys(invalidU)
	out["reused_units"] = sortedKeys(reused)
	out["reconciled_units"] = reconciled
	out["retracted_units"] = retracted
	return out, nil
}
func closeRun(root string, r map[string]any) (map[string]any, error) {
	if r["gate_digest"] == "" || r["gate_stale"] == true {
		return nil, fail("run-not-closable")
	}
	for _, v := range arr(r["units"]) {
		if obj(v)["status"] != "applied" {
			return nil, fail("run-not-closable")
		}
	}
	vault, e := runVault(root, r)
	if e != nil {
		return nil, e
	}
	if len(arr(r["units"])) > 0 && vault == "" {
		return nil, fail("run-not-closable")
	}
	stale := false
	for _, v := range arr(r["units"]) {
		u := obj(v)
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
				u["status"] = "stale"
				u["stale_reason"] = "destination"
				stale = true
			}
		}
	}
	if stale {
		r["status"] = "projecting"
		if e = saveRun(root, r); e != nil {
			return nil, e
		}
		return nil, fail("vault-baseline-stale")
	}
	r["status"] = "complete"
	receipt := receiptView(r)
	receipt["code"] = "run-closed"
	d, e := digest(receipt)
	if e != nil {
		return nil, e
	}
	receipt["receipt_digest"] = d
	for _, parts := range [][]string{{"receipts", str(r["run_id"]) + ".json"}, {"receipts", "fingerprints", str(r["fingerprint"]) + ".json"}} {
		if e = atomicState(root, parts, receipt); e != nil {
			return nil, e
		}
	}
	retired, e := detachActive(root, str(r["run_id"]), "closed")
	if e != nil {
		return nil, e
	}
	if e = os.RemoveAll(retired); e != nil {
		return nil, e
	}
	return receipt, nil
}
