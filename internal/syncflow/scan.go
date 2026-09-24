package syncflow

import (
	"documentation-vault/internal/config"
	"encoding/json"
	"flag"
	"go.yaml.in/yaml/v3"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type scanSource struct{ path, identity, name, head, branch string }

func scanNote(vault, wanted string) (string, map[string]any, error) {
	root := filepath.Join(vault, "20-Repos")
	matches := []string{}
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(vault, p)
		if e != nil {
			return e
		}
		if filepath.Ext(p) != ".md" {
			return nil
		}
		if wanted == filepath.ToSlash(rel) || wanted == strings.TrimSuffix(filepath.Base(p), ".md") {
			matches = append(matches, p)
		}
		return nil
	})
	if e != nil {
		return "", nil, e
	}
	if len(matches) != 1 {
		return "", nil, fail("repository-note-missing-or-ambiguous")
	}
	b, e := os.ReadFile(matches[0])
	if e != nil {
		return "", nil, e
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", nil, fail("note-frontmatter-required")
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", nil, fail("invalid-note-frontmatter")
	}
	m := map[string]any{}
	if e = yaml.Unmarshal([]byte(text[4:4+end]), &m); e != nil {
		return "", nil, e
	}
	return matches[0], m, nil
}
func inspectSource(path string) (scanSource, error) {
	p, e := safe(path, true)
	if e != nil {
		return scanSource{}, e
	}
	remote, e := sourceGit(p, nil, "config", "--get", "remote.origin.url")
	if e != nil {
		return scanSource{}, e
	}
	identity, e := config.RemoteIdentity(strings.TrimSpace(string(remote)))
	if e != nil {
		return scanSource{}, e
	}
	head, e := sourceResolve(p, "HEAD")
	if e != nil {
		return scanSource{}, e
	}
	branchBytes, e := sourceGit(p, nil, "branch", "--show-current")
	if e != nil {
		return scanSource{}, e
	}
	name := identity[strings.LastIndex(identity, "/")+1:]
	branchName := strings.TrimSpace(string(branchBytes))
	if branchName == "" {
		branchName = "detached"
	}
	return scanSource{p, identity, name, head, branchName}, nil
}
func scanAllowed(source string, context map[string]any) bool {
	for _, v := range arr(context["roots"]) {
		r := str(obj(v)["path"])
		if source == filepath.Clean(r) || filepath.Dir(source) == filepath.Clean(r) {
			return true
		}
	}
	clone := str(context["clone_root"])
	if context["clone_authorized"] != true || clone == "" {
		return false
	}
	st, e := os.Stat(filepath.Join(source, ".git"))
	if e != nil || !st.Mode().IsRegular() {
		return false
	}
	b, e := sourceGit(source, nil, "rev-parse", "--git-common-dir")
	if e != nil {
		return false
	}
	common := strings.TrimSpace(string(b))
	if !filepath.IsAbs(common) {
		common = filepath.Join(source, common)
	}
	common, e = filepath.EvalSymlinks(common)
	return e == nil && filepath.Base(common) == ".git" && filepath.Dir(filepath.Dir(common)) == filepath.Clean(clone)
}
func scanRepository(vault, wanted, override string, queries []string, maxHits int) (map[string]any, error) {
	root, e := config.CanonicalRoot(vault)
	if e != nil {
		return nil, e
	}
	inst, e := config.LoadInstance(root)
	if e != nil {
		return nil, e
	}
	note, meta, e := scanNote(root, wanted)
	if e != nil {
		return nil, e
	}
	w, e := config.Workspace(root)
	if e != nil {
		return nil, e
	}
	ctx := obj(w["source_context"])
	if ctx["status"] != "resolved" {
		return nil, fail("source-roots-unconfigured")
	}
	aliases := []string{strings.TrimSuffix(filepath.Base(note), ".md")}
	if a, ok := meta["aliases"].([]any); ok {
		for _, v := range a {
			if s, ok := v.(string); ok && s != "" {
				aliases = append(aliases, s)
			}
		}
	} else if s, ok := meta["aliases"].(string); ok && s != "" {
		aliases = append(aliases, s)
	}
	wantRemote := ""
	if remote := str(meta["github"]); remote != "" {
		wantRemote, e = config.RemoteIdentity(remote)
		if e != nil {
			return nil, e
		}
	}
	matches := func(s scanSource) bool {
		if wantRemote != "" {
			return s.identity == wantRemote
		}
		for _, a := range aliases {
			if strings.EqualFold(a, s.name) {
				return true
			}
			for _, v := range arr(obj(inst["sources"])["repo_prefixes"]) {
				prefix := strings.TrimSuffix(str(v), "-") + "-"
				if strings.HasPrefix(strings.ToLower(s.name), strings.ToLower(prefix)) && strings.EqualFold(strings.TrimPrefix(strings.ToLower(s.name), strings.ToLower(prefix)), strings.ToLower(a)) {
					return true
				}
			}
		}
		return false
	}
	candidates := []scanSource{}
	warnings := []any{}
	if override != "" {
		source, e := inspectSource(override)
		if e != nil {
			return nil, e
		}
		if !scanAllowed(source.path, ctx) {
			return nil, fail("source-outside-configured-roots")
		}
		if !matches(source) {
			return nil, fail("repository-note-identity-mismatch")
		}
		candidates = append(candidates, source)
	} else {
		seen := map[string]bool{}
		for _, v := range arr(ctx["roots"]) {
			p := str(obj(v)["path"])
			paths := []string{}
			if _, e := os.Stat(filepath.Join(p, ".git")); e == nil {
				paths = append(paths, p)
			} else {
				entries, e := os.ReadDir(p)
				if e != nil {
					return nil, e
				}
				for _, entry := range entries {
					if entry.IsDir() {
						paths = append(paths, filepath.Join(p, entry.Name()))
					}
				}
			}
			for _, path := range paths {
				if seen[path] {
					continue
				}
				seen[path] = true
				if _, e := os.Lstat(filepath.Join(path, ".git")); e != nil {
					continue
				}
				s, e := inspectSource(path)
				if e != nil {
					return nil, e
				}
				if matches(s) {
					candidates = append(candidates, s)
				}
			}
		}
	}
	if len(candidates) == 0 {
		return nil, fail("source-repository-not-found")
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].path < candidates[j].path })
	source := candidates[0]
	for _, s := range candidates[1:] {
		if s.identity != source.identity || s.head != source.head || s.branch != source.branch {
			return nil, fail("divergent-source-clones")
		}
		warnings = append(warnings, "equivalent duplicate clone skipped: "+s.path)
	}
	terms := map[string]bool{}
	for _, q := range append(aliases, queries...) {
		if q = strings.TrimSpace(q); q != "" {
			terms[q] = true
		}
	}
	if len(terms) > 32 {
		return nil, fail("too-many-query-terms")
	}
	listed, e := sourceGit(source.path, nil, "ls-files", "-z", "--cached")
	if e != nil {
		return nil, e
	}
	names := map[string]bool{}
	for _, n := range strings.Split(string(listed), "\x00") {
		if n != "" {
			names[n] = true
		}
	}
	hits := []any{}
	inventory := []any{}
	scanned, skipped := 0, 0
	truncated := false
	for _, v := range sortedKeys(names) {
		n := str(v)
		if relative(n, false) != nil {
			skipped++
			continue
		}
		skip := false
		for _, part := range strings.Split(n, "/") {
			if one(part, ".git", "node_modules", "vendor", "dist", "build", ".next", "coverage", "target", "__pycache__") {
				skip = true
			}
		}
		if skip {
			skipped++
			continue
		}
		p := filepath.Join(source.path, filepath.FromSlash(n))
		if _, e = safe(p, false); e != nil {
			skipped++
			continue
		}
		st, e := os.Stat(p)
		if e != nil || !st.Mode().IsRegular() || st.Size() > 1500000 {
			skipped++
			continue
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
			skipped++
			continue
		}
		scanned++
		if len(inventory) < 100 {
			inventory = append(inventory, map[string]any{"path": n, "sha256": hash(b)})
		}
		sensitiveLines := map[int]bool{}
		for _, line := range scanCredentialText(n, string(b)) {
			sensitiveLines[line] = true
		}
		for i, line := range strings.Split(string(b), "\n") {
			if sensitiveLines[i+1] {
				continue
			}
			matched := map[string]bool{}
			lower := strings.ToLower(line)
			for term := range terms {
				if strings.Contains(lower, strings.ToLower(term)) {
					matched[term] = true
				}
			}
			if len(matched) > 0 {
				if len(hits) < maxHits {
					hits = append(hits, map[string]any{"path": n, "line": i + 1, "terms": sortedKeys(matched)})
				} else {
					truncated = true
				}
			}
		}
	}
	status, e := sourceGit(source.path, nil, "status", "--porcelain", "--untracked-files=no")
	if e != nil {
		return nil, e
	}
	rel, _ := filepath.Rel(root, note)
	return map[string]any{"status": "pass", "code": "static-source-scan", "vault": root, "note": filepath.ToSlash(rel), "source_repository": source.path, "source_identity": source.identity, "branch": source.branch, "head_baseline": source.head, "dirty": len(status) > 0, "scope": "tracked working-tree text; includes uncommitted changes; excludes untracked files, symlinks, binary and oversized files; does not establish runtime or deployment", "query_terms": sortedKeys(terms), "scanned_files": scanned, "skipped_files": skipped, "inventory": inventory, "inventory_truncated": scanned > len(inventory), "hits": hits, "hits_truncated": truncated, "warnings": warnings}, nil
}
func runScan(args []string, out io.Writer) error {
	f := flag.NewFlagSet("scan", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	vault := f.String("vault", ".", "")
	repo := f.String("repo", "", "")
	source := f.String("source-repo", "", "")
	max := f.Int("max-hits", 12, "")
	var queries repeated
	f.Var(&queries, "query", "literal term")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if f.NArg() != 0 || *repo == "" || *max < 1 || *max > 100 {
		return fail("invalid-scan-arguments")
	}
	result, e := scanRepository(*vault, *repo, *source, queries, *max)
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(result)
}
