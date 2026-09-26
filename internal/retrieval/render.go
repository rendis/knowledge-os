package retrieval

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"knowledge-os/internal/discover"
)

// Rendering shared by `ask` and `read`: a bounded writer, cited sources with their state on the
// reference branch and, while the budget allows, the cited lines themselves.

const (
	codeLines     = 12
	footnoteRunes = 200
	citedPerText  = 6
)

var (
	footnoteRef  = regexp.MustCompile(`\[\^([^\]\s]+)\]`)
	permalinkMD  = regexp.MustCompile(`\[[^\]]*\]\((https://github\.com/[^/\s]+/([^/\s]+)/blob/([0-9a-f]{7,40})/([^)#\s]+)(#L\d+(?:-L\d+)?)?)\)`)
	backtickPath = regexp.MustCompile("`(\\.?/?[\\w.\\-]+(?:/[\\w.\\-]+)+\\.[A-Za-z0-9]{1,6})`")
	codeName     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	symbolToken  = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.]*(?:\(\))?`)
	textLines    = regexp.MustCompile(`(?i)(?:\bl[íi]neas?\s+|\bL)(\d+)(?:\s*[-–]\s*L?(\d+))?`)
	permalinkRaw = regexp.MustCompile(`https://github\.com/[^/\s]+/([^/\s]+)/blob/([0-9a-f]{7,40})/([^)#\s]+)(#L\d+(?:-L\d+)?)?`)
)

// packWriter counts the characters written, the unit a harness truncates tool output by.
type packWriter struct {
	w      io.Writer
	n      int
	budget int
	cut    bool
}

// Write never lets the output pass its budget: what does not fit is dropped with one marker, so a
// harness always shows the whole pack.
func (p *packWriter) Write(b []byte) (int, error) {
	if p.cut {
		return len(b), nil
	}
	runes := utf8.RuneCount(b)
	if p.budget > 0 && p.n+runes > p.budget {
		keep := []rune(string(b))[:max(0, p.budget-p.n-120)]
		_, e := p.w.Write([]byte(string(keep) + "\n… [cut at the budget: ask again with --focus NOTE, --brief or a larger --budget]\n"))
		p.n, p.cut = p.budget, true
		return len(b), e
	}
	n, e := p.w.Write(b)
	p.n += utf8.RuneCount(b[:n])
	return n, e
}

func (p *packWriter) left() int { return p.budget - p.n }

// sourceRenderer writes the footnotes a text cites, each with the state of its anchors and, while
// the budget allows, the cited lines.
type sourceRenderer struct {
	src      *discover.Sources
	code     bool
	codeLeft int  // characters of cited lines the output may still carry
	perText  int  // footnotes resolved per text; the rest are listed by id
	brief    bool // one line of source marks instead of the footnotes
	covered  map[string]bool
}

// writeCited writes the footnotes cited in body, resolved from defs (the note's footnote definitions),
// once per footnote in seen.
func (r *sourceRenderer) writeCited(out *packWriter, body string, defs map[string]string, seen map[string]bool, repo, commit string) {
	cites, more := r.cited(body, defs, seen, repo, commit, r.perText, out.left()-400)
	if r.brief {
		if len(cites)+len(more) > 0 {
			fmt.Fprintln(out, "\n"+briefSources(cites, more))
		}
		return
	}
	lines := []string{}
	for _, c := range cites {
		lines = append(lines, c.line)
		if !r.code {
			continue
		}
		if code, ok := r.snippet(c.def, focusTerms(body)); ok && runeLen(code) <= r.codeLeft && runeLen(code) < out.left()-runeLen(strings.Join(lines, "\n"))-400 {
			r.codeLeft -= runeLen(code)
			lines = append(lines, code)
		}
	}
	if len(more) > 0 {
		lines = append(lines, alsoCited(more))
	}
	if len(lines) > 0 {
		fmt.Fprintln(out, "\nSources:\n"+strings.Join(lines, "\n"))
	}
}

