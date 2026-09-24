package syncflow

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/gofrs/flock"
)

const defaultStateRoot = ".agents/state/map-ecosystem/sync"
const runFields = "version run_id fingerprint tool_digest inventory_digest status packages gate_digest gate_stale units vault_locator"
const unitFields = "unit_id unit_type status repositories nodes stale_reason failed_patch_digests gate_digest projection_digest patch_digest base_files result_files base_kinds result_kinds note_review_digest receipt_digest"

func executableDigest() (string, error) {
	p, e := os.Executable()
	if e != nil {
		return "", e
	}
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
func fingerprint(tool, inventory string, packages []any) (string, error) {
	pairs := []any{}
	for _, p := range packages {
		m := obj(p)
		pairs = append(pairs, []any{m["repository"], m["oid"]})
	}
	return digest(map[string]any{"version": json.Number("1"), "tool_digest": tool, "inventory_digest": inventory, "packages": pairs})
}
func statePath(root string, parts ...string) (string, error) {
	if _, e := safe(root, true); e != nil {
		return "", e
	}
	p := root
	for _, part := range parts {
		if relative(part, false) != nil {
			return "", fail("state-path-invalid")
		}
		for _, segment := range strings.Split(part, "/") {
			p = filepath.Join(p, segment)
			s, e := os.Lstat(p)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return "", e
			}
			if s.Mode()&os.ModeSymlink != 0 {
				return "", fail("state-path-invalid")
			}
		}
	}
	return p, nil
}
func atomicState(root string, parts []string, v any) error {
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
	b, e := canonical(v)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".state-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, e = f.Write(append(b, '\n'))
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
	if e = os.Rename(tmp, p); e != nil {
		return e
	}
	return nil
}
func readState(root string, parts ...string) (map[string]any, error) {
	p, e := statePath(root, parts...)
	if e != nil {
		return nil, e
	}
	v, e := readJSON(p)
	if e != nil {
		return nil, e
	}
	m := obj(v)
	if m == nil {
		return nil, fail("invalid-json")
	}
	return m, nil
}
func readDigest(root string, want any, parts ...string) (map[string]any, error) {
	m, e := readState(root, parts...)
	if e != nil {
		return nil, e
	}
	d, e := digest(m)
	if e != nil || want != d {
		return nil, fail("checkpoint-digest-mismatch")
	}
	return m, nil
}
func unitProof(root, id string, u map[string]any) error {
	parts := []string{"active", id, "units", str(u["unit_id"])}
	p, e := readDigest(root, u["projection_digest"], append(parts, "projection.json")...)
	if e != nil {
		return e
	}
	if p["gate_digest"] != u["gate_digest"] {
		return fail("unit-checkpoint-invalid")
	}
	file, e := statePath(root, append(parts, "unit.patch")...)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(file)
	if e != nil || hash(b) != u["patch_digest"] {
		return fail("unit-checkpoint-invalid")
	}
	if u["note_review_digest"] != "" {
		r, e := readDigest(root, u["note_review_digest"], append(parts, "note-review.json")...)
		if e != nil {
			return e
		}
		if !exact(r, "version code status run_id unit_id gate_digest projection_digest manifest_digest review_digest") || r["version"] != json.Number("1") || r["code"] != "note-unit-reviewed" || r["status"] != "pass" || r["run_id"] != id || r["unit_id"] != u["unit_id"] || r["gate_digest"] != u["gate_digest"] || r["projection_digest"] != u["projection_digest"] || !digestRE.MatchString(str(r["manifest_digest"])) || !digestRE.MatchString(str(r["review_digest"])) {
			return fail("unit-checkpoint-invalid")
		}
	}
	if _, e = readDigest(root, u["gate_digest"], "active", id, "gates", str(u["gate_digest"])+".json"); e != nil {
		return e
	}
	if u["status"] == "applied" {
		_, e = readDigest(root, u["receipt_digest"], append(parts, "receipt.json")...)
	}
	return e
}
func loadRun(root, id string) (map[string]any, error) {
	if !nameRE.MatchString(id) {
		return nil, fail("invalid-run-id")
	}
	r, e := readState(root, "active", id, "run.json")
	if e != nil {
		return nil, e
	}
	if !exact(r, runFields) || r["version"] != json.Number("1") || r["run_id"] != id || !one(r["status"], "packages", "gated", "projecting", "closing") {
		return nil, fail("run-state-invalid")
	}
	for _, k := range []string{"fingerprint", "tool_digest", "inventory_digest"} {
		if !digestRE.MatchString(str(r[k])) {
			return nil, fail("run-state-invalid")
		}
	}
	if _, ok := r["gate_stale"].(bool); !ok {
		return nil, fail("run-state-invalid")
	}
	if _, ok := r["vault_locator"].(string); !ok {
		return nil, fail("run-state-invalid")
	}
	if d, ok := r["gate_digest"].(string); !ok || d != "" && !digestRE.MatchString(d) {
		return nil, fail("run-state-invalid")
	}
	missingPackages := map[string]bool{}
	ps, ok := r["packages"].([]any)
	if !ok {
		return nil, fail("run-state-invalid")
	}
	prev := ""
	for _, v := range ps {
		p := obj(v)
		n := str(p["repository"])
		d, ok := p["artifact_digest"].(string)
		count, okn := p["checkpoint_count"].(json.Number)
		num, e := count.Int64()
		if !exact(p, "repository oid status artifact_digest checkpoint_count") || !nameRE.MatchString(n) || n <= prev || !oidRE.MatchString(str(p["oid"])) || !one(p["status"], "pending", "checkpointed", "stale") || !ok || d != "" && !digestRE.MatchString(d) || !okn || e != nil || num < 0 {
			return nil, fail("run-state-invalid")
		}
		prev = n
		if p["status"] == "checkpointed" {
			a, e := readDigest(root, d, "active", id, "packages", n, "artifact.json")
			if e != nil {
				if os.IsNotExist(e) {
					p["status"] = "stale"
					p["artifact_digest"] = ""
					missingPackages[n] = true
					continue
				}
				return nil, fmt.Errorf("package-checkpoint-invalid: %w", e)
			}
			if sensitive(a) || validatePackage(a) != nil || a["repository"] != n || a["new_oid"] != p["oid"] {
				return nil, fail("package-checkpoint-invalid")
			}
		}
	}
	fp, e := fingerprint(str(r["tool_digest"]), str(r["inventory_digest"]), ps)
	if e != nil || r["fingerprint"] != fp {
		return nil, fail("run-state-invalid")
	}
	us, ok := r["units"].([]any)
	if !ok {
		return nil, fail("run-state-invalid")
	}
	seen := map[string]bool{}
	for _, v := range us {
		u := obj(v)
		if exact(u, strings.Replace(unitFields, " note_review_digest", "", 1)) {
			u["note_review_digest"] = ""
		}
		n := str(u["unit_id"])
		if !exact(u, unitFields) || !nameRE.MatchString(n) || seen[n] || !one(u["unit_type"], "acknowledgements", "write-group") || !one(u["status"], "pending", "validated", "applied", "projection-invalid", "apply-failed", "stale") || !one(u["stale_reason"], "", "source", "source-retracted", "destination") {
			return nil, fail("run-state-invalid")
		}
		seen[n] = true
		if !stable(u["repositories"], nameRE.MatchString) || !stable(u["nodes"], basename) || !stable(u["failed_patch_digests"], digestRE.MatchString) {
			return nil, fail("run-state-invalid")
		}
		for _, k := range []string{"gate_digest", "projection_digest", "patch_digest", "note_review_digest", "receipt_digest"} {
			d, ok := u[k].(string)
			if !ok || d != "" && !digestRE.MatchString(d) {
				return nil, fail("run-state-invalid")
			}
		}
		for _, k := range []string{"base_files", "result_files"} {
			m := obj(u[k])
			if m == nil {
				return nil, fail("run-state-invalid")
			}
			for path, d := range m {
				if relative(path, false) != nil || !digestRE.MatchString(str(d)) {
					return nil, fail("run-state-invalid")
				}
			}
		}
		for _, k := range []string{"base_kinds", "result_kinds"} {
			m := obj(u[k])
			if m == nil || !reflect.DeepEqual(sortedKeys(m), sortedKeys(obj(u["base_files"]))) {
				return nil, fail("run-state-invalid")
			}
			for _, v := range m {
				if !one(v, "file", "missing") {
					return nil, fail("run-state-invalid")
				}
			}
		}
		for repo := range missingPackages {
			if contains(u["repositories"], repo) {
				u["status"] = "stale"
				u["stale_reason"] = "source"
			}
		}
		if one(u["status"], "validated", "applied", "apply-failed") {
			if e = unitProof(root, id, u); e != nil {
				return nil, e
			}
		}
	}
	if r["gate_digest"] != "" {
		g, e := readDigest(root, r["gate_digest"], "active", id, "gate.json")
		if e != nil {
			g, e = readDigest(root, r["gate_digest"], "active", id, "gates", str(r["gate_digest"])+".json")
			if e == nil {
				e = atomicState(root, []string{"active", id, "gate.json"}, g)
			}
		}
		if e != nil {
			return nil, e
		}
		if e = validateGate(g); e != nil {
			return nil, e
		}
	}
	if len(missingPackages) > 0 {
		r["gate_stale"] = true
		r["status"] = "packages"
		if e = saveRun(root, r); e != nil {
			return nil, e
		}
	}
	return r, nil
}
func next(r map[string]any) (string, map[string]any) {
	for _, v := range arr(r["units"]) {
		u := obj(v)
		if u["status"] == "stale" && u["stale_reason"] == "source" && u["receipt_digest"] != "" {
			return "resume", nil
		}
	}
	for _, v := range arr(r["packages"]) {
		p := obj(v)
		if one(p["status"], "pending", "stale") {
			return "checkpoint-package", p
		}
	}
	if r["gate_stale"] == true || r["gate_digest"] == "" {
		return "seal-gate", nil
	}
	for _, v := range arr(r["units"]) {
		u := obj(v)
		if one(u["status"], "pending", "projection-invalid", "stale") {
			return "validate-unit", u
		}
	}
	for _, v := range arr(r["units"]) {
		u := obj(v)
		if u["unit_type"] == "write-group" && one(u["status"], "validated", "apply-failed") && u["note_review_digest"] == "" {
			return "review-unit", u
		}
	}
	for _, v := range arr(r["units"]) {
		u := obj(v)
		if one(u["status"], "validated", "apply-failed") {
			return "apply-unit", u
		}
	}
	return "close", nil
}
func receiptView(r map[string]any) map[string]any {
	v := map[string]any{}
	for _, k := range strings.Fields("version run_id fingerprint tool_digest inventory_digest status gate_digest") {
		v[k] = r[k]
	}
	for _, spec := range []struct{ k, fields string }{{"packages", "repository oid status artifact_digest"}, {"units", "unit_id unit_type status gate_digest projection_digest patch_digest note_review_digest receipt_digest"}} {
		a := []any{}
		for _, x := range arr(r[spec.k]) {
			m := map[string]any{}
			for _, k := range strings.Fields(spec.fields) {
				m[k] = obj(x)[k]
			}
			a = append(a, m)
		}
		v[spec.k] = a
	}
	return v
}
func publicStatus(root string, r map[string]any) map[string]any {
	v := receiptView(r)
	d, _ := digest(v)
	command, target := next(r)
	argv := []any{command, "--state-root", root, "--run-id", r["run_id"]}
	missing := []any{}
	action := map[string]any{"command": command}
	switch command {
	case "checkpoint-package":
		action["repository"] = target["repository"]
		action["oid"] = target["oid"]
		argv = append(argv, "--repository", target["repository"])
		missing = append(missing, "--artifact")
	case "seal-gate":
		missing = append(missing, "--gate")
	case "validate-unit", "review-unit", "apply-unit":
		action["unit_id"] = target["unit_id"]
		argv = append(argv, "--unit-id", target["unit_id"])
		if command == "validate-unit" {
			missing = append(missing, "--projection", "--patch")
		} else {
			if command == "review-unit" {
				missing = append(missing, "--candidate", "--evidence-root", "--manifest", "--review")
			}
			loc := str(r["vault_locator"])
			vault := filepath.Clean(filepath.Join(root, filepath.FromSlash(loc)))
			if loc != "" {
				if p, e := safe(vault, true); e == nil {
					argv = append(argv, "--vault", p)
				} else {
					missing = append(missing, "--vault")
				}
			} else {
				missing = append(missing, "--vault")
			}
		}
	}
	action["argv"] = argv
	action["missing_inputs"] = missing
	checkpointed, invalidated := 0, 0
	for _, v := range arr(r["packages"]) {
		switch obj(v)["status"] {
		case "checkpointed":
			checkpointed++
		case "stale":
			invalidated++
		}
	}
	units := []any{}
	for _, v := range arr(r["units"]) {
		u := obj(v)
		m := map[string]any{}
		for _, k := range strings.Fields("unit_id unit_type status stale_reason repositories nodes gate_digest projection_digest patch_digest note_review_digest receipt_digest") {
			m[k] = u[k]
		}
		dir := filepath.Join(root, "active", str(r["run_id"]), "units", str(u["unit_id"]))
		m["projection_path"] = filepath.Join(dir, "projection.json")
		m["patch_path"] = filepath.Join(dir, "unit.patch")
		units = append(units, m)
	}
	return map[string]any{"version": json.Number("1"), "code": "run-status", "status": r["status"], "run_id": r["run_id"], "gate_digest": r["gate_digest"], "packages": v["packages"], "units": units, "package_counters": map[string]any{"checkpointed": checkpointed, "expected": len(arr(r["packages"])), "invalidated": invalidated}, "receipt_digest": d, "next_command": command, "next_action": action}
}

