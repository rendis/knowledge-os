package retrieval

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"knowledge-os/internal/discover"
)

// Code reads the cell's repositories at their reference branch, the last mile after a pack: search
// a pattern (and state the scope of an absence), show a file or a range, or show a function with
// where it is called and what each caller does next. It replaces git grep / git show against
// checkouts whose working tree may be elsewhere.

// CodeOptions select what `code` does.
type CodeOptions struct {
	Repo       string // repository, its note's basename or alias, or "all"
	Grep       string // regular expression, Perl-compatible where git has PCRE
	Show       string // path[:FROM-TO]
	Func       string // function or method names, comma-separated
	Path       string // glob limiting --grep and --func to some files
	Up         int    // --func: follow the callers this many levels up
	Down       bool   // --func: the repository's functions it calls, each with its exits
	IgnoreCase bool
	Tests      bool
	Budget     int
	shown      map[string]bool // settings whose values an earlier function of the same call printed
}

// Code writes the result as Markdown.
func (i *Index) Code(ctx context.Context, o CodeOptions, w io.Writer) error {
	modes := 0
	for _, m := range []string{o.Grep, o.Show, o.Func} {
		if m != "" {
			modes++
		}
	}
	if o.Repo == "" || modes != 1 {
		return fmt.Errorf("code needs --repo and exactly one of --grep, --show or --func")
	}
	src := discover.NewSources(i.Root)
	repos := []string{}
	if o.Repo == "all" {
		if o.Grep == "" {
			return fmt.Errorf("--repo all works with --grep")
		}
		repos = discover.Tracked(i.Root)
	} else {
		repo, err := i.codeRepo(ctx, src, o.Repo)
		if err != nil {
			return err
		}
		repos = []string{repo}
	}
	out := &packWriter{w: w, budget: o.Budget}
	switch {
	case o.Grep != "":
		return writeSearch(out, src, repos, o)
	case o.Show != "":
		return writeShow(out, src, repos[0], o.Show)
	}
	// --func a,b,c: each in turn, sharing the budget.
	names := strings.Split(o.Func, ",")
	o.shown = map[string]bool{}
	for k, name := range names {
		one := o
		one.Func = strings.TrimSpace(name)
		if one.Func == "" {
			continue
		}
		if k > 0 {
			fmt.Fprintln(out, "\n---")
		}
		if err := writeFunc(out, src, repos[0], one); err != nil {
			if len(names) == 1 {
				return err
			}
			fmt.Fprintf(out, "\n# %s: %v\n", one.Func, err)
		}
	}
	return nil
}

// codeRepo resolves a repository name, or the repository of a note named by basename or alias.
func (i *Index) codeRepo(ctx context.Context, src *discover.Sources, name string) (string, error) {
	if repo, _, ok := src.Resolve(name); ok {
		return repo, nil
	}
	notes, err := i.knowledgeNotes(ctx)
	if err != nil {
		return "", err
	}
	if n, e := findNote(notes, name); e == nil {
		if s, ok := discover.RepositoryNoteState(i.Root, n.path); ok {
			if repo, _, ok := src.Resolve(s.Repo); ok {
				return repo, nil
			}
		}
	}
	_, why, _ := src.Resolve(name)
	known := discover.Tracked(i.Root)
	return "", fmt.Errorf("cannot read %q: %s; repositories with a checkout: %s", name, why, strings.Join(firstN(known, 30), ", "))
}

