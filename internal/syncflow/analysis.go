package syncflow

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

const analysisFields = "version repository old_oid new_oid paths checklist claims nodes result blockers"
const fallbackReason = "No safe qualifying claim remained after deterministic finalization."
const nonblockingReason = "Unavailable content was recorded as not observed; safe evidence remains eligible."
const redactionReason = "Credential-bearing content was excluded from synchronization evidence."

var analysisDimensions = []string{"inputs", "outputs", "data", "business-behavior", "infrastructure", "deployment"}

func isEmptyNewManifest(m map[string]any) bool { return isNewManifest(m) && len(arr(m["paths"])) == 0 }
func cloneMap(m map[string]any) map[string]any {
	b, _ := canonical(m)
	v, _ := decode(b)
	return obj(v)
}
func localBasename(s string) string {
	return regexp.MustCompile(`^APP[0-9]{5}-`).ReplaceAllString(s, "")
}
func analysisEvidence(v any, empty bool) (map[string]any, error) {
	a, ok := v.([]any)
	if !ok || len(a) == 0 && !empty {
		return nil, fail("evidence-required")
	}
	paths := map[string]any{}
	for _, x := range a {
		m := obj(x)
		if !exact(m, "path anchor") || relative(str(m["path"]), false) != nil || !nonempty(str(m["anchor"])) {
			return nil, fail("invalid-evidence")
		}
		paths[str(m["path"])] = nil
	}
	return paths, nil
}
func manifestPaths(m map[string]any, k string) map[string]any {
	out := map[string]any{}
	for _, v := range arr(m[k]) {
		out[str(obj(v)["path"])] = nil
	}
	return out
}
func analysisSurface(a map[string]any) []any {
	out := []any{a["blockers"]}
	for _, k := range []string{"paths", "nodes"} {
		for _, v := range arr(a[k]) {
			out = append(out, obj(v)["reason"])
		}
	}
	for _, v := range obj(a["checklist"]) {
		m := obj(v)
		out = append(out, m["reason"])
		for _, ev := range arr(m["evidence"]) {
			out = append(out, obj(ev)["anchor"])
		}
	}
	for _, v := range arr(a["claims"]) {
		out = append(out, claimSurface(obj(v)))
	}
	return out
}
func claimSurface(c map[string]any) []any {
	out := []any{c["claim_id"], c["statement"]}
	for _, v := range arr(c["evidence"]) {
		out = append(out, obj(v)["anchor"])
	}
	return out
}
func asciiAlphaNum(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}
func redactLiteral(s, lit string) string {
	if lit == "" {
		return s
	}
	if len(lit) >= 8 {
		return strings.ReplaceAll(s, lit, "[redacted]")
	}
	out := ""
	offset := 0
	for {
		i := strings.Index(s[offset:], lit)
		if i < 0 {
			return out + s[offset:]
		}
		i += offset
		end := i + len(lit)
		if (i == 0 || !asciiAlphaNum(s[i-1])) && (end == len(s) || !asciiAlphaNum(s[end])) {
			out += s[offset:i] + "[redacted]"
			offset = end
		} else {
			out += s[offset:end]
			offset = end
		}
	}
}
func redactValue(v any, literals []string) any {
	switch x := v.(type) {
	case string:
		for _, l := range literals {
			x = redactLiteral(x, l)
		}
		return x
	case []any:
		out := []any{}
		for _, v := range x {
			out = append(out, redactValue(v, literals))
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			out[k] = redactValue(v, literals)
		}
		return out
	}
	return v
}
func exposed(v any, literals []string) bool { return !reflect.DeepEqual(v, redactValue(v, literals)) }
func validateAnalysisShape(a, m, s map[string]any, literals []string) error {
	if !exact(a, analysisFields) || !(a["version"] == json.Number("2") || a["version"] == json.Number("3")) {
		return fail("analysis-fields-invalid")
	}
	if a["version"] != s["version"] || a["repository"] != s["repository"] || a["old_oid"] != m["old_oid"] || a["new_oid"] != m["new_oid"] {
		return fail("scaffold-identity-mismatch")
	}
	if exposed(analysisSurface(a), literals) {
		return fail("credential-value-exposed")
	}
	mp := manifestPaths(m, "paths")
	suspects := manifestPaths(m, "credential_suspects")
	seen := map[string]any{}
	dispositions := map[string]string{}
	refs := []any{}
	blocked := false
	decisions, ok := a["paths"].([]any)
	if !ok {
		return fail("invalid-paths")
	}
	prev := ""
	for _, v := range decisions {
		p := obj(v)
		path := str(p["path"])
		if !exact(p, "path disposition reason claim_ids") || relative(path, false) != nil || path <= prev || !one(p["disposition"], "relevant", "not-documentable", "blocked") || !nonempty(str(p["reason"])) || !stable(p["claim_ids"], func(s string) bool { return true }) {
			return fail("invalid-path-decision")
		}
		prev = path
		seen[path] = nil
		dispositions[path] = str(p["disposition"])
		refs = append(refs, arr(p["claim_ids"])...)
		if p["disposition"] != "relevant" && len(arr(p["claim_ids"])) > 0 {
			return fail("claim-path-disposition-mismatch")
		}
		blocked = blocked || p["disposition"] == "blocked"
	}
	if !reflect.DeepEqual(sortedKeys(seen), sortedKeys(mp)) {
		return fail("path-coverage-mismatch")
	}
	checklist := obj(a["checklist"])
	if !exact(checklist, strings.Join(analysisDimensions, " ")) {
		return fail("checklist-incomplete")
	}
	for _, dimension := range analysisDimensions {
		c := obj(checklist[dimension])
		sc := obj(obj(s["checklist"])[dimension])
		q := obj(c["questions"])
		if !exact(c, "status questions reason evidence") || !one(c["status"], "checked", "not-applicable", "blocked") || !nonempty(str(c["reason"])) || !reflect.DeepEqual(sortedKeys(q), sortedKeys(obj(sc["questions"]))) {
			return fail("invalid-checklist-entry")
		}
		hasBlocked, allNA := false, true
		for _, v := range q {
			if !one(v, "observed", "not-observed", "not-applicable", "blocked") {
				return fail("invalid-question-status")
			}
			hasBlocked = hasBlocked || v == "blocked"
			allNA = allNA && v == "not-applicable"
		}
		if !(c["status"] == "blocked" && hasBlocked || c["status"] == "not-applicable" && allNA || c["status"] == "checked" && !hasBlocked && !allNA) {
			return fail("dimension-status-mismatch")
		}
		ep, e := analysisEvidence(c["evidence"], len(mp) == 0 && c["status"] == "not-applicable")
		if e != nil {
			return e
		}
		for path := range manifestPaths(m, "environment_configs") {
			required := dimension == "infrastructure" || dimension == "deployment" && strings.HasPrefix(path, "kustomization/")
			if _, ok := ep[path]; required && !ok {
				return fail("environment-evidence-missing")
			}
		}
		blocked = blocked || c["status"] == "blocked"
	}
	claims, ok := a["claims"].([]any)
	if !ok {
		return fail("invalid-claims")
	}
	ids := map[string]any{}
	for _, v := range claims {
		c := obj(v)
		id := str(c["claim_id"])
		if !exact(c, "claim_id statement evidence") || !nonempty(id) || !nonempty(str(c["statement"])) {
			return fail("invalid-claim")
		}
		if _, ok := ids[id]; ok {
			return fail("duplicate-claim-id")
		}
		ids[id] = nil
		ep, e := analysisEvidence(c["evidence"], false)
		if e != nil {
			return e
		}
		for path := range ep {
			if _, ok := suspects[path]; ok {
				return fail("credential-suspect-claim-evidence")
			}
			if d, ok := dispositions[path]; ok && d != "relevant" {
				return fail("claim-path-disposition-mismatch")
			}
		}
	}
	nodes, ok := a["nodes"].([]any)
	if !ok || len(nodes) == 0 {
		return fail("node-decision-required")
	}
	nodeNames := map[string]any{}
	writeClaims := map[string]any{}
	actions := []string{}
	prev = ""
	for _, v := range nodes {
		n := obj(v)
		name := str(n["basename"])
		action := str(n["action"])
		if !exact(n, "basename action reason claim_ids") || !basename(name) || name <= prev || !one(action, "create", "update", "consolidate", "retire", "no-change") || !nonempty(str(n["reason"])) || !stable(n["claim_ids"], func(s string) bool { return true }) {
			return fail("invalid-node-decision")
		}
		prev = name
		nodeNames[name] = nil
		actions = append(actions, action)
		refs = append(refs, arr(n["claim_ids"])...)
		if action == "no-change" && len(arr(n["claim_ids"])) > 0 {
			return fail("no-change-node-has-claims")
		}
		if action != "no-change" {
			for _, id := range arr(n["claim_ids"]) {
				writeClaims[str(id)] = nil
			}
			if a["result"] == "documentation-change" && len(arr(n["claim_ids"])) == 0 {
				return fail("write-node-claim-required")
			}
		}
	}
	for _, v := range arr(s["nodes"]) {
		if _, ok := nodeNames[str(obj(v)["basename"])]; !ok {
			return fail("scaffold-seed-node-missing")
		}
	}
	if _, ok := nodeNames[localBasename(str(a["repository"]))]; !ok {
		return fail("repository-node-missing")
	}
	for _, id := range refs {
		if _, ok := ids[str(id)]; !ok {
			return fail("unknown-claim-reference")
		}
	}
	result := a["result"]
	if !one(result, "documentation-change", "traceability-only", "blocked", "no-change") {
		return fail("invalid-result")
	}
	if blocked != (result == "blocked") {
		return fail("blocked-state-mismatch")
	}
	if result == "documentation-change" {
		if len(ids) == 0 {
			return fail("result-claim-mismatch")
		}
		for id := range ids {
			if _, ok := writeClaims[id]; !ok {
				return fail("claim-write-node-reference-missing")
			}
		}
	} else if len(ids) > 0 {
		return fail("result-claim-mismatch")
	}
	if one(result, "blocked", "no-change") {
		for _, action := range actions {
			if action != "no-change" {
				return fail("result-node-action-mismatch")
			}
		}
	}
	if result == "no-change" && !isNewManifest(m) || result == "traceability-only" && isNewManifest(m) {
		return fail("result-repository-kind-mismatch")
	}
	if result == "traceability-only" {
		for _, v := range nodes {
			n := obj(v)
			want := "no-change"
			if n["basename"] == localBasename(str(a["repository"])) {
				want = "update"
			}
			if n["action"] != want {
				return fail("traceability-node-action-mismatch")
			}
		}
	}
	blockers, ok := a["blockers"].([]any)
	if !ok {
		return fail("invalid-blockers")
	}
	for _, v := range blockers {
		if !nonempty(str(v)) {
			return fail("invalid-blockers")
		}
	}
	if result == "blocked" && len(blockers) == 0 || result != "blocked" && len(blockers) > 0 {
		return fail("blocker-state-mismatch")
	}
	if isEmptyNewManifest(m) && !reflect.DeepEqual(a, s) {
		return fail("terminal-analysis-mismatch")
	}
	return nil
}
func evidenceAvailable(repo string, m, a, r map[string]any) error {
	paths := map[string]any{}
	for _, v := range obj(a["checklist"]) {
		for _, ev := range arr(obj(v)["evidence"]) {
			paths[str(obj(ev)["path"])] = nil
		}
	}
	for _, v := range arr(a["claims"]) {
		for _, ev := range arr(obj(v)["evidence"]) {
			paths[str(obj(ev)["path"])] = nil
		}
	}
	for _, v := range arr(r["findings"]) {
		paths[str(obj(obj(v)["evidence"])["path"])] = nil
	}
	for path := range paths {
		found := false
		for _, oid := range []string{str(m["old_oid"]), str(m["new_oid"])} {
			if exec.Command("git", "-C", repo, "cat-file", "-e", oid+":"+path).Run() == nil {
				found = true
				break
			}
		}
		if !found {
			return fail("evidence-path-unavailable")
		}
	}
	return nil
}