// citation is one footnote a text cites, rendered as a line with its state.
type citation struct {
	id, def, line, mark string
}

// cited resolves the footnotes a text cites, not yet seen, up to limit of them and room characters;
// the others are returned by id.
func (r *sourceRenderer) cited(body string, defs map[string]string, seen map[string]bool, repo, commit string, limit, room int) ([]citation, []string) {
	out, more, used := []citation{}, []string{}, 0
	for _, m := range footnoteRef.FindAllStringSubmatch(body, -1) {
		id := m[1]
		d, ok := defs[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		line, mark := r.footnoteLine(id, d, repo, commit)
		if len(out) >= limit || used+runeLen(line) > room {
			more = append(more, "[^"+id+"]")
			continue
		}
		used += runeLen(line) + 1
		out = append(out, citation{id, d, line, mark})
	}
	return out, more
}

func alsoCited(ids []string) string {
	return "- also cited: " + strings.Join(ids, " ") + " (`kos read --note NAME --lines FROM-TO` resolves them)"
}

// footnoteLine is a footnote with the state of its anchors: ✓ the cited lines, ✓ file when the
// link cites a whole file (it did not change, but no lines were checked), ⚠ or ?.
func (r *sourceRenderer) footnoteLine(id, def, repo, commit string) (string, string) {
	mark := ""
	anchors := discover.Anchors(def)
	for _, a := range anchors {
		st := r.src.State(a)
		switch {
		case st.Status == "changed":
			mark = "⚠ changed on " + st.Ref + " since " + short(a.Commit)
		case st.Status == "unknown" && mark == "":
			mark = "? " + st.Detail
		case st.Status == "current" && mark == "":
			mark = "✓"
			if a.From == 0 && !textLines.MatchString(def) {
				mark = "✓ file"
			}
		}
	}
	line := "- [^" + id + "] "
	if mark != "" {
		line += mark + " "
	}
	return line + trimRunes(compactPermalinks(def, repo, commit), footnoteRunes), mark
}

// briefSources is one line of citation states: [^e15] ✓ · [^e3] ✓ file · [^e7] ⚠ changed….
func briefSources(cites []citation, more []string) string {
	parts := []string{}
	for _, c := range cites {
		m := c.mark
		if strings.HasPrefix(m, "?") {
			m = "?"
		}
		parts = append(parts, "[^"+c.id+"] "+m)
	}
	parts = append(parts, more...)
	return "Sources: " + strings.Join(parts, " · ")
}

// snippet returns the cited lines of a footnote's first readable anchor: its line range, the range
// its text names ("líneas 13-26", "L88-L101"), or the definition of a symbol it names; within them,
// the lines around the first literal of focus (what the claim is about).
func (r *sourceRenderer) snippet(def string, focus []string) (string, bool) {
	anchors := discover.Anchors(def)
	if len(anchors) == 1 && anchors[0].From == 0 {
		if m := textLines.FindStringSubmatch(def); m != nil {
			anchors[0].From, _ = strconv.Atoi(m[1])
			anchors[0].To = anchors[0].From
			if m[2] != "" {
				anchors[0].To, _ = strconv.Atoi(m[2])
			}
		}
	}
	for _, a := range anchors {
		max := codeLines
		if !discover.CodeFile(a.Path) {
			max = 3 // a configuration value needs its line, not the file around it
		}
		if code, label, ok := r.src.Excerpt(a, symbolNames(def), focus, max); ok {
			lang := strings.TrimPrefix(filepath.Ext(a.Path), ".")
			return fmt.Sprintf("  ```%s\n  // %s\n%s\n  ```", lang, label, indent(code, "  ")), true
		}
	}
	return "", false
}

// focusTerms are the literals a paragraph's claim is about: its identifiers and backticked names.
func focusTerms(text string) []string {
	out, seen := []string{}, map[string]bool{}
	for _, m := range regexp.MustCompile("`([^`]{3,80})`").FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	for _, w := range regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{3,}`).FindAllString(text, -1) {
		if !seen[w] && (identifier(w) || strings.Contains(w, "_")) {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// filesLine checks the repository paths a text names in backticks when it cites no footnote for
// them: whether each exists at the note's analyzed commit and still reads the same on the reference
// branch. Notes that cite files by path get the same check as notes that cite permalinks.
func (r *sourceRenderer) filesLine(body, repo, commit string) string {
	if repo == "" || commit == "" {
		return ""
	}
	marks := []string{}
	seen := map[string]bool{}
	for _, m := range backtickPath.FindAllStringSubmatch(body, -1) {
		p := strings.TrimPrefix(m[1], "./")
		if seen[p] || len(marks) == 8 {
			continue
		}
		seen[p] = true
		a := discover.Anchor{Repo: repo, Commit: commit, Path: p}
		if !r.src.Has(a) {
			continue // a path of another repository or a route, not a file of this one
		}
		st := r.src.State(a)
		mark := "✓"
		switch st.Status {
		case "changed":
			mark = "⚠ changed on " + st.Ref
		case "unknown":
			mark = "?"
		}
		marks = append(marks, "`"+p+"` "+mark)
	}
	if len(marks) == 0 {
		return ""
	}
	return "Files named: " + strings.Join(marks, " · ")
}

// symbolNames returns the code names a footnote's text mentions after its link, in order: words
// with a capital or an underscore (Handle, FindByOrderId, publish_ack), the last segment of a
// dotted one (s.useCase.Handle → Handle).
func symbolNames(def string) []string {
	text := def
	if locs := permalinkMD.FindAllStringIndex(text, -1); len(locs) > 0 {
		text = text[locs[len(locs)-1][1]:]
	}
	out, seen := []string{}, map[string]bool{}
	add := func(n string) {
		if k := strings.LastIndex(n, "."); k >= 0 {
			n = n[k+1:]
		}
		n = strings.TrimSuffix(n, "()")
		if len(n) >= 3 && !seen[n] && codeName.MatchString(n) {
			seen[n] = true
			out = append(out, n)
		}
	}
	// In the order the text names them: a capital or an underscore marks a code name.
	for _, w := range symbolToken.FindAllString(text, -1) {
		if strings.IndexFunc(w, func(c rune) bool { return c == '_' || c >= 'A' && c <= 'Z' }) >= 0 {
			add(w)
		}
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

// writeCode greps the note's repository at its reference branch for the code names the shown
// paragraphs mention, so what the note says can be checked against today's code in the same call.
func (r *sourceRenderer) writeCode(out *packWriter, repo string, names []string, fromQuery map[string]bool) {
	if repo == "" || len(names) == 0 || !r.code {
		return
	}
	hits, all, ref := r.src.Grep(repo, names, 3)
	lines := []string{}
	for _, n := range names {
		hs := hits[n]
		if len(hs) == 0 {
			if !fromQuery[n] {
				lines = append(lines, "- `"+n+"` — not in "+repo+" at "+ref+": the paragraph may describe an older version")
			}
			continue
		}
		parts := []string{}
		for _, h := range hs {
			in := ""
			if h.In != "" {
				in = " (in " + h.In + ")"
			}
			parts = append(parts, fmt.Sprintf("%s:%d%s `%s`", h.Path, h.Line, in, trimRunes(strings.ReplaceAll(h.Text, "`", "'"), 110)))
		}
		line := "- `" + n + "` — " + strings.Join(parts, " · ")
		if v := configValues(n, all[n]); v != "" {
			line += "\n  values: " + v
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return
	}
	text := fmt.Sprintf("\nIn the code of %s at %s (whole-word grep, tests left out):\n%s\n", repo, ref, strings.Join(lines, "\n"))
	if runeLen(text) <= r.codeLeft && runeLen(text) < out.left()-200 {
		r.codeLeft -= runeLen(text)
		fmt.Fprint(out, text)
	}
}