func writeSearch(out *packWriter, src *discover.Sources, repos []string, o CodeOptions) error {
	scope := "tests left out"
	if o.Tests {
		scope = "tests included"
	}
	fmt.Fprintf(out, "# Code search /%s/%s (%s regex)\n", o.Grep, map[bool]string{true: "i", false: ""}[o.IgnoreCase], discover.GrepDialect())
	none := []string{}
	for _, repo := range repos {
		hits, in, searched, err := src.Search(repo, o.Grep, o.Path, o.IgnoreCase, o.Tests, false)
		_, ref, _ := src.Resolve(repo)
		if err != nil {
			fmt.Fprintf(out, "\n## %s\n\nNot searched: %s\n", repo, err)
			continue
		}
		if len(hits) == 0 {
			none = append(none, fmt.Sprintf("%s at %s (%d files)", repo, ref, searched))
			continue
		}
		byFile, order := map[string][]discover.Hit{}, []string{}
		for _, h := range hits {
			if _, ok := byFile[h.Path]; !ok {
				order = append(order, h.Path)
			}
			byFile[h.Path] = append(byFile[h.Path], h)
		}
		sort.SliceStable(order, func(a, b int) bool {
			return discover.CodeFile(order[a]) && !discover.CodeFile(order[b])
		})
		fmt.Fprintf(out, "\n## %s at %s — %d lines in %d files (%d files searched, %s)\n", repo, ref, len(hits), len(order), searched, scope)
		// Configuration lines that read the same in many files (one per environment) print once.
		same := map[string][]string{}
		for _, f := range order {
			if !discover.CodeFile(f) && len(byFile[f]) == 1 {
				t := byFile[f][0].Text
				same[t] = append(same[t], f)
			}
		}
		printedSame := map[string]bool{}
		for k, f := range order {
			if out.left() < 600 {
				fmt.Fprintf(out, "\n… %d more files: %s\n", len(order)-k, strings.Join(firstN(order[k:], 10), ", "))
				break
			}
			if !discover.CodeFile(f) && len(byFile[f]) == 1 {
				if t := byFile[f][0].Text; len(same[t]) > 2 {
					if !printedSame[t] {
						printedSame[t] = true
						fmt.Fprintf(out, "\n×%d files, L%d: %s\n  %s\n", len(same[t]), byFile[f][0].Line, trimRunes(t, 160), compactFiles(same[t]))
					}
					continue
				}
			}
			fmt.Fprintf(out, "\n%s\n", f)
			for x, h := range byFile[f] {
				if x == 15 {
					fmt.Fprintf(out, "  … %d more lines\n", len(byFile[f])-15)
					break
				}
				where := ""
				if fn := in[h]; fn != "" {
					where = " (in " + fn + ")"
				}
				fmt.Fprintf(out, "  L%d%s: %s\n", h.Line, where, trimRunes(h.Text, 160))
			}
		}
	}
	if len(none) > 0 {
		fmt.Fprintf(out, "\nNo line matches in: %s; %s. An absence covers that pattern, branch and those files only: a name held in configuration under another key is not found this way.\n", strings.Join(none, "; "), scope)
	}
	return nil
}

func writeShow(out *packWriter, src *discover.Sources, repo, show string) error {
	// a.go:1-5,b.go:3-9,20-30: a part naming a file starts that file's ranges.
	groups, cur := []string{}, ""
	for _, part := range strings.Split(show, ",") {
		if strings.Trim(strings.TrimSpace(part), "0123456789-") != "" && cur != "" {
			groups, cur = append(groups, cur), ""
		}
		if cur != "" {
			cur += ","
		}
		cur += strings.TrimSpace(part)
	}
	groups = append(groups, cur)
	for k, g := range groups {
		if k > 0 {
			fmt.Fprintln(out)
		}
		if err := writeShowFile(out, src, repo, g); err != nil {
			return err
		}
	}
	return nil
}

func writeShowFile(out *packWriter, src *discover.Sources, repo, show string) error {
	path, spec := show, ""
	if k := strings.LastIndex(show, ":"); k > 0 && strings.Trim(show[k+1:], "0123456789-, ") == "" {
		path, spec = show[:k], show[k+1:]
	}
	// PATH:FROM-TO[,FROM-TO…]: each range in turn; a range that does not parse is an error, never
	// a silent narrower read.
	type span struct{ from, to int }
	spans := []span{{0, 0}}
	if spec != "" {
		spans = spans[:0]
		for _, r := range strings.Split(spec, ",") {
			ft := strings.SplitN(strings.TrimSpace(r), "-", 2)
			f, e := strconv.Atoi(ft[0])
			if e != nil || f < 1 {
				return fmt.Errorf("--show %s: %q is not a line or FROM-TO range", show, r)
			}
			t := f
			if len(ft) == 2 {
				if t, e = strconv.Atoi(ft[1]); e != nil || t < f {
					return fmt.Errorf("--show %s: %q is not a FROM-TO range", show, r)
				}
			}
			spans = append(spans, span{f, t})
		}
	}
	_, ref, _ := src.Resolve(repo)
	lang := strings.TrimPrefix(filepath.Ext(path), ".")
	header := false
	for _, sp := range spans {
		code, total, err := src.File(repo, path, sp.from, sp.to)
		if err != nil {
			return err
		}
		if !header {
			fmt.Fprintf(out, "# %s:%s at %s (%d lines)\n", repo, path, ref, total)
			header = true
		}
		kept := []string{}
		for k, l := range strings.Split(code, "\n") {
			if runeLen(strings.Join(kept, "\n"))+runeLen(l) > out.left()-300 {
				next := max(sp.from, 1) + k
				fmt.Fprintf(out, "\n```%s\n%s\n```\n… continues at L%d (`--show %s:%d-%d`, or `--budget 60000` for the rest in one read).\n", lang, strings.Join(kept, "\n"), next, path, next, min(max(next+150, sp.to), total))
				return nil
			}
			kept = append(kept, l)
		}
		fmt.Fprintf(out, "\n```%s\n%s\n```\n", lang, code)
	}
	return nil
}

