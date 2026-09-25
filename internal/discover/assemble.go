package discover

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"documentation-vault/internal/config"
)

// A cell run discovers every repository, then assembles cross-repository facts:
// libraries, typed configuration, IaC blocks, code literals and platform wiring.

type repoInput struct {
	Name, Remote, Path, Ref, Commit, Note string
	RefNote, RefErr                       string // reference-branch fallback or blocker
}

type repoScan struct {
	in       repoInput
	code     codeSurface
	deps     map[string]*dependency
	entries  []entry
	cfgFiles int
	paths    []string
}

func (s *repoScan) files() []string { return s.paths }

type evidence struct {
	Kind   string `json:"kind"` // config | iac | code | platform | manifest | import
	Repo   string `json:"repo,omitempty"`
	Commit string `json:"commit,omitempty"`
	File   string `json:"file,omitempty"`
	Key    string `json:"key,omitempty"`
	Value  string `json:"value,omitempty"`
	Scope  string `json:"scope,omitempty"` // "<provider>:<scope>"
}

type resource struct {
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Direction string     `json:"direction"` // consume | publish | reference
	Topic     string     `json:"topic,omitempty"`
	Events    []string   `json:"events,omitempty"`
	Evidence  []evidence `json:"evidence"`
	// MissingIn lists captured projects where a configuration file that declares that project uses
	// this short name but the platform has no such resource (e.g. present in prd, absent in uat).
	MissingIn []string `json:"missing_in,omitempty"`
}

type pending struct {
	Kind    string `json:"kind"` // platform-access | not-in-platform | classification | direction | reference-branch
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
	Confirm string `json:"confirm_with,omitempty"`
}

type repoFacts struct {
	Repo         string         `json:"repo"`
	Remote       string         `json:"remote,omitempty"`
	Path         string         `json:"path"`
	Commit       string         `json:"commit"`
	Ref          string         `json:"ref"`
	Note         string         `json:"note,omitempty"`
	Languages    map[string]int `json:"languages"`
	ServiceIDs   []string       `json:"service_ids"`
	Channels     []string       `json:"channels"`
	Dependencies []dependency   `json:"dependencies"`
	DeclaredOnly []dependency   `json:"declared_not_imported,omitempty"`
	ConfigFiles  int            `json:"config_files"`
	ConfigCount  int            `json:"config_entries"`
	Resources    []resource     `json:"resources"`
	Events       []eventFact    `json:"events,omitempty"`
	Pending      []pending      `json:"pending"`
}

type eventFact struct {
	Name     string     `json:"name"`
	Topic    string     `json:"topic"`
	Role     string     `json:"role"` // consume (subscription filter) | publish-candidate (code literal)
	Evidence []evidence `json:"evidence"`
}

var appPrefix = regexp.MustCompile(`^[a-z]+\d+-`)
var resourceTypes = map[string]bool{"message_topic": true, "message_subscription": true, "database_object": true, "storage_bucket": true, "http_endpoint": true}

func keySignature(e entry) string {
	parts := strings.FieldsFunc(e.KeyPath, func(r rune) bool { return r == '.' || r == '[' || r == ']' })
	clean := []string{}
	for _, p := range parts {
		if strings.Trim(p, "0123456789") != "" {
			clean = append(clean, p)
		}
	}
	parent := ""
	if len(clean) > 1 {
		parent = strings.ToLower(clean[len(clean)-2])
	}
	ctx := e.Context
	if i := strings.Index(ctx, " source="); i >= 0 {
		ctx = ctx[i+len(" source="):]
	} else if f := strings.Fields(ctx); len(f) > 0 {
		ctx = f[0]
	}
	return strings.ToLower(e.Key) + "|" + parent + "|" + ctx
}

func entryID(e entry) string { return keySignature(e) + "||" + e.Value }