var testPath = regexp.MustCompile(`(?i)(^|/)(tests?|__tests__|fixtures?|mocks?|testdata|examples?|samples?)(/|$)|[._-](test|spec|mock|fixture|example|sample)[._-]`)

var configAssign = regexp.MustCompile(`^\s*-?\s*["']?([A-Za-z0-9_.]+)["']?\s*[:=]\s*["']?([^"'#]*?)["']?\s*(?:#.*)?$`)

// configValues groups the values a name is given across configuration files: "6 in production/env-a,
// env-b; 3 in production/env-c".
func configValues(name string, hits []discover.Hit) string {
	byValue, order := map[string][]string{}, []string{}
	for _, h := range hits {
		if discover.CodeFile(h.Path) || testPath.MatchString(h.Path) {
			continue // code uses the name; tests and fixtures do not configure a deployment
		}
		m := configAssign.FindStringSubmatch(h.Text)
		if m == nil || m[1] != name || strings.Trim(m[2], " {}[],") == "" {
			continue
		}
		v := strings.TrimSpace(m[2])
		if _, ok := byValue[v]; !ok {
			order = append(order, v)
		}
		byValue[v] = append(byValue[v], h.Path)
	}
	if len(order) == 0 {
		return ""
	}
	// Paths without their shared directory, production first: "6 in production/env-a, uat/env-a".
	all := []string{}
	for _, v := range order {
		all = append(all, byValue[v]...)
	}
	prefix := commonDir(all)
	parts := []string{}
	for _, v := range order {
		ps := []string{}
		for _, p := range byValue[v] {
			ps = append(ps, strings.TrimPrefix(p, prefix))
		}
		parts = append(parts, fmt.Sprintf("%s in %s", trimRunes(v, 60), compactPaths(ps)))
	}
	return strings.Join(firstN(parts, 8), "; ")
}