// compactFiles groups paths by directory: kustomization/production/{env-a,env-b}, …
func compactFiles(paths []string) string {
	byDir, dirs := map[string][]string{}, []string{}
	for _, p := range paths {
		d, b := filepath.Dir(p)+"/", filepath.Base(p)
		if _, ok := byDir[d]; !ok {
			dirs = append(dirs, d)
		}
		byDir[d] = append(byDir[d], b)
	}
	out := []string{}
	for _, d := range dirs {
		if len(byDir[d]) == 1 {
			out = append(out, d+byDir[d][0])
		} else {
			out = append(out, d+"{"+strings.Join(byDir[d], ",")+"}")
		}
	}
	return strings.Join(out, ", ")
}

// callChain follows the callers of a function levels up: name ← caller (file:line) ← its caller …,
// branching into brackets when a function has several callers.
func callChain(src *discover.Sources, repo, name, file string, levels int) string {
	seen := map[string]bool{name: true}
	var up func(string, string, int) string
	up = func(fn, file string, level int) string {
		if level == 0 {
			return ""
		}
		parts := []string{}
		for _, c := range src.Callers(repo, fn, file, 6) {
			label := fmt.Sprintf("%s:%d", c.Path, c.Line)
			if c.In == "" || seen[c.In] {
				parts = append(parts, label)
				continue
			}
			seen[c.In] = true
			parts = append(parts, c.In+" ("+label+")"+up(c.In, c.Path, level-1))
		}
		if len(parts) == 0 {
			return ""
		}
		if len(parts) == 1 {
			return " ← " + parts[0]
		}
		return " ← [" + strings.Join(parts, " | ") + "]"
	}
	chain := up(name, file, levels)
	if chain == "" {
		return name + " (no caller in this repository)"
	}
	return name + chain
}

// writeSymbol answers --func for a name that is not a function (a constant, a variable, a type, a
// configuration key): where it is declared, then every use grouped by the function it is in.
func writeSymbol(out *packWriter, src *discover.Sources, repo, ref string, o CodeOptions) error {
	name := o.Func
	hits, in, searched, err := src.Search(repo, regexp.QuoteMeta(name), o.Path, false, false, true)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		return fmt.Errorf("%s does not appear in %s at %s (%d files searched, tests left out)", name, repo, ref, searched)
	}
	decl := regexp.MustCompile(`\b(const|var|let|type|enum|static|final|val|def|class|interface|struct)\b[^=]*\b` + regexp.QuoteMeta(name) + `\b|^\s*["']?` + regexp.QuoteMeta(name) + `["']?\s*(=|:=|:)[^=]`)
	fmt.Fprintf(out, "# %s in %s at %s — not a function: its declaration and every use (%d lines, %d files searched, tests left out)\n", name, repo, ref, len(hits), searched)
	uses := map[string][]discover.Hit{}
	order := []string{}
	for _, h := range hits {
		if decl.MatchString(h.Text) {
			ctx, _, _ := src.File(repo, h.Path, h.Line, h.Line+2)
			fmt.Fprintf(out, "\nDeclared at %s:%d\n```%s\n%s\n```\n", h.Path, h.Line, strings.TrimPrefix(filepath.Ext(h.Path), "."), ctx)
			continue
		}
		key := h.Path
		if fn := in[h]; fn != "" {
			key += " (in " + fn + ")"
		}
		if _, ok := uses[key]; !ok {
			order = append(order, key)
		}
		uses[key] = append(uses[key], h)
	}
	fmt.Fprintf(out, "\nUsed in %d places:\n", len(order))
	for _, k := range order {
		if out.left() < 400 {
			fmt.Fprintln(out, "… more (`--grep`).")
			break
		}
		for _, h := range uses[k] {
			fmt.Fprintf(out, "- %s:%d: %s\n", k, h.Line, trimRunes(h.Text, 150))
		}
	}
	return nil
}

