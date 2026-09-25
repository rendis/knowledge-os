package discover

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"documentation-vault/internal/config"
)

// A cell run discovers every repository, then assembles cross-repository facts:
// libraries, typed configuration, IaC blocks, code literals and platform wiring.

type repoInput struct {
	Name, Remote, Path, Ref, Commit, Note string
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
	Kind    string `json:"kind"` // config | iac | code | platform | manifest | import
	Repo    string `json:"repo,omitempty"`
	Commit  string `json:"commit,omitempty"`
	File    string `json:"file,omitempty"`
	Key     string `json:"key,omitempty"`
	Value   string `json:"value,omitempty"`
	Project string `json:"project,omitempty"`
}

type resource struct {
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Direction string     `json:"direction"` // consume | publish | reference
	Topic     string     `json:"topic,omitempty"`
	Events    []string   `json:"events,omitempty"`
	Evidence  []evidence `json:"evidence"`
}

type pending struct {
	Kind    string `json:"kind"` // platform-access | not-in-platform | classification | direction
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
var resourceTypes = map[string]bool{"pubsub_topic": true, "pubsub_subscription": true, "database_object": true, "storage_bucket": true, "http_endpoint": true}

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
	if src, ok := inst["sources"].(map[string]any); ok {
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
	seen := map[string]bool{}
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
				out = append(out, repoInput{Name: name, Remote: remote, Path: p, Note: notes[name]})
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
	scans    []*repoScan
	st       *store
	platform platformIndex
	global   map[string]string // value -> resource type agreed across the cell
}

func (a *assembly) entryType(e entry) (string, float64) {
	if e.Secret || len(e.Placeholders) > 0 {
		return "", 0
	}
	// A platform name decides the type when the value is shaped like a resource name, or when
	// the key itself was judged to hold a Pub/Sub resource (generic words need the key).
	if t := a.platform.typeOf(e.Value); t != "" {
		if k, ok := a.st.ConfigKeys[keySignature(e)]; resourceShaped(e.Value) || ok && (k.Choice == "pubsub_topic" || k.Choice == "pubsub_subscription") {
			return t, 1
		}
	}
	if m := canonicalPubSub.FindStringSubmatch(strings.ToLower(e.Value)); m != nil {
		if m[2] == "topics" {
			return "pubsub_topic", 1
		}
		return "pubsub_subscription", 1
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
			if a.platform.typeOf(e.Value) != "" && resourceShaped(e.Value) || canonicalPubSub.MatchString(strings.ToLower(e.Value)) {
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
		blocks := map[string][]entry{}
		for _, e := range s.entries {
			if t := a.typed(e); t != "" {
				add(s, t, e.Value, evidence{Kind: "config", Repo: s.in.Name, Commit: s.in.Commit, File: e.File, Key: e.KeyPath})
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
	events := a.platformEvents()
	out := []repoFacts{}
	for _, s := range a.scans {
		f := repoFacts{Repo: s.in.Name, Remote: s.in.Remote, Path: s.in.Path, Commit: s.in.Commit, Ref: s.in.Ref, Note: s.in.Note,
			Languages: s.code.Languages, ConfigFiles: s.cfgFiles, ConfigCount: len(s.entries), Pending: []pending{}}
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
			if r.Type == "pubsub_topic" || r.Type == "pubsub_subscription" {
				channels["messaging"] = true
			}
			a.wire(r)
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

// wire completes a Pub/Sub resource with platform facts: subscription -> topic and filters.
func (a *assembly) wire(r *resource) {
	if r.Type != "pubsub_subscription" && r.Type != "pubsub_topic" {
		return
	}
	n := normalizeResource(r.Name)
	if r.Type == "pubsub_subscription" {
		if len(a.platform.subs[n]) == 0 {
			if v := a.platform.variants(n); len(v) > 0 {
				r.Evidence = append(r.Evidence, evidence{Kind: "platform-variant", Project: projectOf(v[0]), Value: strings.Join(v, ", ")})
			}
		}
		for _, s := range a.platform.subs[n] {
			r.Direction = "consume"
			r.Topic = s.Topic
			for _, at := range s.Attributes {
				if strings.EqualFold(at[0], "eventType") {
					r.Events = appendUnique(r.Events, at[1])
				}
			}
			r.Evidence = append(r.Evidence, evidence{Kind: "platform", Project: projectOf(s.Name), Value: s.Name})
		}
		return
	}
	if full := a.platform.topics[n]; len(full) > 0 {
		r.Evidence = append(r.Evidence, evidence{Kind: "platform", Project: projectOf(full[0]), Value: full[0]})
		return
	}
	// IaC modules often declare a base name and append a country or environment suffix.
	if v := a.platform.variants(n); len(v) > 0 {
		r.Evidence = append(r.Evidence, evidence{Kind: "platform-variant", Project: projectOf(v[0]), Value: strings.Join(v, ", ")})
	}
}

func projectOf(full string) string {
	parts := strings.Split(full, "/")
	if len(parts) > 1 && parts[0] == "projects" {
		return parts[1]
	}
	return ""
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
		if r.Type != "pubsub_topic" && r.Type != "pubsub_subscription" {
			continue
		}
		confirmed := false
		for _, ev := range r.Evidence {
			confirmed = confirmed || ev.Kind == "platform" || ev.Kind == "platform-variant"
		}
		if confirmed {
			continue
		}
		project := ""
		if m := canonicalPubSub.FindStringSubmatch(strings.ToLower(r.Name)); m != nil {
			project = m[1]
		}
		switch snap, ok := a.platform.projects[project]; {
		case project != "" && ok && snap.Status == "ok":
			out = append(out, pending{Kind: "not-in-platform", Subject: r.Name, Detail: "configured name is absent from the captured project " + project + "; the resource may be removed or renamed", Confirm: snap.Confirm})
		case project != "" && ok:
			out = append(out, pending{Kind: "platform-access", Subject: r.Name, Detail: fmt.Sprintf("project %s could not be read (%s)", project, snap.Status), Confirm: snap.Confirm})
		default:
			detail := "not found in any captured platform project"
			if project != "" {
				detail = "project " + project + " has not been captured"
			}
			out = append(out, pending{Kind: "platform-unverified", Subject: r.Name, Detail: detail, Confirm: "vaultctl discover platform --vault <VAULT> --project <PROJECT>"})
		}
	}
	for _, d := range f.Dependencies {
		if d.Category == "unclassified" {
			out = append(out, pending{Kind: "classification", Subject: d.ID, Detail: "dependency category not yet judged", Confirm: "vaultctl discover questions --vault <VAULT>"})
		}
	}
	return out
}

// platformProjects lists GCP project ids referenced by configuration (canonical paths and
// entries judged as project identifiers).
func (a *assembly) platformProjects() []string {
	set := map[string]bool{}
	for _, s := range a.scans {
		for _, e := range s.entries {
			if e.Secret {
				continue
			}
			for _, m := range regexp.MustCompile(`projects/([a-z][a-z0-9\-]{4,28}[a-z0-9])/`).FindAllStringSubmatch(e.Value, -1) {
				set[m[1]] = true
			}
			if k, ok := a.st.ConfigKeys[keySignature(e)]; ok && k.Choice == "cloud_project_or_region" && projectID.MatchString(e.Value) && strings.Count(e.Value, "-") >= 2 {
				set[e.Value] = true
			}
		}
	}
	return firstN(set, 1000)
}