// compactPaths writes paths grouped by directory, production first: production/{env-a,env-b}, uat/env-a.
func compactPaths(paths []string) string {
	byDir, dirs := map[string][]string{}, []string{}
	for _, p := range paths {
		d, b := "", p
		if k := strings.LastIndex(p, "/"); k >= 0 {
			d, b = p[:k+1], p[k+1:]
		}
		if _, ok := byDir[d]; !ok {
			dirs = append(dirs, d)
		}
		byDir[d] = append(byDir[d], b)
	}
	sort.SliceStable(dirs, func(a, b int) bool { return strings.Contains(dirs[a], "prod") && !strings.Contains(dirs[b], "prod") })
	out := []string{}
	for _, d := range dirs {
		if bs := byDir[d]; len(bs) == 1 {
			out = append(out, d+bs[0])
		} else {
			out = append(out, d+"{"+strings.Join(bs, ",")+"}")
		}
	}
	return strings.Join(out, ", ")
}

// commonDir is the directory prefix every path shares ("kustomization/"), or "".
func commonDir(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	prefix := paths[0][:strings.LastIndex(paths[0], "/")+1]
	for _, p := range paths[1:] {
		for !strings.HasPrefix(p, prefix) && prefix != "" {
			prefix = prefix[:strings.LastIndex(strings.TrimSuffix(prefix, "/"), "/")+1]
		}
	}
	return prefix
}