func writeFunc(out *packWriter, src *discover.Sources, repo string, o CodeOptions) error {
	name, receiver := o.Func, ""
	if k := strings.LastIndex(name, "."); k > 0 {
		receiver, name = name[:k], name[k+1:] // ReInject.Handle: the method of that type only
		o.Func = name
	}
	defs, err := src.Definition(repo, name, o.Path, 150)
	if receiver != "" {
		kept := defs[:0]
		for _, d := range defs {
			if strings.Contains(d.Name, receiver) || strings.Contains(strings.ToLower(d.Path), strings.ToLower(receiver)) {
				kept = append(kept, d)
			}
		}
		defs = kept
	}
	if err != nil {
		return err
	}
	_, ref, _ := src.Resolve(repo)
	if len(defs) == 0 {
		return writeSymbol(out, src, repo, ref, o)
	}
	fmt.Fprintf(out, "# %s in %s at %s\n", name, repo, ref)
	if len(defs) > 1 {
		// Several definitions share the name: say so first, so the reader picks the right one.
		ds := []string{}
		for _, d := range defs {
			ds = append(ds, fmt.Sprintf("%s:%d `%s`", d.Path, d.From, trimRunes(d.Name, 100)))
		}
		fmt.Fprintf(out, "\n%d definitions with this name (`--path GLOB` keeps one): %s\n", len(defs), strings.Join(ds, " · "))
	}
	lastCallers := ""
	for _, d := range defs {
		if o.Up > 0 {
			fmt.Fprintf(out, "\nCall chain up from %s:%d: %s\n", d.Path, d.From, callChain(src, repo, name, d.Path, o.Up))
			// Each hop of the first path up, with what the caller does with the result.
			fn, file := name, d.Path
			for level := 1; level <= o.Up; level++ {
				cs := src.Callers(repo, fn, file, 1)
				if len(cs) == 0 || cs[0].In == "" {
					break
				}
				c := cs[0]
				fmt.Fprintf(out, "\nUp %d: %s (%s:%d) calls %s\n```%s\n%s\n```\n", level, c.In, c.Path, c.Line, fn, strings.TrimPrefix(filepath.Ext(c.Path), "."), src.CallContext(repo, c, 4))
				fn, file = c.In, c.Path
			}
		}
		fmt.Fprintf(out, "\n## %s#L%d-L%d\n\n```%s\n%s\n```\n", d.Path, d.From, d.To, strings.TrimPrefix(filepath.Ext(d.Path), "."), d.Code)
		if exits := exitLines(d, nil, ref, 14); exits != "" {
			fmt.Fprint(out, "\n"+exits)
		}
		if o.Down {
			writeCallees(out, src, repo, ref, d)
		}
		// The settings the function reads, with the values configuration gives them.
		settings, seen := []string{}, map[string]bool{}
		for _, s := range settingsIn(d.Code) {
			if !seen[s] {
				seen[s] = true
				settings = append(settings, s)
			}
		}
		if len(settings) > 0 {
			_, all, _ := src.Grep(repo, settings, 1)
			vals, again, unset := []string{}, []string{}, []string{}
			for _, s := range settings {
				if o.shown != nil && o.shown[s] {
					again = append(again, "`"+s+"`")
					continue
				}
				if v := configValues(s, all[s]); v != "" {
					vals = append(vals, "`"+s+"`: "+v)
					if o.shown != nil {
						o.shown[s] = true
					}
				} else {
					unset = append(unset, "`"+s+"`")
				}
			}
			if len(again) > 0 {
				vals = append(vals, strings.Join(again, ", ")+" as above")
			}
			if len(unset) > 0 {
				vals = append(vals, strings.Join(firstN(unset, 6), ", ")+": no value in this repository's configuration (set at deploy time, or not at all)")
			}
			if len(vals) > 0 {
				fmt.Fprintf(out, "\nSettings it reads (values as the repository's configuration files version them, not as deployed): %s\n", strings.Join(firstN(vals, 8), " · "))
			}
			// A setting holding a URL: which repository of the vault declares that route.
			for _, st := range settings {
				if line := routeServers(src, repo, st, all[st]); line != "" {
					fmt.Fprint(out, line)
				}
			}
		}
		if len(d.Callers) == 0 {
			fmt.Fprintln(out, "\nCalled from: nowhere by name in this repository's code at this branch (tests left out).")
			fmt.Fprint(out, registrations(src, repo, d))
			continue
		}
		// Definitions sharing a name share their callers as far as a text search can tell.
		sig := ""
		for _, c := range d.Callers {
			sig += fmt.Sprintf("%s:%d;", c.Path, c.Line)
		}
		if sig == lastCallers {
			fmt.Fprintf(out, "\nCalled from: the same %d places as the definition above.\n", len(d.Callers))
			continue
		}
		lastCallers = sig
		// Every caller is listed; the first ones with the lines after the call.
		fmt.Fprintf(out, "\nCalled from %d places (tests left out; the first ones with the lines after the call: what the caller does with the result):\n", len(d.Callers))
		if d.Rivals > 0 {
			fmt.Fprintf(out, "%s has %d other definitions in this repository: a call through an interface or a variable may reach any of them.\n", name, d.Rivals)
		}
		shown := 0
		for _, c := range d.Callers {
			if shown == funcCallContexts || out.left() < 1500 {
				break
			}
			in := ""
			if c.In != "" {
				in = " in " + c.In + " (`--func " + c.In + "` goes one level up)"
			}
			fmt.Fprintf(out, "\n%s:%d%s\n```%s\n%s\n```\n", c.Path, c.Line, in, strings.TrimPrefix(filepath.Ext(c.Path), "."), src.CallContext(repo, c, 4))
			shown++
		}
		if rest := d.Callers[shown:]; len(rest) > 0 {
			rows := []string{}
			for _, c := range rest {
				row := fmt.Sprintf("%s:%d", c.Path, c.Line)
				if c.In != "" {
					row += " in " + c.In
				}
				rows = append(rows, row+" `"+trimRunes(strings.ReplaceAll(c.Text, "`", "'"), 80)+"`")
			}
			fmt.Fprintf(out, "\nAlso called from (%d):\n- %s\n", len(rest), strings.Join(rows, "\n- "))
		}
		writeWithout(out, src, repo, name, d.Callers)
	}
	return nil
}

