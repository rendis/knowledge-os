package syncflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var oidRE = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func arr(v any) []any          { a, _ := v.([]any); return a }
func str(v any) string         { s, _ := v.(string); return s }
func exact(m map[string]any, fields string) bool {
	ks := strings.Fields(fields)
	if m == nil || len(m) != len(ks) {
		return false
	}
	for _, k := range ks {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}
func one(v any, choices ...string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, c := range choices {
		if s == c {
			return true
		}
	}
	return false
}
func basename(v string) bool {
	if v == "" || strings.TrimSpace(v) != v || v == "." || v == ".." || strings.ContainsAny(v, "/\\") {
		return false
	}
	for _, r := range v {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func stable(v any, valid func(string) bool) bool {
	a, ok := v.([]any)
	if !ok {
		return false
	}
	prev := ""
	for i, x := range a {
		s, ok := x.(string)
		if !ok || !valid(s) || (i > 0 && s <= prev) {
			return false
		}
		prev = s
	}
	return true
}
func contains(a any, v any) bool {
	for _, x := range arr(a) {
		if reflect.DeepEqual(x, v) {
			return true
		}
	}
	return false
}
func subset(a, b any) bool {
	for _, x := range arr(a) {
		if !contains(b, x) {
			return false
		}
	}
	return true
}
func nonempty(s string) bool { return strings.TrimSpace(s) != "" }
func branch(s string) bool {
	if s == "" || s == "@" || s == "HEAD" || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "refs/") || strings.HasSuffix(s, ".") || strings.Contains(s, "..") || strings.Contains(s, "@{") {
		return false
	}
	for _, r := range s {
		if r <= 32 || r == 127 || strings.ContainsRune("~^:?*[\\", r) {
			return false
		}
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".lock") {
			return false
		}
	}
	return true
}

const recordFields = "repository verdict review_disposition result new_oid is_new declared_nodes affected_nodes write_nodes accepted_claim_ids rejected_claim_ids partial_accept analysis_fallback fallback_reason cursor_decision disposition"

// validateGate checks the persisted v2 write-authority shape, including connected
// write groups and their exact grants. It does not manufacture semantic approval.
func validateGate(g map[string]any) error {
	bad := func() error { return fail("gate-contract-invalid") }
	if !exact(g, "version code status repositories write_groups acknowledgements fallback_repositories pending_repositories") || g["version"] != json.Number("2") || g["code"] != "batch-gated" || !one(g["status"], "all-ready", "complete-no-write") || !reflect.DeepEqual(g["pending_repositories"], []any{}) {
		return bad()
	}
	repositories, ok := g["repositories"].([]any)
	if !ok {
		return bad()
	}
	records := map[string]any{}
	expectedFallback := map[string]bool{}
	cursor := map[string]any{}
	ready := []map[string]any{}
	prev := ""
	for _, v := range repositories {
		r := obj(v)
		n := str(r["repository"])
		if !exact(r, recordFields) || !basename(n) || n <= prev || !oidRE.MatchString(str(r["new_oid"])) || !one(r["verdict"], "accept", "revise", "blocked") || !one(r["result"], "documentation-change", "traceability-only", "blocked", "no-change") || !one(r["disposition"], "write-ready", "cursor-ready") {
			return bad()
		}
		prev = n
		records[n] = r
		for _, k := range []string{"accepted_claim_ids", "rejected_claim_ids"} {
			if !stable(r[k], nonempty) {
				return bad()
			}
		}
		for _, id := range arr(r["accepted_claim_ids"]) {
			if contains(r["rejected_claim_ids"], id) {
				return bad()
			}
		}
		for _, k := range []string{"declared_nodes", "affected_nodes", "write_nodes"} {
			if !stable(r[k], basename) {
				return bad()
			}
		}
		if !subset(r["write_nodes"], r["declared_nodes"]) {
			return bad()
		}
		for _, k := range []string{"is_new", "partial_accept", "analysis_fallback"} {
			if _, ok := r[k].(bool); !ok {
				return bad()
			}
		}
		for _, k := range []string{"fallback_reason", "cursor_decision", "review_disposition"} {
			if _, ok := r[k].(string); !ok {
				return bad()
			}
		}
		if r["cursor_decision"] == "no-documentation-change" && (r["is_new"] != false || r["result"] != "traceability-only" || r["verdict"] != "accept" || r["review_disposition"] != "accept" || r["analysis_fallback"] != false || r["fallback_reason"] != "" || len(arr(r["accepted_claim_ids"])) != 0 || r["disposition"] != "cursor-ready") {
			return bad()
		}
		if r["fallback_reason"] != "" {
			expectedFallback[n] = true
		}
		if r["disposition"] == "cursor-ready" {
			if len(arr(r["write_nodes"])) != 0 {
				return bad()
			}
			cursor[n] = r
		} else {
			if len(arr(r["write_nodes"])) == 0 {
				return bad()
			}
			ready = append(ready, r)
		}
	}
	groups, ok := g["write_groups"].([]any)
	if !ok {
		return bad()
	}
	expected := writeGroups(ready)
	if len(groups) != len(expected) {
		return bad()
	}
	for i, v := range groups {
		gr := obj(v)
		if !exact(gr, "group_id repositories nodes grants") || gr["group_id"] != expected[i]["group_id"] || !reflect.DeepEqual(gr["repositories"], expected[i]["repositories"]) || !reflect.DeepEqual(gr["nodes"], expected[i]["nodes"]) {
			return bad()
		}
		grants, ok := gr["grants"].([]any)
		if !ok || len(grants) == 0 {
			return bad()
		}
		ns := map[string]bool{}
		rs := map[string]bool{}
		prev := ""
		for _, v := range grants {
			a := obj(v)
			rn := str(a["repository"])
			id := str(a["claim_id"])
			r := obj(records[rn])
			key := rn + "\x00" + id
			if !exact(a, "repository claim_id nodes") || r == nil || !contains(gr["repositories"], rn) || !contains(r["accepted_claim_ids"], id) || contains(r["rejected_claim_ids"], id) || !stable(a["nodes"], basename) || len(arr(a["nodes"])) == 0 || !subset(a["nodes"], gr["nodes"]) || !subset(a["nodes"], r["write_nodes"]) || key <= prev {
				return bad()
			}
			prev = key
			rs[rn] = true
			for _, n := range arr(a["nodes"]) {
				ns[str(n)] = true
			}
		}
		if !reflect.DeepEqual(sortedKeys(ns), gr["nodes"]) || !reflect.DeepEqual(sortedKeys(rs), gr["repositories"]) {
			return bad()
		}
	}
	acks, ok := g["acknowledgements"].([]any)
	if !ok || len(acks) != len(cursor) {
		return bad()
	}
	prev = ""
	for _, v := range acks {
		a := obj(v)
		n := str(a["repository"])
		r := obj(cursor[n])
		if !exact(a, "repository new_oid decision branch analysis_date") || r == nil || n <= prev || a["new_oid"] != r["new_oid"] || a["decision"] != r["cursor_decision"] || !branch(str(a["branch"])) {
			return bad()
		}
		prev = n
		d := str(a["analysis_date"])
		dt, e := time.Parse("2006-01-02", d)
		if e != nil || dt.Format("2006-01-02") != d {
			return bad()
		}
	}
	if !reflect.DeepEqual(g["fallback_repositories"], sortedKeys(expectedFallback)) {
		return bad()
	}
	writes := len(groups) > 0 || len(acks) > 0
	if (g["status"] == "all-ready") != writes {
		return bad()
	}
	return nil
}
func writeGroups(records []map[string]any) []map[string]any {
	remaining := append([]map[string]any{}, records...)
	sort.Slice(remaining, func(i, j int) bool { return str(remaining[i]["repository"]) < str(remaining[j]["repository"]) })
	out := []map[string]any{}
	for len(remaining) > 0 {
		seed := remaining[0]
		remaining = remaining[1:]
		repos := map[string]bool{str(seed["repository"]): true}
		nodes := map[string]bool{}
		for _, n := range arr(seed["write_nodes"]) {
			nodes[str(n)] = true
		}
		for changed := true; changed; {
			changed = false
			for i := 0; i < len(remaining); {
				r := remaining[i]
				hit := false
				for _, n := range arr(r["write_nodes"]) {
					if nodes[str(n)] {
						hit = true
					}
				}
				if hit {
					repos[str(r["repository"])] = true
					for _, n := range arr(r["write_nodes"]) {
						nodes[str(n)] = true
					}
					remaining = append(remaining[:i], remaining[i+1:]...)
					changed = true
				} else {
					i++
				}
			}
		}
		out = append(out, map[string]any{"group_id": fmt.Sprintf("group-%03d", len(out)+1), "repositories": sortedKeys(repos), "nodes": sortedKeys(nodes)})
	}
	return out
}
func validatePackage(p map[string]any) error {
	bad := func() error { return fail("checkpoint-lineage-invalid") }
	if !exact(p, "version code status repository new_oid manifest scaffold analysis review gate validation") || p["version"] != json.Number("1") || p["code"] != "package-closed" || p["status"] != "complete" || !nameRE.MatchString(str(p["repository"])) || !oidRE.MatchString(str(p["new_oid"])) {
		return bad()
	}
	m, s, a, g, v, r := obj(p["manifest"]), obj(p["scaffold"]), obj(p["analysis"]), obj(p["gate"]), obj(p["validation"]), obj(p["review"])
	if !exact(m, "version old_oid new_oid paths environment_configs credential_suspects") || !exact(s, "version repository old_oid new_oid paths checklist claims nodes result blockers") || !exact(a, "version repository old_oid new_oid paths checklist claims nodes result blockers") || !exact(v, "manifest_digest scaffold_digest analysis_digest review_digest gate_digest") || p["review"] != nil && !exact(r, "version repository manifest_digest scaffold_digest analysis_digest verdict findings") {
		return bad()
	}
	for _, k := range []string{"manifest", "scaffold", "analysis", "review", "gate"} {
		d, e := digest(p[k])
		if e != nil || v[k+"_digest"] != d {
			return bad()
		}
	}
	if e := validateGate(g); e != nil {
		return e
	}
	rs := arr(g["repositories"])
	if len(rs) != 1 || obj(rs[0])["repository"] != p["repository"] || obj(rs[0])["new_oid"] != p["new_oid"] || m["new_oid"] != p["new_oid"] || a["repository"] != p["repository"] {
		return bad()
	}
	claims, ok := a["claims"].([]any)
	if !ok {
		return bad()
	}
	ids := map[string]bool{}
	for _, v := range claims {
		c := obj(v)
		e, ok := c["evidence"].([]any)
		if !exact(c, "claim_id statement evidence") || !nonempty(str(c["claim_id"])) || !nonempty(str(c["statement"])) || !ok || len(e) == 0 {
			return bad()
		}
		ids[str(c["claim_id"])] = true
	}
	for _, id := range arr(obj(rs[0])["accepted_claim_ids"]) {
		if !ids[str(id)] {
			return bad()
		}
	}
	if r != nil {
		if r["repository"] != p["repository"] {
			return bad()
		}
		for _, k := range []string{"manifest", "scaffold", "analysis"} {
			d, _ := digest(p[k])
			if r[k+"_digest"] != d {
				return bad()
			}
		}
	}
	if len(arr(g["write_groups"])) > 0 && r == nil {
		return bad()
	}
	return nil
}