// writeMatchingCode shows the functions of the repository at its reference branch that hold the
// most of the question's words: the code that answers "what happens when", not only the note's words.
func (r *sourceRenderer) writeMatchingCode(out *packWriter, repo string, terms []askTerm, prefer map[string]bool) []string {
	if !r.code {
		return nil
	}
	settings, seen := []string{}, map[string]bool{}
	words := []string{}
	for _, t := range terms {
		if len(t.phrase) == 0 && runeLen(t.stem) >= 4 {
			words = append(words, t.stem)
		}
	}
	fns, ref := r.src.Functions(repo, words, 2, 80, prefer)
	for k, f := range fns {
		if k > 0 {
			// the second function gets fewer lines: it is context, the first is the answer
			lines := strings.Split(f.Code, "\n")
			if len(lines) > 16 {
				f.Code = strings.Join(append(lines[:16], "    │ …"), "\n")
			}
		}
		if len(f.Words) < 2 && len(words) > 1 && !prefer[f.Path] {
			continue // one common word in an uncited function says nothing about the question
		}
		lang := strings.TrimPrefix(filepath.Ext(f.Path), ".")
		text := fmt.Sprintf("\nCode matching %s in %s at %s — %s#L%d-L%d:\n```%s\n%s\n```\n", strings.Join(f.Words, ", "), repo, ref, f.Path, f.From, f.To, lang, f.Code)
		if len(f.Outside) > 0 {
			ls := []string{}
			for _, l := range f.Outside {
				ls = append(ls, fmt.Sprint(l))
			}
			text += fmt.Sprintf("Also matching in this function: L%s (`git -C <checkout> show %s:%s`).\n", strings.Join(firstN(ls, 12), ","), ref, f.Path)
		}
		if len(f.Callers) == 0 && f.Name != "" {
			text += "Called from: nowhere in this repository's code at " + ref + " (tests left out).\n"
		}
		if len(f.Callers) > 0 {
			cs := []string{}
			for _, c := range f.Callers {
				in := ""
				if c.In != "" {
					in = " in " + c.In
				}
				cs = append(cs, fmt.Sprintf("%s:%d%s `%s`", c.Path, c.Line, in, trimRunes(strings.ReplaceAll(c.Text, "`", "'"), 90)))
			}
			text += "Called from: " + strings.Join(cs, " · ") + " (`kos code --repo " + repo + " --func NAME` shows a function with what its callers do with the result)\n"
		}
		if room := min(r.codeLeft, out.left()-300); runeLen(text) > room {
			// Too long for what remains: its first lines rather than nothing, when enough fit.
			lines := strings.Split(text, "\n")
			for len(lines) > 14 && runeLen(strings.Join(lines, "\n"))+80 > room {
				lines = lines[:len(lines)-1]
			}
			if len(lines) <= 14 {
				return settings
			}
			text = strings.Join(lines, "\n") + fmt.Sprintf("\n    │ … cut for the budget: `kos code --repo %s --func %s`\n```\n", repo, discover.DefinedName(f.Name))
		}
		r.codeLeft -= runeLen(text)
		fmt.Fprint(out, text)
		// The settings this code reads (MAX_RETRIES): their values per environment come next.
		for _, w := range settingName.FindAllString(f.Code, -1) {
			if !seen[w] {
				seen[w] = true
				settings = append(settings, w)
			}
		}
	}
	return settings
}

var settingName = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)