var sensitiveToken = regexp.MustCompile(`(?:gh[pousr]_[A-Za-z0-9]{10,}|xox[baprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|Bearer\s+[A-Za-z0-9._~-]{10,}|sk-(?:proj-)?[A-Za-z0-9_-]{20,})`)
var assignment = regexp.MustCompile(`(?i)(?:password|passwd|secret|token|api[_-]?key|private[_-]?key)\s*[:=]\s*['"]?[^\s'"]+`)
var localPath = regexp.MustCompile(`/(?:Users|home/[^/[:space:]]+|private/(?:tmp|var)|tmp|var/folders|Volumes)/`)

func sensitive(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for _, v := range x {
			if sensitive(v) {
				return true
			}
		}
	case []any:
		for _, v := range x {
			if sensitive(v) {
				return true
			}
		}
	case string:
		return strings.Contains(x, "-----BEGIN ") || sensitiveToken.MatchString(x) || sensitiveAssignment(x) || localPath.MatchString(x)
	}
	return false
}
func runState(args []string, out io.Writer) error {
	cmd := args[0]
	if cmd == "tool-digest" {
		if len(args) != 1 {
			return fail("unexpected-arguments")
		}
		d, e := executableDigest()
		if e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(map[string]any{"tool_digest": d})
	}
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	root := f.String("state-root", defaultStateRoot, "")
	id := f.String("run-id", "", "")
	inventory := f.String("inventory-digest", "", "")
	tool := f.String("tool-digest", "", "")
	repo := f.String("repository", "", "")
	artifact := f.String("artifact", "", "")
	gatePath := f.String("gate", "", "")
	unitID := f.String("unit-id", "", "")
	projection := f.String("projection", "", "")
	patch := f.String("patch", "", "")
	vault := f.String("vault", "", "")
	candidate := f.String("candidate", "", "")
	evidence := f.String("evidence-root", "", "")
	manifestPath := f.String("manifest", "", "")
	reviewPath := f.String("review", "", "")
	failWrites := f.Int("fail-after-writes", 0, "")
	crashWrites := f.Int("crash-after-writes", 0, "")
	crashBytes := f.Bool("crash-after-bytes", false, "")
	var sources, destinations repeated
	f.Var(&sources, "source-oid", "REPOSITORY=OID")
	f.Var(&destinations, "destination-digest", "PATH=DIGEST")
	var packages repeated
	f.Var(&packages, "package", "REPOSITORY=OID")
	// Preserve the legacy repeatable two-argument package spelling.
	normalized := []string{}
	for i := 1; i < len(args); i++ {
		if one(args[i], "--package", "--source-oid", "--destination-digest") && i+2 < len(args) && !strings.Contains(args[i+1], "=") {
			normalized = append(normalized, args[i], args[i+1]+"="+args[i+2])
			i += 2
		} else {
			normalized = append(normalized, args[i])
		}
	}
	if e := f.Parse(normalized); e != nil {
		return e
	}
	if f.NArg() > 0 {
		return fail("unexpected-arguments")
	}
	if cmd == "begin" {
		abs, e := filepath.Abs(*root)
		if e != nil {
			return e
		}
		*root = abs
		parent := abs
		for {
			if _, e = os.Lstat(parent); e == nil {
				break
			} else if !os.IsNotExist(e) {
				return e
			}
			next := filepath.Dir(parent)
			if next == parent {
				return fail("state-path-invalid")
			}
			parent = next
		}
		if _, e = safe(parent, true); e != nil {
			return e
		}
		if e = os.MkdirAll(abs, 0700); e != nil {
			return e
		}
	}
	var e error
	*root, e = safe(*root, true)
	if e != nil {
		return e
	}
	lockPath, e := statePath(*root, ".native.lock")
	if e != nil {
		return e
	}
	lock := flock.New(lockPath)
	ok, e := lock.TryLock()
	if e != nil {
		return e
	}
	if !ok {
		return fail("sync-state-busy")
	}
	defer lock.Unlock()
	var result map[string]any
	if cmd == "begin" {
		result, e = beginRun(*root, *inventory, *tool, packages)
		if e != nil {
			return e
		}
	} else {
		if cmd == "status" || cmd == "close" {
			p, e := statePath(*root, "receipts", *id+".json")
			if e != nil {
				return e
			}
			if _, e = os.Lstat(p); e == nil {
				receipt, e := validateClosedReceipt(*root, p, *id, "")
				if e != nil {
					return e
				}
				if cmd == "close" {
					if e = retireClosedActive(*root, receipt); e != nil {
						return e
					}
				}
				return json.NewEncoder(out).Encode(receipt)
			} else if !os.IsNotExist(e) {
				return e
			}
		}
		r, e := loadRun(*root, *id)
		if e != nil {
			return e
		}
		if cmd == "status" {
			result = publicStatus(*root, r)
		} else {
			d, e := executableDigest()
			if e != nil {
				return e
			}
			if r["tool_digest"] != d {
				return fail("run-version-mismatch")
			}
			if *tool != "" && *tool != str(r["tool_digest"]) {
				return fail("run-version-mismatch")
			}
			switch cmd {
			case "validate-unit":
				result, e = validateUnit(*root, r, *unitID, *projection, *patch)
			case "review-unit":
				result, e = reviewUnit(*root, r, *unitID, *vault, *candidate, *evidence, *manifestPath, *reviewPath)
			case "apply-unit":
				result, e = applyUnit(*root, r, *unitID, *vault, *failWrites, *crashWrites, *crashBytes)
			case "resume":
				src, e1 := pairMap(sources)
				dst, e2 := pairMap(destinations)
				if e1 != nil {
					return e1
				}
				if e2 != nil {
					return e2
				}
				result, e = resumeRun(*root, r, src, dst)
			case "close":
				result, e = closeRun(*root, r)
			case "checkpoint-package":
				var target map[string]any
				for _, v := range arr(r["packages"]) {
					if obj(v)["repository"] == *repo {
						target = obj(v)
					}
				}
				if target == nil {
					return fail("package-not-found")
				}
				a, e := readJSON(*artifact)
				if e != nil {
					return e
				}
				if sensitive(a) {
					return fail("checkpoint-sensitive-content")
				}
				if e = validatePackage(obj(a)); e != nil {
					return e
				}
				p := obj(a)
				if p["repository"] != *repo || p["new_oid"] != target["oid"] {
					return fail("package-identity-mismatch")
				}
				dg, _ := digest(p)
				reused := target["status"] == "checkpointed" && target["artifact_digest"] == dg
				if target["status"] == "checkpointed" && !reused {
					return fail("package-checkpoint-mismatch")
				}
				parts := []string{"active", *id, "packages", *repo, "artifact.json"}
				if !reused {
					if e = atomicState(*root, parts, p); e != nil {
						return e
					}
					target["status"] = "checkpointed"
					target["artifact_digest"] = dg
					n, _ := target["checkpoint_count"].(json.Number).Int64()
					target["checkpoint_count"] = json.Number(fmt.Sprint(n + 1))
					if e = atomicState(*root, []string{"active", *id, "run.json"}, r); e != nil {
						return e
					}
				}
				path, _ := statePath(*root, parts...)
				result = map[string]any{"version": json.Number("1"), "code": "package-checkpointed", "status": "pass", "run_id": *id, "repository": *repo, "artifact_digest": dg, "artifact_path": path, "reused": reused}
			case "seal-gate":
				result, e = seal(*root, r, *gatePath)
				if e != nil {
					return e
				}
			default:
				return fail("unsupported-sync-operation")
			}
			if e != nil {
				return e
			}
		}
	}
	if e != nil {
		return e
	}
	if e = json.NewEncoder(out).Encode(result); e != nil {
		return e
	}
	if result["status"] == "blocked" {
		return fail(str(result["code"]))
	}
	return nil
}
func newUnit(id, kind string, repos, nodes any) map[string]any {
	return map[string]any{"unit_id": id, "unit_type": kind, "status": "pending", "stale_reason": "", "repositories": repos, "nodes": nodes, "failed_patch_digests": []any{}, "gate_digest": "", "projection_digest": "", "patch_digest": "", "base_files": map[string]any{}, "result_files": map[string]any{}, "base_kinds": map[string]any{}, "result_kinds": map[string]any{}, "note_review_digest": "", "receipt_digest": ""}
}
func seal(root string, r map[string]any, path string) (map[string]any, error) {
	for _, p := range arr(r["packages"]) {
		if obj(p)["status"] != "checkpointed" {
			return nil, fail("package-checkpoint-required")
		}
	}
	for _, v := range arr(r["units"]) {
		u := obj(v)
		if u["status"] == "stale" && u["stale_reason"] == "source" && u["receipt_digest"] != "" {
			return nil, fail("stale-retraction-required")
		}
	}
	v, e := readJSON(path)
	if e != nil {
		return nil, e
	}
	g := obj(v)
	if e = validateGate(g); e != nil {
		return nil, e
	}
	dg, _ := digest(g)
	if r["gate_digest"] != "" {
		if r["gate_digest"] == dg && r["gate_stale"] == false {
			out := publicStatus(root, r)
			out["code"] = "gate-sealed"
			out["reused"] = true
			return out, nil
		}
		if r["gate_stale"] != true {
			return nil, fail("gate-already-sealed")
		}
	}
	if len(arr(g["repositories"])) != len(arr(r["packages"])) {
		return nil, fail("gate-inventory-mismatch")
	}
	id := str(r["run_id"])
	for i, p := range arr(r["packages"]) {
		pk := obj(p)
		rec := obj(arr(g["repositories"])[i])
		if rec["repository"] != pk["repository"] || rec["new_oid"] != pk["oid"] {
			return nil, fail("gate-inventory-mismatch")
		}
		a, e := readState(root, "active", id, "packages", str(pk["repository"]), "artifact.json")
		if e != nil {
			return nil, e
		}
		pg := obj(a["gate"])
		if !reflect.DeepEqual(rec, arr(pg["repositories"])[0]) {
			return nil, fail("gate-authority-unbound")
		}
		for _, field := range []string{"grants", "acknowledgements"} {
			collect := func(g map[string]any) []any {
				out := []any{}
				list := arr(g["acknowledgements"])
				if field == "grants" {
					list = []any{}
					for _, gr := range arr(g["write_groups"]) {
						list = append(list, arr(obj(gr)["grants"])...)
					}
				}
				for _, v := range list {
					if obj(v)["repository"] == pk["repository"] {
						out = append(out, v)
					}
				}
				return out
			}
			if !reflect.DeepEqual(collect(g), collect(pg)) {
				return nil, fail("gate-authority-unbound")
			}
		}
	}
	units := []any{}
	if len(arr(g["acknowledgements"])) > 0 {
		repos := []any{}
		for _, a := range arr(g["acknowledgements"]) {
			repos = append(repos, obj(a)["repository"])
		}
		units = append(units, newUnit("acknowledgements", "acknowledgements", repos, []any{}))
	}
	for _, v := range arr(g["write_groups"]) {
		gr := obj(v)
		units = append(units, newUnit(str(gr["group_id"]), "write-group", gr["repositories"], gr["nodes"]))
	}
	if r["gate_stale"] == true && r["gate_digest"] != "" {
		oldGate, e := readState(root, "active", id, "gates", str(r["gate_digest"])+".json")
		if e != nil {
			return nil, e
		}
		for i, v := range units {
			fresh := obj(v)
			candidates := []map[string]any{}
			for _, p := range arr(r["units"]) {
				previous := obj(p)
				if one(previous["status"], "validated", "apply-failed", "applied") && sameAuthority(previous, fresh, oldGate, g) {
					candidates = append(candidates, previous)
				}
			}
			if len(candidates) == 1 {
				u, e := rebind(root, id, candidates[0], fresh, dg)
				if e != nil {
					return nil, e
				}
				units[i] = u
			}
		}
	}
	if e = atomicState(root, []string{"active", id, "gates", dg + ".json"}, g); e != nil {
		return nil, e
	}
	r["gate_digest"] = dg
	r["gate_stale"] = false
	r["units"] = units
	r["status"] = "gated"
	for _, v := range units {
		u := obj(v)
		if u["status"] == "applied" && u["receipt_digest"] == "" {
			if e = writeUnitReceipt(root, r, u); e != nil {
				return nil, e
			}
		}
	}
	if e = atomicState(root, []string{"active", id, "run.json"}, r); e != nil {
		return nil, e
	}
	if e = atomicState(root, []string{"active", id, "gate.json"}, g); e != nil {
		return nil, e
	}
	out := publicStatus(root, r)
	out["code"] = "gate-sealed"
	out["reused"] = false
	return out, nil
}

func pairMap(values []string) (map[string]string, error) {
	m := map[string]string{}
	for _, value := range values {
		k, v, ok := strings.Cut(value, "=")
		if !ok || k == "" || v == "" {
			return nil, fail("invalid-pair")
		}
		if _, exists := m[k]; exists {
			return nil, fail("duplicate-pair")
		}
		m[k] = v
	}
	return m, nil
}

var permissionRE = regexp.MustCompile(`^id-token:[ \t]*(?:write|read|none)`)

func sensitiveAssignment(text string) bool {
	for offset := 0; offset < len(text); {
		match := assignment.FindStringIndex(text[offset:])
		if match == nil {
			return false
		}
		start := offset + match[0]
		allowed := false
		if start >= 3 && text[start-3:start] == "id-" {
			begin := start - 3
			boundary := begin == 0
			if !boundary {
				c := text[begin-1]
				boundary = !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_.-", rune(c)))
			}
			if boundary {
				p := permissionRE.FindStringIndex(text[begin:])
				if p != nil {
					end := begin + p[1]
					allowed = end == len(text) || strings.ContainsRune(" \t\r\n`\"',;)}]", rune(text[end]))
				}
			}
		}
		if !allowed {
			return true
		}
		offset = start + 1
	}
	return false
}
