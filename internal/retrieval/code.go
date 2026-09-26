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
	Grep       string // extended regular expression
	Show       string // path[:FROM-TO]
	Func       string // function or method name
	Path       string // glob limiting --grep and --func to some files
	Up         int    // --func: follow the callers this many levels up
	IgnoreCase bool
	Tests      bool
	Budget     int
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
	return writeFunc(out, src, repos[0], o)
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
	fmt.Fprintf(out, "# Code search /%s/%s\n", o.Grep, map[bool]string{true: "i", false: ""}[o.IgnoreCase])
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
	path, from, to := show, 0, 0
	if k := strings.LastIndex(show, ":"); k > 0 {
		r := strings.SplitN(show[k+1:], "-", 2)
		if f, e := strconv.Atoi(r[0]); e == nil {
			path, from, to = show[:k], f, f
			if len(r) == 2 {
				if t, e := strconv.Atoi(r[1]); e == nil {
					to = t
				}
			}
		}
	}
	code, total, err := src.File(repo, path, from, to)
	if err != nil {
		return err
	}
	_, ref, _ := src.Resolve(repo)
	lines := strings.Split(code, "\n")
	kept := []string{}
	for k, l := range lines {
		if runeLen(strings.Join(kept, "\n"))+runeLen(l) > out.left()-300 {
			next := max(from, 1) + k
			fmt.Fprintf(out, "# %s:%s at %s (%d lines)\n\n```%s\n%s\n```\n… continues at L%d (`--show %s:%d-%d`, or `--budget 60000` for the rest in one read).\n", repo, path, ref, total, strings.TrimPrefix(filepath.Ext(path), "."), strings.Join(kept, "\n"), next, path, next, min(next+150, total))
			return nil
		}
		kept = append(kept, l)
	}
	fmt.Fprintf(out, "# %s:%s at %s (%d lines)\n\n```%s\n%s\n```\n", repo, path, ref, total, strings.TrimPrefix(filepath.Ext(path), "."), code)
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
		// The settings the function reads, with the values configuration gives them.
		settings, seen := []string{}, map[string]bool{}
		for _, s := range settingName.FindAllString(d.Code, -1) {
			if !seen[s] {
				seen[s] = true
				settings = append(settings, s)
			}
		}
		if len(settings) > 0 {
			_, all, _ := src.Grep(repo, settings, 1)
			vals := []string{}
			for _, s := range settings {
				if v := configValues(s, all[s]); v != "" {
					vals = append(vals, "`"+s+"`: "+v)
				}
			}
			if len(vals) > 0 {
				fmt.Fprintf(out, "\nSettings it reads: %s\n", strings.Join(firstN(vals, 8), " · "))
			}
		}
		if len(d.Callers) == 0 {
			fmt.Fprintln(out, "\nCalled from: nowhere in this repository's code at this branch (tests left out).")
			continue
		}
		fmt.Fprintln(out, "\nCalled from (each call with the lines after it: what the caller does with the result):")
		for _, c := range d.Callers {
			if out.left() < 800 {
				fmt.Fprintln(out, "… more callers (`--grep`).")
				break
			}
			in := ""
			if c.In != "" {
				in = " in " + c.In + " (`--func " + c.In + "` goes one level up)"
			}
			fmt.Fprintf(out, "\n%s:%d%s\n```%s\n%s\n```\n", c.Path, c.Line, in, strings.TrimPrefix(filepath.Ext(c.Path), "."), src.CallContext(repo, c, 4))
		}
	}
	return nil
}