// codeNames returns, in order, the code names a text writes in backticks: identifiers with a
// capital, an underscore or a dot (FindByOrderId, MAX_RETRIES, s.useCase.Handle → Handle).
func codeNames(text string, limit int, seen map[string]bool) []string {
	out := []string{}
	// Constants written in capitals with underscores (BUSINESS_ID_IS_DUPLICATED) name code even
	// without backticks: where they are produced is often the answer.
	for _, n := range settingName.FindAllString(text, -1) {
		if len(n) >= 6 && !seen[n] && len(out) < limit {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, m := range regexp.MustCompile("`([^`\\s]+)`").FindAllStringSubmatch(text, -1) {
		n := strings.TrimSuffix(m[1], "()")
		if strings.ContainsAny(n, "/") || strings.Count(n, ".") > 2 {
			continue // a path or a resource name, not a symbol
		}
		if k := strings.LastIndex(n, "."); k >= 0 {
			n = n[k+1:]
		}
		if len(n) < 4 || seen[n] || !codeName.MatchString(n) || strings.IndexFunc(n, func(c rune) bool { return c == '_' || c >= 'A' && c <= 'Z' }) < 0 {
			continue
		}
		if strings.ToUpper(n) == n && !strings.Contains(n, "_") {
			continue // POST, JSON, PENDING: a word in capitals, not a symbol to look up
		}
		seen[n] = true
		out = append(out, n)
		if len(out) == limit {
			break
		}
	}
	return out
}

// citedRepo is the one repository all the citations point at, or "".
func citedRepo(cites []citation) string {
	repo := ""
	for _, c := range cites {
		for _, a := range discover.Anchors(c.def) {
			name := a.Repo[strings.Index(a.Repo, "/")+1:]
			if repo != "" && !strings.EqualFold(repo, name) {
				return ""
			}
			repo = name
		}
	}
	return repo
}

// compactPermalinks writes a GitHub permalink as path#L when it cites the note's repository at its
// analyzed commit, and as repo@commit:path#L otherwise: the same source in a fraction of the text.
func compactPermalinks(def, repo, commit string) string {
	def = permalinkMD.ReplaceAllStringFunc(def, func(m string) string {
		return compactOne(permalinkMD.FindStringSubmatch(m)[1], repo, commit)
	})
	return permalinkRaw.ReplaceAllStringFunc(def, func(m string) string { return compactOne(m, repo, commit) })
}

func compactOne(url, repo, commit string) string {
	g := permalinkRaw.FindStringSubmatch(url)
	if g == nil {
		return url
	}
	if commit != "" && strings.EqualFold(g[1], repo) && (strings.HasPrefix(g[2], commit) || strings.HasPrefix(commit, g[2])) {
		return g[3] + g[4]
	}
	return g[1] + "@" + short(g[2]) + ":" + g[3] + g[4]
}

func footnoteDefinitions(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "[^") {
			continue
		}
		if k := strings.Index(line, "]:"); k > 2 {
			out[line[2:k]] = strings.TrimSpace(line[k+2:])
		}
	}
	return out
}

// writeRepoState writes what makes a repository note trustworthy or not, in a few lines.
func writeRepoState(s discover.NoteState, out io.Writer) {
	line := "Repository: " + s.Repo
	if s.Commit != "" {
		line += " · analyzed at " + s.Commit
	}
	if s.Checkout != "" {
		line += " · checkout " + s.Checkout
	}
	fmt.Fprintln(out, line)
	switch {
	case s.Unknown != "":
		fmt.Fprintln(out, "Freshness: unknown — "+s.Unknown)
	case len(s.StaleCited) > 0:
		fmt.Fprintf(out, "Freshness: STALE — cited files changed up to %s (%s): %s. Claims on them are unverified until read there.\n", s.Ref, s.Head, strings.Join(firstN(s.StaleCited, 8), ", "))
	case s.ChangedFiles > 0:
		fmt.Fprintf(out, "Freshness: fresh for its citations — %d files changed up to %s (%s), none cited.\n", s.ChangedFiles, s.Ref, s.Head)
	default:
		fmt.Fprintf(out, "Freshness: current — %s is at the analyzed commit.\n", s.Ref)
	}
	if s.CheckoutHead != "" {
		ref := strings.TrimPrefix(strings.TrimPrefix(s.Ref, "refs/remotes/"), "refs/heads/")
		fmt.Fprintf(out, "Checkout: the working tree is at %s, not at %s (%s): read code with `kos code --repo %s`, which reads %s, not the files on disk.\n", s.CheckoutHead, ref, s.Head, s.Repo, ref)
	}
	for _, d := range s.Discrepancies {
		fmt.Fprintf(out, "Unsupported by the code: %s [[%s]] (the repository names %s); verify before relying on it.\n", d.Field, d.Target, strings.Join(firstN(d.Found, 4), ", "))
	}
	if len(s.Undocumented) > 0 {
		fmt.Fprintf(out, "In the code, not in the note (%d): %s\n", len(s.Undocumented), strings.Join(firstN(s.Undocumented, 5), "; "))
	}
}