// discoverRepositories lists local checkouts from the configured roots and keeps tracked ones.
func discoverRepositories(vault string, only map[string]bool) ([]repoInput, error) {
	inst, e := config.LoadInstance(vault)
	if e != nil {
		return nil, e
	}
	w, e := config.Workspace(vault)
	if e != nil {
		return nil, e
	}
	prefixes := []string{}
	branches := map[string]any{}
	if src, ok := inst["sources"].(map[string]any); ok {
		branches = asMap(src["reference_branches"])
		for _, p := range asList(src["repo_prefixes"]) {
			if s, ok := p.(string); ok && s != "" {
				prefixes = append(prefixes, strings.TrimRight(s, "-")+"-")
			}
		}
	}
	notes, e := repoNotes(vault)
	if e != nil {
		return nil, e
	}
	vaultID, _ := config.RemoteIdentity(gitRemoteOf(vault))
	vaultPath, _ := filepath.EvalSymlinks(vault)
	seen := map[string]bool{vaultPath: true} // the vault is never one of its own sources
	out := []repoInput{}
	sc, _ := w["source_context"].(config.Object)
	for _, r := range asList(sc["roots"]) {
		root, _ := asMap(r)["path"].(string)
		entries, e := os.ReadDir(root)
		if e != nil {
			continue
		}
		for _, d := range entries {
			if !d.IsDir() && d.Type()&os.ModeSymlink == 0 {
				continue
			}
			p, e := filepath.EvalSymlinks(filepath.Join(root, d.Name()))
			if e != nil || seen[p] {
				continue
			}
			if _, e := os.Stat(filepath.Join(p, ".git")); e != nil {
				continue
			}
			seen[p] = true
			remote := gitRemoteOf(p)
			id, _ := config.RemoteIdentity(remote)
			if id != "" && id == vaultID {
				continue
			}
			name := d.Name()
			if id != "" { // identity is lower-cased; keep the remote's own spelling
				r := strings.TrimSuffix(strings.TrimRight(remote, "/"), ".git")
				name = r[strings.LastIndexAny(r, "/:")+1:]
			}
			tracked := notes[name] != ""
			for _, pre := range prefixes {
				tracked = tracked || strings.HasPrefix(name, pre)
			}
			if len(only) > 0 {
				tracked = only[name]
			}
			if tracked {
				in := repoInput{Name: name, Remote: remote, Path: p, Note: notes[name]}
				configured, _ := branches[name].(string)
				var e error
				if in.Ref, in.RefNote, e = referenceRef(p, configured, config.ReferenceBranchOrder(inst)); e != nil {
					in.RefErr = e.Error()
				}
				out = append(out, in)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func gitRemoteOf(p string) string {
	s, _ := gitOutput(p, "remote", "get-url", "origin")
	return s
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}
func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

var aliasesLine = regexp.MustCompile(`(?m)^aliases:\s*\[(.*?)\]`)
var commitLine = regexp.MustCompile(`(?m)^commit-analizado:\s*"?([0-9a-f]{7,40})`)

// repoNotes maps repository names (note aliases and basenames) to repository note paths.
func repoNotes(vault string) (map[string]string, error) {
	out := map[string]string{}
	e := filepath.WalkDir(filepath.Join(vault, "20-Repos"), func(p string, d os.DirEntry, e error) error {
		if e != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(vault, p)
		names := []string{strings.TrimSuffix(filepath.Base(p), ".md")}
		if m := aliasesLine.FindStringSubmatch(string(b)); m != nil {
			for _, a := range strings.Split(m[1], ",") {
				if a = strings.Trim(strings.TrimSpace(a), `"'`); a != "" {
					names = append(names, a)
				}
			}
		}
		for _, n := range names {
			if out[n] == "" {
				out[n] = filepath.ToSlash(rel)
			}
		}
		return nil
	})
	if os.IsNotExist(e) {
		return out, nil
	}
	return out, e
}

func noteCommit(vault, note string) string {
	if note == "" {
		return ""
	}
	b, e := os.ReadFile(filepath.Join(vault, note))
	if e != nil {
		return ""
	}
	if m := commitLine.FindStringSubmatch(string(b)); m != nil {
		return m[1]
	}
	return ""
}

func scanRepository(in repoInput) (*repoScan, error) {
	s, e := openSnapshot(in.Path, in.Ref)
	if e != nil {
		return nil, e
	}
	defer s.close()
	in.Commit = s.commit
	code, e := scanCode(s, nil)
	if e != nil {
		return nil, e
	}
	entries, n, e := scanConfig(s)
	if e != nil {
		return nil, e
	}
	return &repoScan{in: in, code: code, deps: directDependencies(code), entries: entries, cfgFiles: n, paths: s.files}, nil
}

// resolveLibraries expands imports of local company libraries (Go modules, npm packages)
// into the dependencies of the imported package directory, recursively.
func resolveLibraries(scans []*repoScan) {
	goMods := map[string]*repoScan{}
	npmNames := map[string]*repoScan{}
	for _, s := range scans {
		for _, m := range s.code.Manifest.goModules {
			goMods[m] = s
		}
		if s.code.Manifest.npmName != "" {
			npmNames[s.code.Manifest.npmName] = s
		}
	}
	type key struct {
		repo *repoScan
		dir  string
	}
	memo := map[key]map[string]*dependency{}
	var pkgDeps func(lib *repoScan, dir string, stack map[key]bool) map[string]*dependency
	expand := func(owner *repoScan, file string, refs []importRef, stack map[key]bool, into map[string]*dependency, via string) {
		lang := codeExt[path.Ext(file)]
		for _, r := range refs {
			var lib *repoScan
			sub := ""
			if lang == "go" {
				best := ""
				for m := range goMods {
					if (r.Spec == m || strings.HasPrefix(r.Spec, m+"/")) && len(m) > len(best) {
						best = m
					}
				}
				if best != "" && goMods[best] != owner {
					lib, sub = goMods[best], strings.Trim(strings.TrimPrefix(r.Spec, best), "/")
				} else if best != "" && via != "" {
					// Inside a library only imported packages are expanded, so follow its own packages.
					for id, dep := range pkgDeps(owner, strings.Trim(strings.TrimPrefix(r.Spec, best), "/"), stack) {
						if into[id] == nil {
							cp := *dep
							into[id] = &cp
						}
					}
					continue
				} else if r.Kind == "own" {
					continue // the repository's own package: its files are expanded directly
				}
			} else if lang == "js" && npmNames[r.Family] != nil && npmNames[r.Family] != owner {
				lib, sub = npmNames[r.Family], "*"
			}
			if lib != nil {
				lk := "lib:" + lib.in.Name
				d := into[lk]
				if d == nil {
					d = &dependency{ID: lk, Lang: lang, Origin: "library", Category: "internal_library"}
					into[lk] = d
				}
				d.Files = appendUnique(d.Files, file)
				for id, dep := range pkgDeps(lib, sub, stack) {
					if into[id] == nil {
						cp := *dep
						cp.Origin, cp.Via, cp.Files = "library", lib.in.Name, []string{file}
						into[id] = &cp
					}
				}
				continue
			}
			id := dependencyID(lang, r.Family)
			d := into[id]
			if d == nil {
				d = &dependency{ID: id, Lang: lang, Origin: "import", Via: via}
				into[id] = d
			}
			d.Files = appendUnique(d.Files, file)
			d.Paths = appendUnique(d.Paths, r.Spec)
		}
	}
	pkgDeps = func(lib *repoScan, dir string, stack map[key]bool) map[string]*dependency {
		k := key{lib, dir}
		if m, ok := memo[k]; ok {
			return m
		}
		if stack[k] {
			return nil
		}
		stack[k] = true
		defer delete(stack, k)
		out := map[string]*dependency{}
		for f, refs := range lib.code.Files {
			d := path.Dir(f)
			if d == "." {
				d = ""
			}
			if dir == "*" || d == dir {
				expand(lib, f, refs, stack, out, lib.in.Name)
			}
		}
		memo[k] = out
		return out
	}
	for _, s := range scans {
		all := map[string]*dependency{}
		for f, refs := range s.code.Files {
			expand(s, f, refs, map[key]bool{}, all, "")
		}
		for _, d := range all {
			sort.Strings(d.Files)
			sort.Strings(d.Paths)
		}
		s.deps = all
	}
}

// assembly holds the typed, cross-repository view used by the report and the notes.
type assembly struct {
	scans     []*repoScan
	st        *store
	platform  platformIndex
	providers []string          // clouds the cell configured (platform.providers)
	global    map[string]string // value -> resource type agreed across the cell
}

func (a *assembly) entryType(e entry) (string, float64) {
	if e.Secret || len(e.Placeholders) > 0 {
		return "", 0
	}
	// A platform name decides the type when the value is shaped like a resource name, or when
	// the key itself was judged to hold a messaging resource (generic words need the key).
	if t := a.platform.typeOf(e.Value); t != "" {
		if k, ok := a.st.ConfigKeys[keySignature(e)]; resourceShaped(e.Value) || ok && isMessaging(k.Choice) {
			return t, 1
		}
	}
	if t := canonicalKind(e.Value); t != "" {
		return t, 1
	}
	if resourceShaped(e.Value) {
		if t := objectType(a.platform.objectsNamed(e.Value)); t != "" {
			return t, 1 // a database, table, collection or bucket the platform lists by this name
		}
	}
	if j, ok := a.st.ConfigValues[entryID(e)]; ok && resourceTypes[j.Choice] && j.Confidence >= 0.5 {
		return j.Choice, j.Confidence
	}
	return "", 0
}

func (a *assembly) typed(e entry) string {
	if e.Secret || len(e.Placeholders) > 0 {
		return ""
	}
	if g := a.global[e.Value]; g != "" {
		return g
	}
	t, _ := a.entryType(e)
	return t
}

func (a *assembly) buildConsensus() {
	votes := map[string]map[string]int{}
	for _, s := range a.scans {
		for _, e := range s.entries {
			if t, c := a.entryType(e); t != "" && c >= 0.8 {
				if votes[e.Value] == nil {
					votes[e.Value] = map[string]int{}
				}
				votes[e.Value][t]++
			}
		}
	}
	a.global = map[string]string{}
	for v, m := range votes {
		best, n := "", 0
		for t, c := range m {
			if c > n || c == n && t < best {
				best, n = t, c
			}
		}
		a.global[v] = best
	}
}

// pendingQuestions lists judgments that are still needed, in dependency order.
func (a *assembly) pendingQuestions() []question {
	qs := []question{}
	deps := map[string]*dependency{}
	for _, s := range a.scans {
		for id, d := range s.deps {
			if d.Origin != "library" || !strings.HasPrefix(id, "lib:") {
				if deps[id] == nil {
					cp := *d
					deps[id] = &cp
				} else {
					for _, p := range d.Paths {
						deps[id].Paths = appendUnique(deps[id].Paths, p)
					}
				}
			}
		}
		for _, m := range s.code.Manifest.declared {
			if deps["manifest:"+m] == nil && s.deps[m] == nil {
				deps["manifest:"+m] = &dependency{ID: "manifest:" + m, Origin: "manifest"}
			}
		}
	}
	for id, d := range deps {
		if strings.HasPrefix(id, "lib:") {
			continue
		}
		if _, ok := a.st.Dependencies[id]; ok {
			continue
		}
		if _, ok := a.st.Dependencies[strings.TrimPrefix(id, "manifest:")]; ok {
			continue // the same dependency was already judged from its imports
		}
		{
			paths := d.Paths
			sort.Strings(paths)
			if len(paths) > 12 {
				paths = paths[:12]
			}
			where := "imported in source code"
			if d.Origin == "manifest" {
				where = "declared in a build manifest"
			}
			qs = append(qs, newQuestion("dependency", id, map[string]any{"dependency": strings.SplitN(id, ":", 2)[1], "imported_paths": paths, "where_found": where, "language": d.Lang}))
		}
	}
	type group struct {
		paths, files, values map[string]bool
		ctx                  string
	}
	groups := map[string]*group{}
	for _, s := range a.scans {
		for _, e := range s.entries {
			sig := keySignature(e)
			g := groups[sig]
			if g == nil {
				g = &group{paths: map[string]bool{}, files: map[string]bool{}, values: map[string]bool{}, ctx: e.Context}
				groups[sig] = g
			}
			g.paths[e.KeyPath] = true
			g.files[e.File] = true
			if !e.Secret && resourceShaped(e.Value) {
				v := e.Value
				if len(v) > 120 {
					v = v[:120]
				}
				g.values[v] = true
			}
		}
	}
	for sig, g := range groups {
		if _, ok := a.st.ConfigKeys[sig]; ok {
			continue
		}
		vals := firstN(g.values, 4)
		if len(vals) == 0 {
			continue // no value of this key can name a resource (secret, scalar or free text)
		}
		ctx := g.ctx
		if len(ctx) > 160 {
			ctx = ctx[:160]
		}
		qs = append(qs, newQuestion("config_key", sig, map[string]any{"key_path_examples": firstN(g.paths, 3), "file_examples": firstN(g.files, 3), "context": ctx, "value_examples": vals}))
	}
	seen := map[string]bool{}
	for _, s := range a.scans {
		for _, e := range s.entries {
			if e.Secret || len(e.Placeholders) > 0 {
				continue
			}
			k, ok := a.st.ConfigKeys[keySignature(e)]
			if !ok || !resourceTypes[k.Choice] {
				continue
			}
			if a.platform.typeOf(e.Value) != "" && resourceShaped(e.Value) || canonicalKind(e.Value) != "" {
				continue // decided by the platform or by the canonical resource format
			}
			id := entryID(e)
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, ok := a.st.ConfigValues[id]; ok {
				continue
			}
			ctx := e.Context
			if len(ctx) > 200 {
				ctx = ctx[:200]
			}
			qs = append(qs, newQuestion("config_entry", id, map[string]any{"key_path": e.KeyPath, "value": e.Value, "file": e.File, "block_context": ctx}))
		}
	}
	sort.Slice(qs, func(i, j int) bool {
		if qs[i].Kind != qs[j].Kind {
			return kindOrder(qs[i].Kind) < kindOrder(qs[j].Kind)
		}
		return qs[i].ID < qs[j].ID
	})
	return qs
}

// resourceShaped is a format rule: resource names, paths and URLs have no whitespace and
// contain a separator; placeholders are resolved elsewhere.
func resourceShaped(v string) bool {
	return !strings.ContainsAny(v, " \t") && strings.ContainsAny(v, "-._/:") && !strings.Contains(v, "${") && !strings.Contains(v, "{{")
}

func kindOrder(k string) int {
	switch k {
	case "dependency":
		return 0
	case "config_key":
		return 1
	}
	return 2
}

func firstN(m map[string]bool, n int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// facts assembles every repository's facts once all judgments available are applied.
func (a *assembly) facts() []repoFacts {
	a.buildConsensus()
	ids := map[*repoScan]map[string]bool{}
	owners := map[string][]*repoScan{}
	for _, s := range a.scans {
		set := map[string]bool{strings.ToLower(s.in.Name): true, appPrefix.ReplaceAllString(strings.ToLower(s.in.Name), ""): true}
		for _, e := range s.entries {
			if k, ok := a.st.ConfigKeys[keySignature(e)]; ok && k.Choice == "service_identity" && len(e.Value) > 5 && len(e.Placeholders) == 0 && !e.Secret {
				set[strings.ToLower(e.Value)] = true
			}
		}
		ids[s] = set
		for id := range set {
			owners[id] = append(owners[id], s)
		}
	}
	res := map[*repoScan]map[string]*resource{}
	add := func(s *repoScan, t, name string, ev evidence) {
		if res[s] == nil {
			res[s] = map[string]*resource{}
		}
		k := t + "|" + normalizeResource(name)
		r := res[s][k]
		if r == nil {
			r = &resource{Type: t, Name: name, Direction: "reference"}
			res[s][k] = r
		}
		if len(r.Evidence) < 12 {
			r.Evidence = append(r.Evidence, ev)
		}
	}
	for _, s := range a.scans {
		fileScope := a.fileScopes(s)
		blocks := map[string][]entry{}
		for _, e := range s.entries {
			if t := a.typed(e); t != "" {
				ev := evidence{Kind: "config", Repo: s.in.Name, Commit: s.in.Commit, File: e.File, Key: e.KeyPath}
				if canonicalKind(e.Value) == "" {
					ev.Scope = fileScope[e.File] // a short name belongs to the scope its file declares
				}
				add(s, t, e.Value, ev)
			}
			if strings.HasPrefix(e.Context, "module") || strings.HasPrefix(e.Context, "resource") {
				blocks[e.File+"\x00"+e.Context] = append(blocks[e.File+"\x00"+e.Context], e)
			}
		}
		// An IaC block that names another service assigns its resources to that service.
		for _, bes := range blocks {
			targets := map[*repoScan]bool{}
			for _, e := range bes {
				for _, o := range owners[strings.ToLower(e.Value)] {
					if o != s {
						targets[o] = true
					}
				}
			}
			for o := range targets {
				for _, e := range bes {
					if t := a.typed(e); t != "" {
						add(o, t, e.Value, evidence{Kind: "iac", Repo: s.in.Name, Commit: s.in.Commit, File: e.File, Key: e.KeyPath})
					}
				}
			}
		}
	}
	// Code literals equal to a typed resource name (specific names only).
	known := map[string]string{}
	for _, m := range res {
		for _, r := range m {
			if len(r.Name) >= 8 && strings.ContainsAny(r.Name, "-._/:") {
				known[r.Name] = r.Type
			}
		}
	}
	for _, s := range a.scans {
		for lit, files := range s.code.Literals {
			if t := known[lit]; t != "" {
				add(s, t, lit, evidence{Kind: "code", Repo: s.in.Name, Commit: s.in.Commit, File: files[0], Value: lit})
			}
		}
	}
	// Code literals naming a data service the platform lists (a Firestore collection, a table, a bucket),
	// in a repository whose dependencies use that kind of service.
	for _, s := range a.scans {
		uses := a.categories(s)
		for lit, files := range s.code.Literals {
			objs := []objectRef{}
			for _, o := range a.platform.objectsNamed(lit) {
				if uses[o.res.Kind] {
					objs = append(objs, o)
				}
			}
			if t := objectType(objs); t != "" {
				add(s, t, lit, evidence{Kind: "code", Repo: s.in.Name, Commit: s.in.Commit, File: files[0], Value: lit})
			}
		}
	}
	events := a.platformEvents()
	out := []repoFacts{}
	for _, s := range a.scans {
		f := repoFacts{Repo: s.in.Name, Remote: s.in.Remote, Path: s.in.Path, Commit: s.in.Commit, Ref: s.in.Ref, Note: s.in.Note,
			Languages: s.code.Languages, ConfigFiles: s.cfgFiles, ConfigCount: len(s.entries), Pending: []pending{}}
		if s.in.RefNote != "" {
			f.Pending = append(f.Pending, pending{Kind: "reference-branch", Subject: s.in.Name, Detail: s.in.RefNote, Confirm: "onboard-cell: 90-Meta/reference-branches.md"})
		}
		f.ServiceIDs = firstN(ids[s], 50)
		channels := map[string]bool{}
		imported := map[string]bool{}
		for _, d := range s.deps {
			cp := *d
			if j, ok := a.st.Dependencies[strings.TrimPrefix(cp.ID, "")]; ok && cp.Category == "" {
				cp.Category = j.Choice
			}
			if cp.Category == "" {
				cp.Category = "unclassified"
			}
			imported[cp.Category] = true
			if cp.Category != "internal_no_io" && cp.Category != "internal_library" && cp.Category != "unclassified" {
				channels[cp.Category] = true
			}
			f.Dependencies = append(f.Dependencies, cp)
		}
		for _, m := range s.code.Manifest.declared {
			j, ok := a.st.Dependencies["manifest:"+m]
			if !ok {
				j, ok = a.st.Dependencies[m]
			}
			if ok && isConnectorCategory(j.Choice) && !imported[j.Choice] {
				f.DeclaredOnly = append(f.DeclaredOnly, dependency{ID: m, Origin: "manifest", Category: j.Choice})
				channels[j.Choice] = true
			}
		}
		sort.Slice(f.Dependencies, func(i, j int) bool { return f.Dependencies[i].ID < f.Dependencies[j].ID })
		for _, r := range res[s] {
			if isMessaging(r.Type) {
				channels["messaging"] = true
			}
			a.wire(r)
			if isMessaging(r.Type) {
				r.MissingIn = a.scopedMisses(r)
			}
			f.Resources = append(f.Resources, *r)
			for _, ev := range r.Events {
				f.Events = append(f.Events, eventFact{Name: ev, Topic: r.Topic, Role: "consume", Evidence: []evidence{{Kind: "platform", Value: r.Name}}})
			}
		}
		for name, topic := range events {
			if files, ok := s.code.Literals[name]; ok {
				f.Events = append(f.Events, eventFact{Name: name, Topic: topic, Role: "publish-candidate", Evidence: []evidence{{Kind: "code", Repo: s.in.Name, Commit: s.in.Commit, File: files[0], Value: name}}})
			}
		}
		sort.Slice(f.Resources, func(i, j int) bool {
			if f.Resources[i].Type != f.Resources[j].Type {
				return f.Resources[i].Type < f.Resources[j].Type
			}
			return f.Resources[i].Name < f.Resources[j].Name
		})
		sort.Slice(f.Events, func(i, j int) bool { return f.Events[i].Name+f.Events[i].Role < f.Events[j].Name+f.Events[j].Role })
		for c := range channels {
			f.Channels = append(f.Channels, c)
		}
		sort.Strings(f.Channels)
		f.Pending = append(f.Pending, a.pendingFor(f)...)
		out = append(out, f)
	}
	return out
}

func isConnectorCategory(c string) bool {
	switch c {
	case "messaging", "document_db", "sql_db", "warehouse", "object_storage", "file_transfer", "http_client", "http_server":
		return true
	}
	return false
}

func isMessaging(t string) bool { return t == "message_topic" || t == "message_subscription" }

// wire completes a messaging resource with platform facts: subscription or queue -> topic and filters.
func (a *assembly) wire(r *resource) {
	if r.Type == "database_object" || r.Type == "storage_bucket" {
		for _, o := range a.platform.objectsNamed(r.Name) {
			r.Evidence = append(r.Evidence, evidence{Kind: "platform", Scope: o.scope, Value: o.res.Type + " " + o.res.Name})
		}
	}
	for _, ob := range a.platform.observed[normalizeResource(r.Name)] {
		r.Evidence = append(r.Evidence, evidence{Kind: "platform-observed", Scope: ob.scope, Value: ob.service + " " + ob.name})
	}
	if !isMessaging(r.Type) {
		return
	}
	n := normalizeResource(r.Name)
	if r.Type == "message_subscription" {
		if len(a.platform.subs[n]) == 0 {
			if v := a.platform.variants(n); len(v) > 0 {
				r.Evidence = append(r.Evidence, evidence{Kind: "platform-variant", Scope: scopeOf(v[0]), Value: strings.Join(v, ", ")})
			}
		}
		for _, s := range a.platform.subs[n] {
			r.Direction = "consume"
			if s.Topic != "" {
				r.Topic = s.Topic
			}
			for _, at := range s.Attributes {
				if strings.EqualFold(at[0], "eventType") {
					r.Events = appendUnique(r.Events, at[1])
				}
			}
			r.Evidence = append(r.Evidence, evidence{Kind: "platform", Scope: scopeOf(s.Name), Value: s.Name})
		}
		return
	}
	if full := a.platform.topics[n]; len(full) > 0 {
		r.Evidence = append(r.Evidence, evidence{Kind: "platform", Scope: scopeOf(full[0]), Value: full[0]})
		return
	}
	// IaC modules often declare a base name and append a country or environment suffix.
	if v := a.platform.variants(n); len(v) > 0 {
		r.Evidence = append(r.Evidence, evidence{Kind: "platform-variant", Scope: scopeOf(v[0]), Value: strings.Join(v, ", ")})
	}
}

// fileScopes maps each configuration file to the captured platform scope it declares (an entry judged a
// cloud project or region whose value is a captured scope); files naming several scopes map to none.
func (a *assembly) fileScopes(s *repoScan) map[string]string {
	captured := map[string]string{}
	for k, snap := range a.platform.scopes {
		captured[strings.ToLower(snap.Scope)] = k
	}
	found := map[string]map[string]bool{}
	for _, e := range s.entries {
		if e.Secret || len(e.Placeholders) > 0 {
			continue
		}
		key, ok := captured[strings.ToLower(strings.TrimSpace(e.Value))]
		if !ok {
			continue
		}
		if k, ok := a.st.ConfigKeys[keySignature(e)]; ok && k.Choice == "cloud_project_or_region" {
			if found[e.File] == nil {
				found[e.File] = map[string]bool{}
			}
			found[e.File][key] = true
		}
	}
	out := map[string]string{}
	for f, ks := range found {
		if len(ks) == 1 {
			for k := range ks {
				out[f] = k
			}
		}
	}
	return out
}

// scopedMisses returns the declared scopes (with a readable snapshot) where the resource is absent.
func (a *assembly) scopedMisses(r *resource) []string {
	n := normalizeResource(r.Name)
	in := map[string]bool{}
	if r.Type == "message_subscription" {
		for _, s := range a.platform.subs[n] {
			in[scopeOf(s.Name)] = true
		}
	} else {
		for _, t := range a.platform.topics[n] {
			in[scopeOf(t)] = true
		}
	}
	for _, v := range a.platform.variants(n) {
		in[scopeOf(v)] = true
	}
	missing := []string{}
	if len(in) == 0 {
		return missing // absent everywhere: the generic pending already reports it
	}
	for _, ev := range r.Evidence {
		if ev.Kind != "config" || ev.Scope == "" || in[ev.Scope] {
			continue
		}
		if snap, ok := a.platform.scopes[ev.Scope]; !ok || snap.Status != "ok" {
			continue
		}
		// A consumer may name a topic owned by another system's scope; only an environment
		// counterpart of the same scope family (acme-x-prd / acme-x-uat) proves a missing resource.
		sibling := r.Type == "message_subscription"
		for p := range in {
			sibling = sibling || scopeFamily(p) == scopeFamily(ev.Scope)
		}
		if sibling {
			missing = appendUnique(missing, ev.Scope)
		}
	}
	return missing
}

// scopeFamily drops the trailing environment segment of a scope (gcp:acme-stock-uat -> gcp:acme-stock).
func scopeFamily(p string) string {
	if i := strings.LastIndex(p, "-"); i > 0 {
		return p[:i]
	}
	return p
}

func (a *assembly) platformEvents() map[string]string {
	out := map[string]string{}
	for _, subs := range a.platform.subs {
		for _, s := range subs {
			for _, at := range s.Attributes {
				if strings.EqualFold(at[0], "eventType") {
					out[at[1]] = s.Topic
				}
			}
		}
	}
	return out
}

// pendingFor turns everything that could not be confirmed into explicit pending items.
func (a *assembly) pendingFor(f repoFacts) []pending {
	out := []pending{}
	for _, r := range f.Resources {
		if !isMessaging(r.Type) {
			continue
		}
		for _, p := range r.MissingIn {
			present := []string{}
			for _, ev := range r.Evidence {
				if (ev.Kind == "platform" || ev.Kind == "platform-variant") && ev.Scope != "" && ev.Scope != p {
					present = appendUnique(present, ev.Scope)
				}
			}
			detail := "configuration declaring scope " + p + " uses this name, but " + p + " has no such resource"
			if len(present) > 0 {
				detail += " (it exists in " + strings.Join(present, ", ") + ")"
			}
			out = append(out, pending{Kind: "not-in-platform", Subject: r.Name + " @ " + p, Detail: detail, Confirm: a.platform.scopes[p].Confirm})
		}
		confirmed := false
		for _, ev := range r.Evidence {
			confirmed = confirmed || ev.Kind == "platform" || ev.Kind == "platform-variant" || ev.Kind == "platform-observed"
		}
		if confirmed {
			continue
		}
		scope := scopeOf(r.Name)
		switch snap, ok := a.platform.scopes[scope]; {
		case scope != "" && ok && snap.Status == "ok":
			out = append(out, pending{Kind: "not-in-platform", Subject: r.Name, Detail: "configured name is absent from the captured scope " + scope + "; the resource may be removed or renamed", Confirm: snap.Confirm})
		case scope != "" && ok:
			out = append(out, pending{Kind: "platform-access", Subject: r.Name, Detail: fmt.Sprintf("scope %s could not be read (%s)", scope, snap.Status), Confirm: snap.Confirm})
		default:
			detail, confirm := "not found in any captured platform scope", "vaultctl discover platform --vault <VAULT> --provider <PROVIDER> --scope <SCOPE>"
			if scope != "" {
				provider, id, _ := strings.Cut(scope, ":")
				detail, confirm = "scope "+scope+" has not been captured", "vaultctl discover platform --vault <VAULT> --provider "+provider+" --scope "+id
			}
			out = append(out, pending{Kind: "platform-unverified", Subject: r.Name, Detail: detail, Confirm: confirm})
		}
	}
	out = append(out, a.unmanaged(f)...)
	for _, d := range f.Dependencies {
		if d.Category == "unclassified" {
			out = append(out, pending{Kind: "classification", Subject: d.ID, Detail: "dependency category not yet judged", Confirm: "vaultctl discover questions --vault <VAULT>"})
		}
	}
	return out
}

// objectType is the resource type of the data services a name matches.
func objectType(objs []objectRef) string {
	t := ""
	for _, o := range objs {
		switch o.res.Kind {
		case "object_storage":
			t = "storage_bucket"
		default:
			return "database_object"
		}
	}
	return t
}

// categories returns the dependency categories a repository uses.
func (a *assembly) categories(s *repoScan) map[string]bool {
	out := map[string]bool{}
	for id, d := range s.deps {
		c := d.Category
		if j, ok := a.st.Dependencies[id]; ok && c == "" {
			c = j.Choice
		}
		out[c] = true
	}
	for _, m := range s.code.Manifest.declared {
		if j, ok := a.st.Dependencies["manifest:"+m]; ok {
			out[j.Choice] = true
		} else if j, ok := a.st.Dependencies[m]; ok {
			out[j.Choice] = true
		}
	}
	return out
}

// platformCategories are the dependency categories that reach a platform service.
var platformCategories = []string{"messaging", "document_db", "sql_db", "warehouse", "object_storage", "file_transfer", "scheduler_or_queue_runner", "secrets_or_identity", "cloud_sdk_other"}

// captures reports whether a configured provider reads the platform service behind a category.
func (a *assembly) captures(category string) bool {
	for _, n := range a.providers {
		if p := providers[n]; p != nil && slices.Contains(p.kinds(), category) {
			return true
		}
	}
	return false
}

// unmanaged lists the platform services a repository uses that no configured provider captures: the
// agent inspects them with the tools in reach instead of stopping at the binary's coverage.
func (a *assembly) unmanaged(f repoFacts) []pending {
	out := []pending{}
	for _, c := range platformCategories {
		if !slices.Contains(f.Channels, c) || a.captures(c) {
			continue
		}
		deps := []string{}
		for _, d := range append(append([]dependency{}, f.Dependencies...), f.DeclaredOnly...) {
			if d.Category == c && len(deps) < 3 {
				deps = appendUnique(deps, d.ID)
			}
		}
		out = append(out, pending{Kind: "platform-unmanaged", Subject: c,
			Detail:  fmt.Sprintf("no configured platform provider reads %s (%s): inspect it read-only with the tools in reach and record what you observe", c, strings.Join(deps, ", ")),
			Confirm: "vaultctl discover platform --vault <VAULT> --record <FILE>"})
	}
	return out
}

// platformScopes lists the scopes of the given providers that configuration references: canonical
// identifiers (resource paths, ARNs, resource ids) and entries judged a cloud project or region.
func (a *assembly) platformScopes(names []string) []string {
	set := map[string]bool{}
	for _, s := range a.scans {
		for _, e := range s.entries {
			if e.Secret {
				continue
			}
			judged := false
			if k, ok := a.st.ConfigKeys[keySignature(e)]; ok && k.Choice == "cloud_project_or_region" && len(e.Placeholders) == 0 {
				judged = true
			}
			for _, n := range names {
				p := providers[n]
				for _, sc := range p.scopesIn(e.Value) {
					set[scopeKey(n, sc)] = true
				}
				if judged && p.isScope(strings.TrimSpace(e.Value)) {
					set[scopeKey(n, strings.TrimSpace(e.Value))] = true
				}
			}
		}
	}
	return firstN(set, 1000)
}

// libraryContext scans, transitively, the tracked repositories whose Go module or npm package the
// given scans import, so a run limited to some repositories resolves company libraries exactly as a
// full run does. The returned scans only feed resolveLibraries; they produce no facts.
func libraryContext(vault string, scans []*repoScan) []*repoScan {
	all, e := discoverRepositories(vault, nil)
	if e != nil {
		return nil
	}
	have := map[string]bool{}
	for _, s := range scans {
		have[s.in.Name] = true
	}
	type candidate struct {
		in           repoInput
		module, name string
	}
	cands := []candidate{}
	for _, in := range all {
		if have[in.Name] || in.RefErr != "" {
			continue
		}
		c := candidate{in: in}
		if b, e := gitOutput(in.Path, "show", in.Ref+":go.mod"); e == nil {
			if m := goModule.FindStringSubmatch(b); m != nil {
				c.module = m[1]
			}
		}
		if b, e := gitOutput(in.Path, "show", in.Ref+":package.json"); e == nil {
			var j map[string]any
			if json.Unmarshal([]byte(b), &j) == nil {
				c.name, _ = j["name"].(string)
			}
		}
		if c.module != "" || c.name != "" {
			cands = append(cands, c)
		}
	}
	out := []*repoScan{}
	queue := append([]*repoScan{}, scans...)
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, refs := range s.code.Files {
			for _, r := range refs {
				for i, c := range cands {
					if have[c.in.Name] {
						continue
					}
					if c.module != "" && (r.Spec == c.module || strings.HasPrefix(r.Spec, c.module+"/")) || c.name != "" && r.Family == c.name {
						have[c.in.Name] = true
						if lib, e := scanRepository(cands[i].in); e == nil {
							out = append(out, lib)
							queue = append(queue, lib)
						}
					}
				}
			}
		}
	}
	return out
}