// funcCallContexts is how many callers --func shows with the lines after the call.
const funcCallContexts = 6

// writeCallees lists the repository's functions a function calls, in call order, each with where
// it is defined and its exits: what happens below the call without opening every callee.
func writeCallees(out *packWriter, src *discover.Sources, repo, ref string, d discover.Function) {
	callees := src.Callees(repo, d, 12)
	name := discover.DefinedName(d.Name)
	if len(callees) == 0 {
		fmt.Fprintf(out, "\nCalls down from %s: no function of this repository (library and built-in calls left out).\n", name)
		return
	}
	fmt.Fprintf(out, "\nCalls down from %s to this repository's functions, in call order (`--func NAME` shows one whole):\n", name)
	ends := processEnders(src, repo)
	for _, c := range callees {
		if out.left() < 600 {
			fmt.Fprintln(out, "- … more calls (`--budget`).")
			return
		}
		if len(c.Defs) > 2 {
			ds := []string{}
			for _, f := range c.Defs {
				ds = append(ds, fmt.Sprintf("%s:%d", f.Path, f.Start))
			}
			fmt.Fprintf(out, "\nL%d %s → %d definitions (an interface method or a shared name; `--func %s --path GLOB` reads one): %s\n", c.Line, c.Name, len(c.Defs), c.Name, strings.Join(firstN(ds, 6), " · "))
			continue
		}
		for _, f := range c.Defs {
			fmt.Fprintf(out, "\nL%d %s → %s#L%d-L%d\n", c.Line, c.Name, f.Path, f.Start, f.End)
			if end := endsThrough(f, ends); end != "" || ends[discover.DefinedName(f.Name)] {
				if end == "" {
					end = "ends the process"
				}
				fmt.Fprintf(out, "- %s\n", end)
			}
			if exits := exitLines(f, nil, ref, 6); exits != "" {
				fmt.Fprint(out, exits[strings.Index(exits, "\n")+1:]) // its header repeats the name
			} else {
				fmt.Fprintln(out, "- no exit but its end")
			}
		}
	}
}

