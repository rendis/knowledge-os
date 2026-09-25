package syncflow

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func beginRun(root, inventory, tool string, packages []string) (map[string]any, error) {
	d, e := executableDigest()
	if e != nil {
		return nil, e
	}
	if tool != "" && tool != d {
		return nil, fail("run-version-mismatch")
	}
	if !digestRE.MatchString(inventory) {
		return nil, fail("inventory-digest-invalid")
	}
	psByName := map[string]any{}
	for _, p := range packages {
		n, oid, ok := strings.Cut(p, "=")
		if !ok || !nameRE.MatchString(n) || !oidRE.MatchString(oid) || psByName[n] != nil {
			return nil, fail("package-contract-invalid")
		}
		psByName[n] = map[string]any{"repository": n, "oid": oid, "status": "pending", "artifact_digest": "", "checkpoint_count": json.Number("0")}
	}
	if len(psByName) == 0 {
		return nil, fail("package-contract-invalid")
	}
	ps := []any{}
	for _, n := range sortedKeys(psByName) {
		ps = append(ps, psByName[str(n)])
	}
	fp, _ := fingerprint(d, inventory, ps)
	reuse := func(r map[string]any) (map[string]any, error) { r = clone(r); r["reused"] = true; return r, nil }
	indexed, e := statePath(root, "receipts", "fingerprints", fp+".json")
	if e != nil {
		return nil, e
	}
	if _, e = os.Lstat(indexed); e == nil {
		v, e := readJSON(indexed)
		if e != nil {
			return nil, e
		}
		rid := str(obj(v)["run_id"])
		r, e := validateClosedReceipt(root, indexed, rid, fp)
		if e != nil {
			return nil, e
		}
		return reuse(r)
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	rid := "run-" + fp[:24]
	for gen := 1; ; gen++ {
		candidate := rid
		if gen > 1 {
			candidate = fmt.Sprintf("%s-%d", rid, gen)
		}
		closed, e := statePath(root, "receipts", candidate+".json")
		if e != nil {
			return nil, e
		}
		if _, e = os.Lstat(closed); e == nil {
			r, e := validateClosedReceipt(root, closed, candidate, "")
			if e != nil {
				return nil, e
			}
			if r["fingerprint"] == fp {
				return reuse(r)
			}
			continue
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		if _, found, e := archivedAbandonment(root, candidate); e != nil {
			return nil, e
		} else if found {
			continue
		}
		p, e := statePath(root, "active", candidate, "run.json")
		if e != nil {
			return nil, e
		}
		if _, e = os.Lstat(p); e == nil {
			r, e := loadRun(root, candidate)
			if e != nil {
				return nil, e
			}
			if r["fingerprint"] != fp {
				return nil, fail("run-fingerprint-mismatch")
			}
			return reuse(publicStatus(root, r))
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		rid = candidate
		break
	}
	path, e := statePath(root, "active", rid)
	if e != nil {
		return nil, e
	}
	if _, e = os.Lstat(path); e == nil {
		return nil, fail("run-state-missing")
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	active, e := statePath(root, "active")
	if e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(active)
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fail("state-path-invalid")
		}
		if !entry.IsDir() {
			continue
		}
		closed, e := statePath(root, "receipts", entry.Name()+".json")
		if e != nil {
			return nil, e
		}
		if _, e = os.Lstat(closed); e == nil {
			r, e := validateClosedReceipt(root, closed, entry.Name(), "")
			if e != nil {
				return nil, e
			}
			if e = retireClosedActive(root, r); e != nil {
				return nil, e
			}
			if r["fingerprint"] == fp {
				return reuse(r)
			}
			continue
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		r, e := loadRun(root, entry.Name())
		if e != nil {
			return nil, e
		}
		if r["fingerprint"] == fp {
			return reuse(publicStatus(root, r))
		}
		return nil, fail("active-run-conflict")
	}
	r := map[string]any{"version": json.Number("1"), "run_id": rid, "fingerprint": fp, "tool_digest": d, "inventory_digest": inventory, "status": "packages", "packages": ps, "gate_digest": "", "gate_stale": false, "units": []any{}, "vault_locator": ""}
	if e = saveRun(root, r); e != nil {
		return nil, e
	}
	out := publicStatus(root, r)
	out["reused"] = false
	return out, nil
}
