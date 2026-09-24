package syncflow

import (
	"documentation-vault/internal/config"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

func groupsWithGrants(records []map[string]any) []any {
	groups := writeGroups(records)
	result := []any{}
	for _, g := range groups {
		grants := []any{}
		for _, r := range records {
			if contains(g["repositories"], r["repository"]) {
				grants = append(grants, arr(r["grants"])...)
			}
		}
		sort.Slice(grants, func(i, j int) bool {
			a, b := obj(grants[i]), obj(grants[j])
			return str(a["repository"])+"\x00"+str(a["claim_id"]) < str(b["repository"])+"\x00"+str(b["claim_id"])
		})
		g["grants"] = grants
		result = append(result, g)
	}
	return result
}
func gateBatch(packages []map[string]any, expected []string) (map[string]any, error) {
	names := map[string]bool{}
	for _, n := range expected {
		if !basename(n) || names[n] {
			return nil, fail("invalid-expected-repository")
		}
		names[n] = true
	}
	if len(names) == 0 {
		return nil, fail("invalid-expected-repository")
	}
	records := []map[string]any{}
	acks := []any{}
	fallback := map[string]bool{}
	seen := map[string]bool{}
	for _, p := range packages {
		if e := validatePackage(p); e != nil {
			return nil, e
		}
		n := str(p["repository"])
		if seen[n] || !names[n] {
			return nil, fail("batch-repository-coverage-mismatch")
		}
		seen[n] = true
		g := obj(p["gate"])
		r := clone(obj(arr(g["repositories"])[0]))
		grants := []any{}
		for _, gr := range arr(g["write_groups"]) {
			for _, v := range arr(obj(gr)["grants"]) {
				if obj(v)["repository"] == n {
					grants = append(grants, v)
				}
			}
		}
		r["grants"] = grants
		records = append(records, r)
		acks = append(acks, arr(g["acknowledgements"])...)
		for _, v := range arr(g["fallback_repositories"]) {
			fallback[str(v)] = true
		}
	}
	if len(seen) != len(names) {
		return nil, fail("batch-repository-coverage-mismatch")
	}
	sort.Slice(records, func(i, j int) bool { return str(records[i]["repository"]) < str(records[j]["repository"]) })
	sort.Slice(acks, func(i, j int) bool { return str(obj(acks[i])["repository"]) < str(obj(acks[j])["repository"]) })
	ready := []map[string]any{}
	repositories := []any{}
	for _, r := range records {
		if r["disposition"] == "write-ready" {
			ready = append(ready, r)
		}
		v := clone(r)
		delete(v, "grants")
		repositories = append(repositories, v)
	}
	status := "complete-no-write"
	if len(ready) > 0 || len(acks) > 0 {
		status = "all-ready"
	}
	g := map[string]any{"version": json.Number("2"), "code": "batch-gated", "status": status, "repositories": repositories, "write_groups": groupsWithGrants(ready), "acknowledgements": acks, "fallback_repositories": sortedKeys(fallback), "pending_repositories": []any{}}
	if e := validateGate(g); e != nil {
		return nil, e
	}
	return g, nil
}
func gitRef(repo, ref string) (string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsRune(ref, 0) {
		return "", fail("invalid-current-ref")
	}
	b, e := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}").Output()
	if e != nil {
		return "", fail("git-ref-unavailable")
	}
	s := strings.TrimSpace(string(b))
	if !oidRE.MatchString(s) {
		return "", fail("git-ref-invalid")
	}
	return s, nil
}
func gateFromSource(repo string, m, s, a, r map[string]any, currentRef, branchName, date string) (map[string]any, error) {
	if e := checkAnalysis(repo, m, s, a, currentRef); e != nil {
		return nil, e
	}
	var e error
	repository := str(a["repository"])
	reviewFailure := ""
	fallback := fallbackAnalysisPayload(s, m)
	analysisFallback := !emptyNewManifest(m) && reflect.DeepEqual(a, fallback)
	if analysisFallback {
		reviewFailure = "analysis-fallback"
	} else if r == nil {
		reviewFailure = "review-missing"
	} else if e = validateAnalysisReview(repo, m, s, a, r); e != nil {
		reviewFailure = "review-invalid"
	}
	claimIDs := map[string]bool{}
	for _, v := range arr(a["claims"]) {
		claimIDs[str(obj(v)["claim_id"])] = true
	}
	rejected := map[string]bool{}
	partial := false
	if reviewFailure == "" && r["verdict"] == "revise" {
		valid := len(arr(r["findings"])) > 0
		for _, v := range arr(r["findings"]) {
			target := str(obj(v)["target"])
			if !strings.HasPrefix(target, "claims.") || !claimIDs[strings.TrimPrefix(target, "claims.")] {
				valid = false
				break
			}
			rejected[strings.TrimPrefix(target, "claims.")] = true
		}
		if valid {
			partial = len(rejected) < len(claimIDs)
		} else {
			rejected = map[string]bool{}
		}
	}
	accepted := map[string]bool{}
	if reviewFailure == "" {
		if r["verdict"] == "accept" {
			for id := range claimIDs {
				accepted[id] = true
			}
		} else if partial {
			for id := range claimIDs {
				if !rejected[id] {
					accepted[id] = true
				}
			}
		}
	}
	grantNodes := map[string]map[string]bool{}
	declared := map[string]bool{}
	for _, v := range arr(a["nodes"]) {
		n := obj(v)
		name := str(n["basename"])
		declared[name] = true
		if n["action"] == "no-change" {
			continue
		}
		for _, c := range arr(n["claim_ids"]) {
			id := str(c)
			if accepted[id] {
				if grantNodes[id] == nil {
					grantNodes[id] = map[string]bool{}
				}
				grantNodes[id][name] = true
			}
		}
	}
	grants := []any{}
	writeNodes := map[string]bool{}
	for _, v := range sortedKeys(grantNodes) {
		id := str(v)
		for n := range grantNodes[id] {
			writeNodes[n] = true
		}
		grants = append(grants, map[string]any{"repository": repository, "claim_id": id, "nodes": sortedKeys(grantNodes[id])})
	}
	affected := map[string]bool{}
	if reviewFailure == "" {
		for _, v := range arr(r["findings"]) {
			for _, n := range arr(obj(v)["nodes"]) {
				affected[str(n)] = true
			}
		}
	}
	verdict := str(r["verdict"])
	reviewDisposition := verdict
	if reviewFailure != "" {
		verdict = "blocked"
		reviewDisposition = "blocked"
	} else if partial {
		reviewDisposition = "partial-accept"
	}
	cursorDecision := ""
	if reviewFailure != "" {
		cursorDecision = "inspection-limited"
	} else if verdict == "revise" && !partial {
		reviewFailure = "review-revise"
		cursorDecision = "review-rejected"
	} else if verdict == "blocked" {
		reviewFailure = "review-limited"
		cursorDecision = "inspection-limited"
	}
	disposition := "write-ready"
	if cursorDecision != "" {
		affected = map[string]bool{}
		writeNodes = map[string]bool{}
		disposition = "cursor-ready"
	} else if len(writeNodes) == 0 {
		disposition = "cursor-ready"
		cursorDecision = "no-documentation-change"
		if isNewManifest(m) {
			cursorDecision = "no-durable-node"
		}
	}
	record := map[string]any{"repository": repository, "verdict": verdict, "review_disposition": reviewDisposition, "result": a["result"], "new_oid": m["new_oid"], "is_new": isNewManifest(m), "declared_nodes": sortedKeys(declared), "affected_nodes": sortedKeys(affected), "write_nodes": sortedKeys(writeNodes), "accepted_claim_ids": sortedKeys(accepted), "rejected_claim_ids": sortedKeys(rejected), "partial_accept": partial, "analysis_fallback": analysisFallback, "fallback_reason": reviewFailure, "cursor_decision": cursorDecision, "disposition": disposition}
	acks := []any{}
	ready := []map[string]any{}
	if disposition == "cursor-ready" {
		acks = append(acks, map[string]any{"repository": repository, "new_oid": m["new_oid"], "decision": cursorDecision, "branch": branchName, "analysis_date": date})
	} else {
		rr := clone(record)
		rr["grants"] = grants
		ready = append(ready, rr)
	}
	fallbackRepos := []any{}
	if reviewFailure != "" {
		fallbackRepos = append(fallbackRepos, repository)
	}
	g := map[string]any{"version": json.Number("2"), "code": "batch-gated", "status": "all-ready", "repositories": []any{record}, "write_groups": groupsWithGrants(ready), "acknowledgements": acks, "fallback_repositories": fallbackRepos, "pending_repositories": []any{}}
	if e = validateGate(g); e != nil {
		return nil, e
	}
	return g, nil
}
func closePackage(repo string, m, s, a, r map[string]any, productionRef, date, root string) (map[string]any, error) {
	parsed, e := time.Parse("2006-01-02", date)
	if e != nil || parsed.Format("2006-01-02") != date {
		return nil, fail("package-invalid")
	}
	var branchName string
	for _, prefix := range []string{"refs/heads/", "refs/remotes/origin/"} {
		if strings.HasPrefix(productionRef, prefix) {
			branchName = strings.TrimPrefix(productionRef, prefix)
		}
	}
	if !branch(branchName) {
		return nil, fail("package-invalid")
	}
	allowed := []string{"main", "master"}
	if root != "" {
		inst, e := config.LoadInstance(root)
		if e != nil {
			return nil, e
		}
		v := obj(obj(inst["sources"])["reference_branches"])[str(a["repository"])]
		if v != nil {
			allowed = []string{str(v)}
		}
	}
	ok := false
	for _, v := range allowed {
		if branchName == v {
			ok = true
		}
	}
	if !ok {
		return nil, fail("package-reference-policy-mismatch")
	}
	oid, e := gitRef(repo, productionRef)
	if e != nil || oid != m["new_oid"] {
		return nil, fail("package-source-stale")
	}
	g, e := gateFromSource(repo, m, s, a, r, productionRef, branchName, date)
	if e != nil {
		return nil, e
	}
	p := map[string]any{"version": json.Number("1"), "code": "package-closed", "status": "complete", "repository": a["repository"], "new_oid": m["new_oid"], "manifest": m, "scaffold": s, "analysis": a, "review": r, "gate": g}
	if r == nil {
		p["review"] = nil
	}
	validation := map[string]any{}
	for _, k := range []string{"manifest", "scaffold", "analysis", "review", "gate"} {
		d, e := digest(p[k])
		if e != nil {
			return nil, e
		}
		validation[k+"_digest"] = d
	}
	p["validation"] = validation
	if e = validatePackage(p); e != nil {
		return nil, e
	}
	return p, nil
}
func noClobberJSON(path string, v any) error {
	parent, e := safe(filepath.Dir(path), true)
	if e != nil {
		return e
	}
	p := filepath.Join(parent, filepath.Base(path))
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	f, e := os.CreateTemp(parent, ".sync-artifact-*")
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
	if e = os.Link(f.Name(), p); os.IsExist(e) {
		old, readErr := readJSON(p)
		if readErr == nil && reflect.DeepEqual(old, v) {
			return nil
		}
	}
	return e
}
func runGate(args []string, out io.Writer) error {
	cmd := args[0]
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	repo := f.String("repo", "", "")
	mp := f.String("manifest", "", "")
	sp := f.String("scaffold", "", "")
	ap := f.String("analysis", "", "")
	rp := f.String("review", "", "")
	ref := f.String("production-ref", "", "")
	date := f.String("analysis-date", "", "")
	root := f.String("root", "", "")
	output := f.String("output", "", "")
	var packages, expected repeated
	f.Var(&packages, "package", "")
	f.Var(&expected, "expected-repository", "")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if f.NArg() > 0 {
		return fail("unexpected-arguments")
	}
	var result map[string]any
	var e error
	if cmd == "gate-batch" {
		ps := []map[string]any{}
		for _, path := range packages {
			v, e := readJSON(path)
			if e != nil {
				return e
			}
			ps = append(ps, obj(v))
		}
		result, e = gateBatch(ps, expected)
	} else {
		values := []map[string]any{}
		for _, path := range []string{*mp, *sp, *ap} {
			v, e := readJSON(path)
			if e != nil {
				return e
			}
			values = append(values, obj(v))
		}
		rv, re := readJSON(*rp)
		if re != nil && os.IsNotExist(re) {
			return re
		}
		result, e = closePackage(*repo, values[0], values[1], values[2], obj(rv), *ref, *date, *root)
	}
	if e != nil {
		return e
	}
	if *output != "" {
		if e = noClobberJSON(*output, result); e != nil {
			return e
		}
	}
	return json.NewEncoder(out).Encode(result)
}