// writeWiring writes what the discovery facts and the platform snapshots say about a topic.
func writeWiring(w discover.Wiring, src *discover.Sources, out io.Writer, terms []askTerm) {
	if len(w.Repos) > 0 {
		// Where each repository's code reads the name tells whether it publishes or consumes.
		roles := map[string][]string{}
		order := []string{}
		for _, r := range w.Repos {
			role, where := "names it", r.File
			if r.Via != "" {
				role = "consumes via subscription " + r.Via
			} else if r.Key != "" && src != nil {
				if rl, at := codeRole(src, r.Repo, r.Key); rl != "" {
					role, where = rl, at
				}
			}
			if strings.Contains(r.File, "/pubsub/") || strings.HasSuffix(r.File, ".tf") {
				role = "declares it (infrastructure)"
			}
			if _, ok := roles[role]; !ok {
				order = append(order, role)
			}
			label := r.Repo
			if where != "" {
				label += " (" + where
				if r.Key != "" && !strings.Contains(where, r.Key) {
					label += " " + r.Key
				}
				label += ")"
			}
			roles[role] = append(roles[role], label)
		}
		sort.SliceStable(order, func(a, b int) bool { return roleRank(order[a]) < roleRank(order[b]) })
		for _, role := range order {
			fmt.Fprintf(out, "%s: %s\n", strings.ToUpper(role[:1])+role[1:], strings.Join(firstN(roles[role], 8), "; "))
		}
	}
	if len(w.Subscriptions) > 0 {
		// Those the question's words name (an event type, a consumer) first, then production.
		subs := append([]discover.WiringSubscription{}, w.Subscriptions...)
		hit := func(s discover.WiringSubscription) int {
			f, n := fold(s.Name+" "+s.Filter+" "+strings.Join(s.ConfiguredBy, " ")), 0
			for _, t := range terms {
				if t.in(f) {
					n++
				}
			}
			return n
		}
		sort.SliceStable(subs, func(a, b int) bool {
			if ha, hb := hit(subs[a]), hit(subs[b]); ha != hb {
				return ha > hb
			}
			return strings.Contains(subs[a].Scope, "prod") && !strings.Contains(subs[b].Scope, "prod")
		})
		// When some match the question, those only; the others are counted.
		if len(subs) > 3 && hit(subs[0]) > 0 {
			k := 0
			for k < len(subs) && hit(subs[k]) > 0 {
				k++
			}
			if k < len(subs) {
				others := len(subs) - k
				subs = subs[:k]
				defer fmt.Fprintf(out, "(+%d other subscriptions on it that the question does not name: `kos read --note NAME` lists them)\n", others)
			}
		}
		parts := []string{}
		for _, s := range subs {
			p := s.Name + " @" + s.Scope
			if s.Filter != "" {
				p += " filter " + s.Filter
			}
			if s.Push != "" {
				p += " push " + s.Push
			}
			if s.DeadLetter != "" {
				p += " dead-letter " + s.DeadLetter
			} else {
				p += " no dead letter"
			}
			if len(s.ConfiguredBy) > 0 {
				p += " ← " + strings.Join(s.ConfiguredBy, ", ")
			}
			parts = append(parts, trimRunes(p, 220))
		}
		fmt.Fprintf(out, "Subscriptions on it (platform snapshots, those the question names first; retry policy and ack deadline are not captured): %s\n", strings.Join(firstN(parts, 8), "; "))
	} else if len(w.Scopes) > 0 {
		counts := []string{}
		for _, sc := range w.Scopes {
			counts = append(counts, fmt.Sprintf("%s (%d subscriptions, %s)", sc.Scope, sc.Subscriptions, sc.Captured))
		}
		fmt.Fprintf(out, "Subscriptions on it: none among those captured in %s. A subscription created in a project that was not captured does not show here; confirm with `%s`.\n", strings.Join(firstN(counts, 6), ", "), w.Confirm)
	}
	if len(w.NoSubs) > 0 {
		fmt.Fprintf(out, "Scopes captured without any subscription listed (their subscriptions are unknown, not absent): %s.\n", strings.Join(firstN(w.NoSubs, 8), ", "))
	}
	if len(w.Similar) > 0 {
		fmt.Fprintf(out, "Not to confuse with: %s (other topics with the same last segment).\n", strings.Join(firstN(w.Similar, 6), ", "))
	}
}