var passedAs = regexp.MustCompile(`^\s*(?:await\s+|return\s+)?([\w$.]+)\(`)

// writeWithout: when the function is passed as a value in same-shaped lines (app.get('/x',
// authMiddleware, handler)), the lines of that shape in the same files that do not pass it: the
// routes a middleware leaves out. A registration for every line (app.use(authMiddleware)) says so.
func writeWithout(out *packWriter, src *discover.Sources, repo, name string, callers []discover.Hit) {
	shapes := map[string]map[string]bool{} // file → call prefixes it is passed to
	for _, c := range callers {
		m := passedAs.FindStringSubmatch(c.Text)
		if m == nil || strings.Contains(c.Text, name+"(") || m[1] == name {
			continue
		}
		if strings.HasSuffix(m[1], ".use") {
			fmt.Fprintf(out, "\n%s is registered for every following line at %s:%d (`%s`).\n", name, c.Path, c.Line, trimRunes(c.Text, 80))
			return
		}
		if shapes[c.Path] == nil {
			shapes[c.Path] = map[string]bool{}
		}
		shapes[c.Path][m[1]] = true
	}
	files := []string{}
	for f := range shapes {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		code, _, err := src.File(repo, f, 0, 0)
		if err != nil {
			continue
		}
		rows := []string{}
		for _, l := range strings.Split(code, "\n") {
			k := strings.Index(l, "│ ")
			if k < 0 {
				continue
			}
			text := strings.TrimSpace(l[k+len("│ "):])
			m := passedAs.FindStringSubmatch(text)
			if m == nil || !shapes[f][m[1]] || discover.WholeWord(text, name) {
				continue
			}
			rows = append(rows, "L"+strings.TrimSpace(l[:k])+" `"+trimRunes(strings.ReplaceAll(text, "`", "'"), 90)+"`")
		}
		if len(rows) > 0 {
			prefixes := []string{}
			for p := range shapes[f] {
				prefixes = append(prefixes, p+"(")
			}
			sort.Strings(prefixes)
			fmt.Fprintf(out, "\nSame-shaped lines in %s that do not pass %s (%s), %d:\n- %s\n", f, name, strings.Join(prefixes, ", "), len(rows), strings.Join(firstN(rows, 20), "\n- "))
		}
	}
}

var (
	urlValue     = regexp.MustCompile(`https?://[^\s"'<>]+`)
	versionPart  = regexp.MustCompile(`^v\d+$`)
	routeLiteral = regexp.MustCompile(`["'` + "`" + `]/?([\w\-{}:/.]*)["'` + "`" + `]`)
)

