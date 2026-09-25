package syncflow

import (
	"encoding/json"
	"os"
)

const abandonmentFields = "version code status run_id fingerprint tool_digest inventory_digest run_digest receipt_digest"

func pristineEmptyRun(r map[string]any, id string) error {
	if !exact(r, runFields) || r["version"] != json.Number("1") || r["run_id"] != id || r["status"] != "packages" || r["gate_digest"] != "" || r["gate_stale"] != false || len(arr(r["units"])) != 0 || r["vault_locator"] != "" {
		return fail("run-not-abandonable")
	}
	for _, key := range []string{"fingerprint", "tool_digest", "inventory_digest"} {
		if !digestRE.MatchString(str(r[key])) {
			return fail("run-not-abandonable")
		}
	}
	packages, ok := r["packages"].([]any)
	if !ok || len(packages) == 0 {
		return fail("run-not-abandonable")
	}
	previous := ""
	for _, value := range packages {
		p := obj(value)
		count, countOK := p["checkpoint_count"].(json.Number)
		n, countErr := count.Int64()
		name := str(p["repository"])
		if !exact(p, "repository oid status artifact_digest checkpoint_count") || !nameRE.MatchString(name) || name <= previous || !oidRE.MatchString(str(p["oid"])) || p["status"] != "pending" || p["artifact_digest"] != "" || !countOK || countErr != nil || n != 0 {
			return fail("run-not-abandonable")
		}
		previous = name
	}
	fingerprintValue, err := fingerprint(str(r["tool_digest"]), str(r["inventory_digest"]), packages)
	if err != nil || r["fingerprint"] != fingerprintValue {
		return fail("run-not-abandonable")
	}
	return nil
}

func abandonmentReceipt(r map[string]any) (map[string]any, error) {
	runDigest, err := digest(r)
	if err != nil {
		return nil, err
	}
	receipt := map[string]any{
		"version":          json.Number("1"),
		"code":             "run-abandoned",
		"status":           "abandoned",
		"run_id":           r["run_id"],
		"fingerprint":      r["fingerprint"],
		"tool_digest":      r["tool_digest"],
		"inventory_digest": r["inventory_digest"],
		"run_digest":       runDigest,
	}
	receiptDigest, err := digest(receipt)
	if err != nil {
		return nil, err
	}
	receipt["receipt_digest"] = receiptDigest
	return receipt, nil
}

func validateAbandonmentAt(root, id string, base ...string) (map[string]any, error) {
	directory, err := statePath(root, base...)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 || entries[0].Name() != "abandonment.json" || entries[1].Name() != "run.json" {
		return nil, fail("run-abandonment-invalid")
	}
	r, err := readState(root, append(base, "run.json")...)
	if err != nil {
		return nil, err
	}
	if err = pristineEmptyRun(r, id); err != nil {
		return nil, fail("run-abandonment-invalid")
	}
	receipt, err := readState(root, append(base, "abandonment.json")...)
	if err != nil {
		return nil, err
	}
	if !exact(receipt, abandonmentFields) || receipt["version"] != json.Number("1") || receipt["code"] != "run-abandoned" || receipt["status"] != "abandoned" || receipt["run_id"] != id || receipt["fingerprint"] != r["fingerprint"] || receipt["tool_digest"] != r["tool_digest"] || receipt["inventory_digest"] != r["inventory_digest"] {
		return nil, fail("run-abandonment-invalid")
	}
	runDigest, err := digest(r)
	if err != nil || receipt["run_digest"] != runDigest {
		return nil, fail("run-abandonment-invalid")
	}
	unsigned := clone(receipt)
	delete(unsigned, "receipt_digest")
	receiptDigest, err := digest(unsigned)
	if err != nil || receipt["receipt_digest"] != receiptDigest {
		return nil, fail("run-abandonment-invalid")
	}
	return receipt, nil
}

func archivedAbandonment(root, id string) (map[string]any, bool, error) {
	if !nameRE.MatchString(id) {
		return nil, false, fail("invalid-run-id")
	}
	path, err := statePath(root, "retired", id+"-abandoned")
	if err != nil {
		return nil, false, err
	}
	if _, err = os.Lstat(path); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	receipt, err := validateAbandonmentAt(root, id, "retired", id+"-abandoned")
	return receipt, true, err
}

func abandonEmpty(root string, r map[string]any) (map[string]any, error) {
	id := str(r["run_id"])
	if err := pristineEmptyRun(r, id); err != nil {
		return nil, err
	}
	directory, err := statePath(root, "active", id)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	prepared := len(entries) == 2 && entries[0].Name() == "abandonment.json" && entries[1].Name() == "run.json"
	if len(entries) != 1 || entries[0].Name() != "run.json" {
		if !prepared {
			return nil, fail("run-not-abandonable")
		}
	}
	receipt, err := abandonmentReceipt(r)
	if err != nil {
		return nil, err
	}
	if prepared {
		stored, validationErr := validateAbandonmentAt(root, id, "active", id)
		if validationErr != nil || stored["receipt_digest"] != receipt["receipt_digest"] {
			return nil, fail("run-abandonment-invalid")
		}
		receipt = stored
	} else if err = atomicState(root, []string{"active", id, "abandonment.json"}, receipt); err != nil {
		return nil, err
	}
	if _, err = detachActive(root, id, "abandoned"); err != nil {
		return nil, err
	}
	out := clone(receipt)
	out["reused"] = false
	return out, nil
}