// codeRole reads where a repository's code uses the configuration key that holds a topic and
// classifies it: a publisher, a subscriber, or unknown.
func codeRole(src *discover.Sources, repo, key string) (string, string) {
	_, all, _ := src.Grep(repo, []string{key}, 1)
	for _, h := range all[key] {
		// Deployment overlays only assign the value; the application's code or its own
		// configuration (publishers.orders.topic-id: ${KEY}) says what it is for.
		if strings.Contains(h.Path, "kustomization/") || strings.Contains(h.Path, "helm/") || strings.HasPrefix(filepath.Base(h.Path), "env") {
			continue
		}
		context, _, _ := src.File(repo, h.Path, max(1, h.Line-4), h.Line)
		if ext := filepath.Ext(h.Path); ext == ".yml" || ext == ".yaml" || ext == ".json" {
			// A value's meaning is in its parent keys: publishers.orders.topic-id.
			whole, _, _ := src.File(repo, h.Path, 1, h.Line)
			context += " " + parentKeys(whole)
		}
		t := strings.ToLower(h.Path + " " + context)
		switch {
		case strings.Contains(t, "subscri") || strings.Contains(t, "receive") || strings.Contains(t, "listen") || strings.Contains(t, "consum"):
			return "consumes it", fmt.Sprintf("%s:%d", h.Path, h.Line)
		case strings.Contains(t, "publish") || strings.Contains(t, "produc") || strings.Contains(t, "sender") || strings.Contains(t, "notif"):
			return "publishes to it", fmt.Sprintf("%s:%d", h.Path, h.Line)
		}
	}
	return "", ""
}

// parentKeys walks up numbered lines (the last one is the value) and returns the keys that
// enclose it by indentation.
func parentKeys(numbered string) string {
	lines := strings.Split(numbered, "\n")
	body := func(l string) string {
		if k := strings.Index(l, "│ "); k >= 0 {
			return l[k+len("│ "):]
		}
		return l
	}
	indent := func(l string) int { return len(l) - len(strings.TrimLeft(l, " \t")) }
	last := body(lines[len(lines)-1])
	level, keys := indent(last), []string{}
	for k := len(lines) - 2; k >= 0 && level > 0; k-- {
		l := body(lines[k])
		if strings.TrimSpace(l) == "" {
			continue
		}
		if indent(l) < level {
			level = indent(l)
			keys = append(keys, strings.TrimSpace(l))
		}
	}
	return strings.Join(keys, " ")
}

func roleRank(role string) int {
	switch {
	case strings.HasPrefix(role, "publishes"):
		return 0
	case strings.HasPrefix(role, "consumes"):
		return 1
	case strings.HasPrefix(role, "declares"):
		return 3
	}
	return 2
}

// noteSections lists the note's second-level headings with their lines.
func noteSections(raw []byte) []string {
	out := []string{}
	fence := false
	for n, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "```") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(line, "## ") {
			out = append(out, fmt.Sprintf("%s L%d", strings.TrimSpace(line[3:]), n+1))
		}
	}
	return out
}

func short(sha string) string { return sha[:min(12, len(sha))] }

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

func firstN(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(append([]string{}, xs[:n]...), fmt.Sprintf("+%d more", len(xs)-n))
}

func trimRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