var legacySweep = map[string]string{
	"inputs":            "http-openapi pubsub-subscriptions cron-cronjobs scheduler functions eventarc cloud-run application user",
	"outputs":           "publications outgoing-http data-writes integrations fire-and-forget retries dlq",
	"data":              "engine database-schema-dataset-bucket-collection read-targets write-targets migrations-entities upsert truncation bulk-deletion synchronize",
	"business-behavior": "validations transformations states country-store-bu-brand-category-vendor flags deduplication idempotency",
	"infrastructure":    "runtime-project-by-environment deployment schedulers-subscriptions config-maps service-accounts secret-references",
	"deployment":        "event-input workflow-job environment-condition build-artifact deploy-action gcp-project platform-resource location namespace-workload manifest-overlay-values",
}

func validateAnalysisInputs(repo string, m, s map[string]any) error {
	if e := validateSourceManifest(m); e != nil {
		return e
	}
	expected, e := buildManifest(repo, str(m["old_oid"]), str(m["new_oid"]), isNewManifest(m))
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(expected, m) {
		return fail("manifest-source-mismatch")
	}
	if !exact(s, analysisFields) {
		return fail("scaffold-fields-invalid")
	}
	repository := str(s["repository"])
	matched := false
	remotes, e := sourceGit(repo, nil, "remote")
	if e != nil {
		return e
	}
	for _, remote := range strings.Fields(string(remotes)) {
		urls, e := sourceGit(repo, nil, "remote", "get-url", "--all", remote)
		if e != nil {
			continue
		}
		for _, url := range strings.Fields(string(urls)) {
			tail := regexp.MustCompile(`[/\\:]`).Split(strings.TrimRight(url, "/"), -1)
			n := strings.TrimSuffix(strings.ToLower(tail[len(tail)-1]), ".git")
			matched = matched || n == strings.ToLower(repository) || n == strings.ToLower(localBasename(repository))
		}
	}
	if !matched {
		return fail("repository-checkout-mismatch")
	}
	nodes := []string{}
	for _, v := range arr(s["nodes"]) {
		nodes = append(nodes, str(obj(v)["basename"]))
	}
	expected, e = initializeAnalysis(m, repository, nodes)
	if e != nil {
		return e
	}
	if s["version"] == json.Number("2") {
		expected["version"] = json.Number("2")
		for dimension, qs := range legacySweep {
			questions := map[string]any{}
			status := ""
			if isEmptyNewManifest(m) {
				status = "not-applicable"
			}
			for _, q := range strings.Fields(qs) {
				questions[q] = status
			}
			obj(obj(expected["checklist"])[dimension])["questions"] = questions
		}
	}
	if !reflect.DeepEqual(expected, s) {
		return fail("scaffold-mismatch")
	}
	return nil
}
func credentialLiterals(repo string, m map[string]any) ([]string, error) {
	set := map[string]bool{}
	for _, v := range arr(m["credential_suspects"]) {
		sus := obj(v)
		path := str(sus["path"])
		n, e := objNumber(sus["line"])
		if e != nil || n < 1 {
			return nil, fail("manifest-invalid")
		}
		for _, oid := range []string{str(m["old_oid"]), str(m["new_oid"])} {
			b, kind, e := sourceBlob(repo, oid, path)
			if e != nil || kind != "text" {
				continue
			}
			lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
			if n > len(lines) {
				continue
			}
			line := lines[n-1]
			add := func(values []string) {
				for _, s := range values {
					set[s] = true
				}
			}
			for _, x := range getenvRE.FindAllStringSubmatch(line, -1) {
				if credentialName(x[1]) {
					add(literalValues(x[2], false))
				}
			}
			for _, x := range credentialAssignRE.FindAllStringSubmatch(line, -1) {
				if credentialName(x[1]) && !safeIDToken(x[1], x[2]) {
					add(literalValues(x[2], sourceBare(path)))
				}
			}
			if x := yamlValueRE.FindStringSubmatch(line); x != nil {
				add(literalValues(x[1], false))
			}
			if credentialName(filepath.Base(path)) {
				add(literalValues(line, false))
			}
		}
	}
	out := []string{}
	for s := range set {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out, nil
}
func objNumber(v any) (int, error) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, fail("invalid-integer")
	}
	i, e := n.Int64()
	return int(i), e
}
func checkAnalysis(repo string, m, s, a map[string]any, currentRef string) error {
	if e := validateAnalysisInputs(repo, m, s); e != nil {
		return e
	}
	literals, e := credentialLiterals(repo, m)
	if e != nil {
		return e
	}
	if e = validateAnalysisShape(a, m, s, literals); e != nil {
		return e
	}
	if e = evidenceAvailable(repo, m, a, nil); e != nil {
		return e
	}
	if currentRef != "" {
		if !strings.HasPrefix(currentRef, "refs/heads/") && !strings.HasPrefix(currentRef, "refs/remotes/") {
			return fail("invalid-current-ref")
		}
		if _, e = sourceGit(repo, nil, "check-ref-format", currentRef); e != nil {
			return fail("invalid-current-ref")
		}
		oid, e := sourceResolve(repo, currentRef)
		if e != nil {
			return fail("invalid-current-ref")
		}
		if oid != m["new_oid"] {
			return fail("stale-head")
		}
	}
	return nil
}
func validateAnalysisReview(repo string, m, s, a, r map[string]any) error {
	if !exact(r, "version repository manifest_digest scaffold_digest analysis_digest verdict findings") || r["version"] != json.Number("3") || r["repository"] != a["repository"] || !one(r["verdict"], "accept", "revise", "blocked") {
		return fail("review-fields-invalid")
	}
	for k, v := range map[string]any{"manifest": m, "scaffold": s, "analysis": a} {
		d, e := digest(v)
		if e != nil || r[k+"_digest"] != d {
			return fail("review-artifact-mismatch")
		}
	}
	findings, ok := r["findings"].([]any)
	if !ok {
		return fail("invalid-review-findings")
	}
	literals, e := credentialLiterals(repo, m)
	if e != nil {
		return e
	}
	for _, v := range findings {
		f := obj(v)
		if !exact(f, "target category reason nodes evidence") || !nonempty(str(f["target"])) || !nonempty(str(f["category"])) || !nonempty(str(f["reason"])) || !stable(f["nodes"], basename) || len(arr(f["nodes"])) == 0 {
			return fail("invalid-review-finding")
		}
		if _, e = analysisEvidence([]any{f["evidence"]}, false); e != nil {
			return e
		}
		if exposed([]any{f["reason"], obj(f["evidence"])["anchor"]}, literals) {
			return fail("credential-value-exposed")
		}
	}
	if r["verdict"] == "accept" && len(findings) > 0 || r["verdict"] != "accept" && len(findings) == 0 {
		return fail("review-verdict-findings-mismatch")
	}
	if a["result"] == "blocked" && r["verdict"] == "accept" {
		return fail("blocked-analysis-cannot-be-accepted")
	}
	if isEmptyNewManifest(m) && (r["verdict"] != "accept" || len(findings) > 0) {
		return fail("terminal-review-mismatch")
	}
	return evidenceAvailable(repo, m, a, r)
}
func fallbackAnalysisPayload(s, m map[string]any) map[string]any {
	a := cloneMap(s)
	if isEmptyNewManifest(m) {
		return a
	}
	paths := []any{}
	for _, v := range arr(m["paths"]) {
		paths = append(paths, obj(v)["path"])
	}
	for _, v := range arr(a["paths"]) {
		p := obj(v)
		p["disposition"] = "not-documentable"
		p["reason"] = fallbackReason
		p["claim_ids"] = []any{}
	}
	for dim, v := range obj(a["checklist"]) {
		c := obj(v)
		status, qstatus := "checked", "not-observed"
		if len(paths) == 0 {
			status, qstatus = "not-applicable", "not-applicable"
		}
		c["status"] = status
		c["reason"] = fallbackReason
		for q := range obj(c["questions"]) {
			obj(c["questions"])[q] = qstatus
		}
		ep := []any{}
		if len(paths) > 0 {
			for _, ev := range arr(m["environment_configs"]) {
				path := str(obj(ev)["path"])
				if dim == "infrastructure" || dim == "deployment" && sourceKustomizeRE.MatchString(path) {
					ep = append(ep, path)
				}
			}
			if len(ep) == 0 {
				ep = append(ep, paths[0])
			}
		}
		evidence := []any{}
		for _, path := range ep {
			evidence = append(evidence, map[string]any{"path": path, "anchor": "exact commit path"})
		}
		c["evidence"] = evidence
	}
	a["claims"] = []any{}
	a["blockers"] = []any{}
	a["result"] = "traceability-only"
	if isNewManifest(m) {
		a["result"] = "no-change"
	}
	for _, v := range arr(a["nodes"]) {
		n := obj(v)
		n["action"] = "no-change"
		if !isNewManifest(m) && n["basename"] == localBasename(str(a["repository"])) {
			n["action"] = "update"
		}
		n["reason"] = fallbackReason
		n["claim_ids"] = []any{}
	}
	return a
}
func materializeAnalysis(a, s map[string]any) map[string]any {
	out := cloneMap(s)
	for _, k := range []string{"paths", "nodes"} {
		key := "path"
		fields := []string{"disposition", "reason", "claim_ids"}
		if k == "nodes" {
			key = "basename"
			fields = []string{"action", "reason", "claim_ids"}
		}
		by := map[string]map[string]any{}
		for _, v := range arr(a[k]) {
			x := obj(v)
			name := str(x[key])
			if k == "paths" && relative(name, false) == nil || k == "nodes" && basename(name) {
				by[name] = x
			}
		}
		targets := map[string]map[string]any{}
		for _, v := range arr(out[k]) {
			x := obj(v)
			targets[str(x[key])] = x
		}
		if k == "nodes" {
			for n := range by {
				if targets[n] == nil {
					targets[n] = map[string]any{"basename": n, "action": "no-change", "reason": "", "claim_ids": []any{}}
				}
			}
		}
		items := []any{}
		for _, nv := range sortedKeys(targets) {
			name := str(nv)
			target := targets[name]
			for _, field := range fields {
				if value, ok := by[name][field]; ok {
					target[field] = value
				}
			}
			items = append(items, target)
		}
		out[k] = items
	}
	for dim, v := range obj(out["checklist"]) {
		target := obj(v)
		source := obj(obj(a["checklist"])[dim])
		if source == nil {
			continue
		}
		for q := range obj(target["questions"]) {
			if value, ok := obj(source["questions"])[q]; ok {
				obj(target["questions"])[q] = value
			}
		}
		status, valid := questionAggregate(obj(target["questions"]))
		if valid {
			target["status"] = status
		}
		for _, field := range []string{"reason", "evidence"} {
			if value, ok := source[field]; ok {
				target[field] = value
			}
		}
	}
	if claims, ok := a["claims"].([]any); ok {
		out["claims"] = []any{}
		for _, v := range claims {
			if c := obj(v); c != nil {
				item := map[string]any{}
				for _, k := range []string{"claim_id", "statement", "evidence"} {
					if value, ok := c[k]; ok {
						item[k] = value
					}
				}
				out["claims"] = append(arr(out["claims"]), item)
			}
		}
	}
	out["result"] = a["result"]
	if out["result"] == nil {
		out["result"] = ""
	}
	out["blockers"] = a["blockers"]
	if out["blockers"] == nil {
		out["blockers"] = []any{}
	}
	return cloneMap(out)
}
func questionAggregate(q map[string]any) (string, bool) {
	if len(q) == 0 {
		return "", false
	}
	blocked, allNA := false, true
	for _, v := range q {
		if !one(v, "observed", "not-observed", "not-applicable", "blocked") {
			return "", false
		}
		blocked = blocked || v == "blocked"
		allNA = allNA && v == "not-applicable"
	}
	if blocked {
		return "blocked", true
	}
	if allNA {
		return "not-applicable", true
	}
	return "checked", true
}
func finalizeAnalysisPayload(a, m map[string]any, literals []string) (map[string]any, error) {
	out := cloneMap(a)
	sus := manifestPaths(m, "credential_suspects")
	claims := []any{}
	ids := map[string]bool{}
	for _, v := range arr(a["claims"]) {
		c := obj(v)
		bad := exposed(claimSurface(c), literals)
		for _, ev := range arr(c["evidence"]) {
			if _, ok := sus[str(obj(ev)["path"])]; ok {
				bad = true
			}
		}
		if !bad {
			claims = append(claims, c)
			if nonempty(str(c["claim_id"])) {
				ids[str(c["claim_id"])] = true
			}
		}
	}
	filter := func(v any) []any {
		set := map[string]bool{}
		for _, id := range arr(v) {
			if ids[str(id)] {
				set[str(id)] = true
			}
		}
		return sortedKeys(set)
	}
	for _, k := range []string{"paths", "nodes"} {
		for _, v := range arr(out[k]) {
			x := obj(v)
			x["reason"] = redactValue(x["reason"], literals)
			if _, ok := x["claim_ids"].([]any); ok {
				x["claim_ids"] = filter(x["claim_ids"])
			}
			if k == "paths" {
				if _, ok := sus[str(x["path"])]; ok {
					x["disposition"] = "not-documentable"
					x["reason"] = redactionReason
					x["claim_ids"] = []any{}
				} else if x["disposition"] == "blocked" {
					x["disposition"] = "not-documentable"
					x["reason"] = nonblockingReason
					x["claim_ids"] = []any{}
				}
			} else if x["action"] == "no-change" {
				x["claim_ids"] = []any{}
			} else if len(arr(x["claim_ids"])) == 0 {
				x["action"] = "no-change"
				x["reason"] = fallbackReason
			}
		}
	}
	for dim, v := range obj(out["checklist"]) {
		c := obj(v)
		c["reason"] = redactValue(c["reason"], literals)
		for _, ev := range arr(c["evidence"]) {
			obj(ev)["anchor"] = redactValue(obj(ev)["anchor"], literals)
		}
		for q, value := range obj(c["questions"]) {
			if value == "blocked" {
				obj(c["questions"])[q] = "not-observed"
				c["reason"] = nonblockingReason
			}
		}
		if status, ok := questionAggregate(obj(c["questions"])); ok {
			c["status"] = status
		}
		required := []string{}
		for _, ev := range arr(m["environment_configs"]) {
			path := str(obj(ev)["path"])
			if dim == "infrastructure" || dim == "deployment" && sourceKustomizeRE.MatchString(path) {
				required = append(required, path)
			}
		}
		if len(required) > 0 {
			by := map[string]any{}
			for _, ev := range arr(c["evidence"]) {
				item := obj(ev)
				if nonempty(str(item["path"])) && nonempty(str(item["anchor"])) {
					by[str(item["path"])] = item
				}
			}
			for _, path := range required {
				if by[path] == nil {
					by[path] = map[string]any{"path": path, "anchor": "exact commit path"}
				}
			}
			c["evidence"] = []any{}
			for _, path := range sortedKeys(by) {
				c["evidence"] = append(arr(c["evidence"]), by[str(path)])
			}
		}
	}
	writeIDs := map[string]bool{}
	for _, v := range arr(out["nodes"]) {
		n := obj(v)
		if n["action"] != "no-change" {
			for _, id := range arr(n["claim_ids"]) {
				writeIDs[str(id)] = true
			}
		}
	}
	kept := []any{}
	ids = map[string]bool{}
	for _, v := range claims {
		c := obj(v)
		id := str(c["claim_id"])
		if writeIDs[id] {
			kept = append(kept, c)
			ids[id] = true
		}
	}
	out["claims"] = kept
	for _, k := range []string{"paths", "nodes"} {
		for _, v := range arr(out[k]) {
			x := obj(v)
			if _, ok := x["claim_ids"].([]any); ok {
				x["claim_ids"] = filter(x["claim_ids"])
			}
		}
	}
	out["blockers"] = []any{}
	out["result"] = "documentation-change"
	if len(kept) == 0 {
		out["result"] = "traceability-only"
		if isNewManifest(m) {
			out["result"] = "no-change"
		}
		for _, v := range arr(out["nodes"]) {
			n := obj(v)
			n["action"] = "no-change"
			if !isNewManifest(m) && n["basename"] == localBasename(str(out["repository"])) {
				n["action"] = "update"
			}
			n["reason"] = fallbackReason
			n["claim_ids"] = []any{}
		}
		for _, v := range arr(out["paths"]) {
			obj(v)["claim_ids"] = []any{}
		}
	}
	return out, nil
}
func ambiguousAnalysis(a, m map[string]any) bool {
	for _, spec := range []struct{ k, key string }{{"paths", "path"}, {"claims", "claim_id"}, {"nodes", "basename"}} {
		seen := map[string]bool{}
		for _, v := range arr(a[spec.k]) {
			id := str(obj(v)[spec.key])
			if seen[id] {
				return true
			}
			seen[id] = true
		}
		if spec.k == "paths" {
			mp := manifestPaths(m, "paths")
			if !reflect.DeepEqual(sortedKeys(seen), sortedKeys(mp)) {
				return true
			}
		}
	}
	return false
}
func replaceAnalysisJSON(path string, v any) error {
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	if _, e = safe(filepath.Dir(abs), true); e != nil {
		return e
	}
	if st, e := os.Lstat(abs); e == nil && (!st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0) {
		return fail("unsafe-path")
	} else if e != nil && !os.IsNotExist(e) {
		return e
	}
	return atomicState(filepath.Dir(abs), []string{filepath.Base(abs)}, v)
}
func runAnalysis(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fail("missing-command")
	}
	cmd := args[0]
	if cmd != "check" && cmd != "finalize-analysis" {
		return fail("unsupported-analysis-operation")
	}
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	repo := f.String("repo", "", "")
	mp := f.String("manifest", "", "")
	sp := f.String("scaffold", "", "")
	ap := f.String("analysis", "", "")
	ref := f.String("current-ref", "", "")
	output := f.String("output", "", "")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if f.NArg() != 0 || *repo == "" || *mp == "" || *sp == "" || *ap == "" {
		return fail("missing-required-argument")
	}
	if *output != "" {
		abs, e := filepath.Abs(*output)
		if e != nil {
			return e
		}
		for _, input := range []string{*ap, *mp, *sp} {
			p, e := filepath.Abs(input)
			if e != nil {
				return e
			}
			if p == abs {
				return fail("output-conflicts-with-input")
			}
		}
	}
	mv, e := readJSON(*mp)
	if e != nil {
		return e
	}
	sv, e := readJSON(*sp)
	if e != nil {
		return e
	}
	m, s := obj(mv), obj(sv)
	av, readErr := readJSON(*ap)
	a := obj(av)
	var result map[string]any
	if cmd == "check" {
		if readErr != nil {
			return readErr
		}
		if *ref == "" {
			return fail("invalid-current-ref")
		}
		if e = checkAnalysis(*repo, m, s, a, *ref); e != nil {
			return e
		}
		md, _ := digest(m)
		sd, _ := digest(s)
		ad, _ := digest(a)
		result = map[string]any{"code": "ok", "status": "pass", "repository": a["repository"], "manifest_digest": md, "scaffold_digest": sd, "analysis_digest": ad}
	} else {
		if e = validateAnalysisInputs(*repo, m, s); e != nil {
			return e
		}
		literals, e := credentialLiterals(*repo, m)
		if e != nil {
			return e
		}
		fallback := fallbackAnalysisPayload(s, m)
		useFallback := readErr != nil || a == nil || ambiguousAnalysis(a, m) || !isEmptyNewManifest(m) && reflect.DeepEqual(a, fallback)
		for _, k := range []string{"repository", "old_oid", "new_oid"} {
			if value, ok := a[k]; ok && value != s[k] {
				useFallback = true
			}
		}
		inputIssues := []any{}
		addIssue := func(e error) {
			if e != nil {
				inputIssues = append(inputIssues, map[string]any{"code": e.Error(), "field": "$"})
			}
		}
		if readErr != nil {
			addIssue(fail("invalid-json"))
		} else {
			addIssue(validateAnalysisShape(a, m, s, literals))
		}
		var finalized map[string]any
		if useFallback {
			finalized = fallback
		} else {
			finalized, e = finalizeAnalysisPayload(materializeAnalysis(a, s), m, literals)
			if e != nil {
				return e
			}
		}
		candidateErr := validateAnalysisShape(finalized, m, s, literals)
		if candidateErr == nil {
			candidateErr = evidenceAvailable(*repo, m, finalized, nil)
		}
		if candidateErr != nil {
			addIssue(candidateErr)
			useFallback = true
			finalized = fallback
			if e = validateAnalysisShape(finalized, m, s, literals); e != nil {
				return e
			}
			if e = evidenceAvailable(*repo, m, finalized, nil); e != nil {
				return e
			}
		}
		changed := readErr != nil || !reflect.DeepEqual(a, finalized)
		if changed {
			if e = replaceAnalysisJSON(*ap, finalized); e != nil {
				return e
			}
		}
		dropped := len(arr(a["claims"])) - len(arr(finalized["claims"]))
		if readErr != nil {
			dropped = 0
		}
		result = map[string]any{"code": "analysis-finalized", "status": "pass", "changed": changed, "dropped_claims": dropped, "fallback_used": useFallback, "input_issues": inputIssues}
	}
	if *output != "" {
		abs, e := filepath.Abs(*output)
		if e != nil {
			return e
		}
		for _, input := range []string{*ap, *mp, *sp} {
			p, e := filepath.Abs(input)
			if e != nil {
				return e
			}
			if p == abs {
				return fail("output-conflicts-with-input")
			}
		}
		if e = replaceAnalysisJSON(*output, result); e != nil {
			return e
		}
	}
	return json.NewEncoder(out).Encode(result)
}