// routeServers: for a setting whose values are URLs, the lines of the vault's repositories that
// declare the URL's path: its last two segments (a version left out) in code or an API definition,
// or the last one as a route literal in a file under the one before (controllers/auth/router.ts:
// .post('/validate')). Repositories named like the URL's segments (a gateway proxy
// gw-orders-base → orders-base-http-adapter) are searched first, the others only if they
// declare nothing.
func routeServers(src *discover.Sources, repo, setting string, hits []discover.Hit) string {
	type route struct {
		parent, last string
		words        map[string]bool
	}
	seen, routes := map[string]bool{}, []route{}
	for _, h := range hits {
		for _, u := range urlValue.FindAllString(h.Text, -1) {
			rest := u[strings.Index(u, "//")+2:]
			k := strings.Index(rest, "/")
			if k < 0 {
				continue
			}
			parts, words := []string{}, map[string]bool{}
			for _, p := range strings.Split(strings.Trim(rest[k:], "/"), "/") {
				if p != "" && !versionPart.MatchString(p) && !strings.ContainsAny(p, "?#$…") {
					parts = append(parts, p)
					for _, w := range strings.FieldsFunc(strings.ToLower(p), func(c rune) bool { return c == '-' || c == '_' || c == '.' }) {
						words[w] = true
					}
				}
			}
			if len(parts) < 2 || seen[strings.Join(parts[len(parts)-2:], "/")] {
				continue
			}
			seen[strings.Join(parts[len(parts)-2:], "/")] = true
			routes = append(routes, route{parts[len(parts)-2], parts[len(parts)-1], words})
		}
	}
	if len(routes) == 0 {
		return ""
	}
	r0 := routes[0]
	tail := r0.parent + "/" + r0.last
	// Repositories named like the URL first.
	repos := discover.Tracked(src.Vault())
	overlap := func(name string) int {
		n := 0
		for _, w := range strings.FieldsFunc(strings.ToLower(name), func(c rune) bool { return c == '-' || c == '_' || c == '.' }) {
			if r0.words[w] {
				n++
			}
		}
		return n
	}
	near, far := []string{}, []string{}
	for _, r := range repos {
		if r == repo {
			continue
		}
		if overlap(r) >= 2 {
			near = append(near, r)
		} else {
			far = append(far, r)
		}
	}
	sort.SliceStable(near, func(a, b int) bool { return overlap(near[a]) > overlap(near[b]) })
	pattern := regexp.QuoteMeta(tail) + `|["'` + "`" + `]/?` + regexp.QuoteMeta(r0.last) + `["'` + "`" + `]`
	find := func(list []string) ([]string, int) {
		rows, searched := []string{}, 0
		for _, r := range list {
			found, in, _, err := src.Search(r, pattern, "", false, false, false)
			if err != nil {
				continue
			}
			searched++
			for _, h := range found {
				declares := strings.Contains(h.Text, tail) && (discover.CodeFile(h.Path) || apiDefinition.MatchString(h.Path)) && !urlValue.MatchString(h.Text) && !strings.Contains(h.Text, "${") ||
					discover.CodeFile(h.Path) && strings.Contains(strings.ToLower(h.Path), strings.ToLower(r0.parent)) && !strings.Contains(h.Text, tail)
				if !declares {
					continue
				}
				row := fmt.Sprintf("%s %s:%d", r, h.Path, h.Line)
				if in[h] != "" {
					row += " in " + in[h]
				}
				rows = append(rows, row+" `"+trimRunes(strings.ReplaceAll(h.Text, "`", "'"), 80)+"`")
			}
		}
		return rows, searched
	}
	rows, searched := find(near)
	if len(rows) == 0 {
		var n int
		rows, n = find(far)
		searched += n
	}
	if len(rows) == 0 {
		return fmt.Sprintf("`%s` calls …/%s: no code or API definition of the %d other repositories with a checkout declares that path (a gateway may rewrite it).\n", setting, tail, searched)
	}
	return fmt.Sprintf("`%s` calls …/%s, declared in: %s\n", setting, tail, strings.Join(firstN(rows, 6), " · "))
}

var apiDefinition = regexp.MustCompile(`(?i)(openapi|swagger|governance|api[\w\-]*)\.(ya?ml|json)$`)

// registrations: for a function nothing calls by name, where its class is provided, registered or
// constructed, so "nowhere" does not hide a framework call (an interceptor, a handler).
func registrations(src *discover.Sources, repo string, f discover.Function) string {
	class, hits := src.Registrations(repo, f, 6)
	if class == "" {
		return ""
	}
	if len(hits) == 0 {
		return fmt.Sprintf("Its type %s is not named outside its own file either.\n", class)
	}
	rows := []string{}
	for _, h := range hits {
		rows = append(rows, fmt.Sprintf("%s:%d `%s`", h.Path, h.Line, trimRunes(strings.ReplaceAll(h.Text, "`", "'"), 90)))
	}
	return fmt.Sprintf("Its type %s is used at (a framework may call it through these): %s\n", class, strings.Join(rows, " · "))
}
