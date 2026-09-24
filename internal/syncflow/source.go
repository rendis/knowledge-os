package syncflow

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var minimumSweepQuestions = map[string][]string{
	"inputs":            {"entrypoints", "producers-callers", "input-contracts"},
	"outputs":           {"consumers-destinations", "output-contracts", "delivery-failure-behavior"},
	"data":              {"resources", "read-write-paths", "schema-changes", "destructive-operations"},
	"business-behavior": {"validations", "transformations", "states", "domain-dimensions", "flags", "deduplication-idempotency"},
	"infrastructure":    {"resources-by-environment", "identity-access", "configuration-secret-references"},
	"deployment":        {"trigger-to-artifact", "artifact-to-target", "environment-conditions", "external-indirections"},
}
var sourceEnvRE = regexp.MustCompile(`^cloudrun/([^/]+)/\.env\.ya?ml$`)
var sourceKustomizeRE = regexp.MustCompile(`^kustomization/([^/]+)/.+$`)
var credentialAssignRE = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9_.-])['"]?([A-Za-z0-9_.-]*(?:password|passwd|pass|secret|token|api[_-]?key|private[_-]?key)[A-Za-z0-9_.-]*)['"]?\s*[:=]\s*([^,};\n]+)`)
var getenvRE = regexp.MustCompile(`(?:os\.getenv|os\.environ\.get)\(\s*['"]([^'"]+)['"]\s*,\s*(['"][^'"]*['"])\s*\)`)
var yamlNameRE = regexp.MustCompile(`(?i)^(\s*)-\s*name\s*:\s*['"]?([^'"#\s]+)['"]?\s*$`)
var yamlValueRE = regexp.MustCompile(`(?i)^\s*value\s*:\s*(.+?)\s*$`)
var yamlFromRE = regexp.MustCompile(`(?i)^\s*valueFrom\s*:`)
var quotedLiteralRE = regexp.MustCompile(`'([^']*)'|"([^"]*)"`)
var variableRE = regexp.MustCompile(`^\$[A-Za-z_][A-Za-z0-9_]*$`)
var bareReferenceRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$.\[\]()<>? |&]*$`)
var templateRE = regexp.MustCompile(`^['"]?\s*\{\{.+\}\}\s*['"]?$`)
var passNameRE = regexp.MustCompile(`(?:^|[_.-])pass(?:$|[_.-])`)

func sourceGit(repo string, input []byte, args ...string) ([]byte, error) {
	c := exec.Command("git", append([]string{"-C", repo}, args...)...)
	c.Stdin = bytes.NewReader(input)
	b, e := c.Output()
	if e != nil {
		return nil, fail("git-failure")
	}
	return b, nil
}
func sourceResolve(repo, ref string) (string, error) {
	b, e := sourceGit(repo, nil, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	s := strings.TrimSpace(string(b))
	if e != nil || !oidRE.MatchString(s) {
		return "", fail("invalid-revision")
	}
	return s, nil
}
func sourceEnvironment(p string) string {
	if m := sourceEnvRE.FindStringSubmatch(p); m != nil {
		return m[1]
	}
	if m := sourceKustomizeRE.FindStringSubmatch(p); m != nil && strings.ToLower(m[1]) != "base" {
		return m[1]
	}
	return ""
}
func credentialName(v string) bool {
	v = strings.ToLower(v)
	c := strings.NewReplacer("_", "", ".", "", "-", "").Replace(v)
	if strings.HasSuffix(c, "url") || strings.HasSuffix(c, "uri") {
		return false
	}
	for _, m := range []string{"password", "passwd", "secret", "token", "apikey", "privatekey"} {
		if strings.Contains(c, m) {
			return true
		}
	}
	return passNameRE.MatchString(v)
}
func sourcePlaceholder(v string) bool {
	v = strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(v), "'\"")))
	if v == "" || one(v, "changeme", "change-me", "example", "inherit", "placeholder", "redacted", "replace-me", "todo", "xxx", "undefined", "null", "none") {
		return true
	}
	for _, p := range []string{"dummy-", "fake-", "fixture-", "fixture_", "mock-", "test-"} {
		if strings.HasPrefix(v, p) {
			return true
		}
	}
	return strings.HasPrefix(v, "<") && strings.HasSuffix(v, ">")
}
func literalValues(rhs string, bareReference bool) []string {
	v := strings.TrimRight(strings.TrimSpace(rhs), ";, ")
	first := -1
	for _, s := range []string{"||", "??", "?:"} {
		if i := strings.Index(v, s); i >= 0 && (first < 0 || i < first) {
			first = i
		}
	}
	if first >= 0 {
		return literalValues(v[first+2:], false)
	}
	lower := strings.ToLower(v)
	if strings.Contains(lower, "${") {
		return nil
	}
	for _, m := range []string{"process.env", "os.getenv", "system.getenv", "secretkeyref", "valuefrom", "secretmanager", "getsecret", "getenv", "vault:"} {
		if strings.Contains(lower, m) {
			return nil
		}
	}
	if variableRE.MatchString(v) || templateRE.MatchString(v) {
		return nil
	}
	matches := quotedLiteralRE.FindAllStringSubmatch(v, -1)
	values := map[string]bool{}
	if len(matches) > 0 {
		for _, m := range matches {
			s := m[1]
			if strings.HasPrefix(m[0], "\"") {
				s = m[2]
			}
			if !sourcePlaceholder(s) {
				values[s] = true
			}
		}
	} else {
		v = strings.TrimSpace(strings.SplitN(v, " #", 2)[0])
		if !sourcePlaceholder(v) && !(bareReference && bareReferenceRE.MatchString(v)) {
			values[v] = true
		}
	}
	out := []string{}
	for v := range values {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
func sourceBare(p string) bool {
	return one(strings.ToLower(path.Ext(p)), ".cs", ".go", ".java", ".js", ".jsx", ".kt", ".py", ".rb", ".ts", ".tsx")
}
func sourceComment(l string) bool {
	s := strings.TrimSpace(l)
	return strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") || strings.HasPrefix(s, "*")
}
func safeIDToken(n, rhs string) bool {
	v := strings.ToLower(strings.Trim(strings.TrimRight(strings.TrimSpace(strings.SplitN(rhs, "#", 2)[0]), ";, "), "'\""))
	return strings.EqualFold(n, "id-token") && one(v, "read", "write", "none")
}
func scanCredentialText(p, text string) []int {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	bare := sourceBare(p)
	hits := map[int]bool{}
	if !bare && credentialName(path.Base(p)) {
		for i, l := range lines {
			if nonempty(l) && !sourceComment(l) && len(literalValues(l, false)) > 0 {
				hits[i+1] = true
				break
			}
		}
	}
	for i, l := range lines {
		if sourceComment(l) {
			continue
		}
		for _, m := range getenvRE.FindAllStringSubmatch(l, -1) {
			if credentialName(m[1]) && len(literalValues(m[2], false)) > 0 {
				hits[i+1] = true
			}
		}
		for _, m := range credentialAssignRE.FindAllStringSubmatch(l, -1) {
			if credentialName(m[1]) && !safeIDToken(m[1], m[2]) && len(literalValues(m[2], bare)) > 0 {
				hits[i+1] = true
			}
		}
	}
	for i, l := range lines {
		m := yamlNameRE.FindStringSubmatch(l)
		if m == nil || !credentialName(m[2]) {
			continue
		}
		indent := len(m[1])
		for j := i + 1; j < len(lines); j++ {
			c := lines[j]
			if !nonempty(c) || strings.HasPrefix(strings.TrimSpace(c), "#") {
				continue
			}
			if yamlNameRE.MatchString(c) || len(c)-len(strings.TrimLeft(c, " \t")) <= indent || yamlFromRE.MatchString(c) {
				break
			}
			if v := yamlValueRE.FindStringSubmatch(c); v != nil {
				if len(literalValues(v[1], false)) > 0 {
					hits[j+1] = true
				}
				break
			}
		}
	}
	out := []int{}
	for n := range hits {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}
func sourceBlob(repo, oid, p string) ([]byte, string, error) {
	kind, e := sourceGit(repo, nil, "cat-file", "-t", oid+":"+p)
	if e != nil {
		return nil, "", e
	}
	if strings.TrimSpace(string(kind)) == "commit" {
		return nil, "gitlink", nil
	}
	if strings.TrimSpace(string(kind)) != "blob" {
		return nil, "", fail("unsupported-path-object")
	}
	b, e := sourceGit(repo, nil, "show", oid+":"+p)
	if e != nil {
		return nil, "", e
	}
	if bytes.ContainsRune(b, 0) || !utf8.Valid(b) {
		return b, "binary", nil
	}
	return b, "text", nil
}
func buildManifest(repo, old, new string, newRepository bool) (map[string]any, error) {
	if _, e := sourceGit(repo, nil, "rev-parse", "--git-dir"); e != nil {
		return nil, fail("not-a-repository")
	}
	newOID, e := sourceResolve(repo, new)
	if e != nil {
		return nil, e
	}
	oldOID := ""
	paths := []any{}
	if newRepository {
		b, e := sourceGit(repo, nil, "hash-object", "-t", "tree", "--stdin")
		if e != nil {
			return nil, e
		}
		oldOID = strings.TrimSpace(string(b))
		b, e = sourceGit(repo, nil, "ls-tree", "-r", "-z", "--full-tree", newOID)
		if e != nil {
			return nil, e
		}
		for _, r := range bytes.Split(b, []byte{0}) {
			if len(r) == 0 {
				continue
			}
			f := bytes.SplitN(r, []byte{'\t'}, 2)
			if len(f) != 2 || !utf8.Valid(f[1]) {
				return nil, fail("invalid-path-encoding")
			}
			paths = append(paths, map[string]any{"path": string(f[1]), "status": "added"})
		}
	} else {
		oldOID, e = sourceResolve(repo, old)
		if e != nil {
			return nil, e
		}
		if _, e = sourceGit(repo, nil, "merge-base", "--is-ancestor", oldOID, newOID); e != nil {
			return nil, fail("non-ancestor")
		}
		b, e := sourceGit(repo, nil, "diff", "--raw", "-z", "--no-renames", "--abbrev=64", oldOID, newOID, "--")
		if e != nil {
			return nil, e
		}
		f := bytes.Split(b, []byte{0})
		if len(f) > 0 && len(f[len(f)-1]) == 0 {
			f = f[:len(f)-1]
		}
		if len(f)%2 != 0 {
			return nil, fail("invalid-git-output")
		}
		for i := 0; i < len(f); i += 2 {
			h := strings.Fields(string(f[i]))
			if len(h) != 5 || !utf8.Valid(f[i+1]) {
				return nil, fail("invalid-git-output")
			}
			status := map[string]string{"A": "added", "D": "deleted", "M": "modified", "T": "type-changed"}[h[4]]
			if status == "" {
				return nil, fail("unsupported-change-status")
			}
			paths = append(paths, map[string]any{"path": string(f[i+1]), "status": status})
		}
	}
	sort.Slice(paths, func(i, j int) bool { return str(obj(paths[i])["path"]) < str(obj(paths[j])["path"]) })
	envs, suspects := []any{}, []any{}
	for _, v := range paths {
		r := obj(v)
		p := str(r["path"])
		if relative(p, false) != nil {
			return nil, fail("invalid-path")
		}
		status := str(r["status"])
		rev := newOID
		if status == "deleted" {
			rev = oldOID
		}
		b, kind, e := sourceBlob(repo, rev, p)
		if e != nil {
			return nil, e
		}
		r["content_kind"] = kind
		if env := sourceEnvironment(p); env != "" {
			envs = append(envs, map[string]any{"environment": env, "path": p})
		}
		hits := map[int]bool{}
		if kind == "text" {
			for _, n := range scanCredentialText(p, string(b)) {
				hits[n] = true
			}
		}
		if status == "modified" || status == "type-changed" {
			b, kind, e = sourceBlob(repo, oldOID, p)
			if e != nil {
				return nil, e
			}
			if kind == "text" {
				for _, n := range scanCredentialText(p, string(b)) {
					hits[n] = true
				}
			}
		}
		ns := []int{}
		for n := range hits {
			ns = append(ns, n)
		}
		sort.Ints(ns)
		for _, n := range ns {
			suspects = append(suspects, map[string]any{"path": p, "line": json.Number(strconv.Itoa(n))})
		}
	}
	return map[string]any{"version": json.Number("1"), "old_oid": oldOID, "new_oid": newOID, "paths": paths, "environment_configs": envs, "credential_suspects": suspects}, nil
}
func validateSourceManifest(m map[string]any) error {
	bad := func() error { return fail("manifest-invalid") }
	if !exact(m, "version old_oid new_oid paths environment_configs credential_suspects") || m["version"] != json.Number("1") || !oidRE.MatchString(str(m["old_oid"])) || !oidRE.MatchString(str(m["new_oid"])) {
		return bad()
	}
	ps, ok := m["paths"].([]any)
	if !ok {
		return bad()
	}
	paths := map[string]bool{}
	expected := []any{}
	prev := ""
	for _, v := range ps {
		r := obj(v)
		p := str(r["path"])
		if !exact(r, "path status content_kind") || relative(p, false) != nil || p <= prev || !one(r["status"], "added", "deleted", "modified", "type-changed") || !one(r["content_kind"], "text", "binary", "gitlink") {
			return bad()
		}
		prev = p
		paths[p] = true
		if env := sourceEnvironment(p); env != "" {
			expected = append(expected, map[string]any{"path": p, "environment": env})
		}
	}
	if !reflect.DeepEqual(m["environment_configs"], expected) {
		return bad()
	}
	ss, ok := m["credential_suspects"].([]any)
	if !ok {
		return bad()
	}
	prev = ""
	last := 0
	for _, v := range ss {
		r := obj(v)
		p := str(r["path"])
		number, ok := r["line"].(json.Number)
		n, e := strconv.Atoi(string(number))
		if !exact(r, "path line") || !paths[p] || !ok || e != nil || n < 1 || p < prev || p == prev && n <= last {
			return bad()
		}
		prev = p
		last = n
	}
	return nil
}
func initializeAnalysis(m map[string]any, repository string, nodes []string) (map[string]any, error) {
	if e := validateSourceManifest(m); e != nil {
		return nil, e
	}
	repository = strings.TrimSpace(repository)
	base := repositoryPrefixRE.ReplaceAllString(repository, "")
	if !basename(repository) || !basename(base) {
		return nil, fail("invalid-repository-name")
	}
	set := map[string]bool{base: true}
	for _, n := range nodes {
		n = strings.TrimSpace(n)
		if !basename(n) {
			return nil, fail("invalid-node-basename")
		}
		set[n] = true
	}
	empty := len(arr(m["paths"])) == 0 && (m["old_oid"] == "4b825dc642cb6eb9a060e54bf8d69288fbee4904" || m["old_oid"] == hash([]byte("tree 0\x00")))
	reason, status, result := "", "", ""
	if empty {
		reason = "repository tree contains no paths"
		status = "not-applicable"
		result = "no-change"
	}
	checklist := map[string]any{}
	for dim, qs := range minimumSweepQuestions {
		questions := map[string]any{}
		for _, q := range qs {
			questions[q] = status
		}
		evidence := []any{}
		if !empty {
			for _, v := range arr(m["environment_configs"]) {
				p := str(obj(v)["path"])
				if dim == "infrastructure" || dim == "deployment" && sourceKustomizeRE.MatchString(p) {
					evidence = append(evidence, map[string]any{"path": p, "anchor": ""})
				}
			}
			if len(evidence) == 0 {
				evidence = append(evidence, map[string]any{"path": "", "anchor": ""})
			}
		}
		checklist[dim] = map[string]any{"status": status, "questions": questions, "reason": reason, "evidence": evidence}
	}
	claims := []any{}
	if !empty {
		claims = append(claims, map[string]any{"claim_id": "", "statement": "", "evidence": []any{map[string]any{"path": "", "anchor": ""}}})
	}
	records := []any{}
	for _, n := range sortedKeys(set) {
		records = append(records, map[string]any{"basename": n, "action": "no-change", "reason": reason, "claim_ids": []any{}})
	}
	paths := []any{}
	for _, v := range arr(m["paths"]) {
		paths = append(paths, map[string]any{"path": obj(v)["path"], "disposition": "", "reason": "", "claim_ids": []any{}})
	}
	return map[string]any{"version": json.Number("3"), "repository": repository, "old_oid": m["old_oid"], "new_oid": m["new_oid"], "paths": paths, "checklist": checklist, "claims": claims, "nodes": records, "result": result, "blockers": []any{}}, nil
}
func runSource(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fail("usage-error")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var repo, old, new, mp, repository string
	var nodes repeated
	switch args[0] {
	case "build", "build-new":
		f.StringVar(&repo, "repo", "", "")
		f.StringVar(&new, "new", "", "")
		if args[0] == "build" {
			f.StringVar(&old, "old", "", "")
		}
	case "init-analysis":
		f.StringVar(&mp, "manifest", "", "")
		f.StringVar(&repository, "repository", "", "")
		f.Var(&nodes, "node", "")
	default:
		return fail("usage-error")
	}
	if f.Parse(args[1:]) != nil || f.NArg() != 0 {
		return fail("usage-error")
	}
	var v map[string]any
	var e error
	if args[0] == "init-analysis" {
		if mp == "" || repository == "" {
			return fail("usage-error")
		}
		m, err := readJSON(mp)
		if err != nil {
			return err
		}
		v, e = initializeAnalysis(obj(m), repository, nodes)
	} else {
		if repo == "" || new == "" || args[0] == "build" && old == "" {
			return fail("usage-error")
		}
		v, e = buildManifest(repo, old, new, args[0] == "build-new")
	}
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(v)
}

func isNewManifest(m map[string]any) bool {
	return m["old_oid"] == "4b825dc642cb6eb9a060e54bf8d69288fbee4904" || m["old_oid"] == hash([]byte("tree 0\x00"))
}
func emptyNewManifest(m map[string]any) bool { return len(arr(m["paths"])) == 0 && isNewManifest(m) }
