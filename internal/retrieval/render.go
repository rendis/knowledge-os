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
		_, e := p.w.Write([]byte(string(keep) + "\n… [cut at the budget: read a narrower range, use --brief or a larger --budget]\n"))
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
	routed   bool // a URL setting was followed to its route already
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
	lines, files := []string{}, []citation{}
	for _, c := range cites {
		if c.mark == "✓ file" {
			files = append(files, c) // a whole unchanged file: one line for all of them below
			continue
		}
		lines = append(lines, c.line)
		if !r.code {
			continue
		}
		if code, ok := r.snippet(c.def, focusTerms(body)); ok && runeLen(code) <= r.codeLeft && runeLen(code) < out.left()-runeLen(strings.Join(lines, "\n"))-400 {
			r.codeLeft -= runeLen(code)
			lines = append(lines, code)
		}
	}
	switch {
	case len(files) == 1:
		lines = append(lines, files[0].line)
	case len(files) > 1:
		ps := []string{}
		for _, c := range files {
			p := c.def
			if as := discover.Anchors(c.def); len(as) > 0 {
				p = as[0].Path
			}
			ps = append(ps, "[^"+c.id+"] "+p)
		}
		lines = append(lines, "- ✓ file, unchanged whole files (no lines cited): "+strings.Join(ps, " · "))
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
	if len(ids) > 5 {
		return fmt.Sprintf("- %d more sources not resolved for the budget (%s …): `kos read --note NAME --lines` of this paragraph alone resolves them", len(ids), strings.Join(ids[:3], " "))
	}
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
		// A setting configuration gives a value that no code names: configured, not read by that name.
		if settingName.MatchString(n) && settingName.FindString(n) == n {
			read := false
			for _, h := range all[n] {
				if discover.CodeFile(h.Path) && !commentLine(h.Text) {
					read = true
					break
				}
			}
			if !read {
				line += "\n  ✗ no code reads `" + n + "` at " + ref + ": configuration sets it, nothing uses it by that name (a note calling it configurable describes a setting with no effect)"
			}
		}
		if v := configValues(n, all[n]); v != "" {
			line += "\n  values: " + v
			// One URL setting per pack is followed to the route that answers it.
			if !r.routed && urlValue.MatchString(v) {
				r.routed = true
				if route := routeServers(r.src, repo, n, all[n]); route != "" {
					line += "\n  " + strings.TrimSuffix(route, "\n")
				}
			}
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
		limit := 60
		if urlValue.MatchString(v) {
			limit = 160 // a URL is read to its path: that is where a call goes
		}
		parts = append(parts, fmt.Sprintf("%s in %s", trimRunes(v, limit), compactPaths(ps)))
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

var settingName = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)

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
		// The event types each publisher's code writes: which of them go to this topic is read at
		// the lines given, not assumed.
		if src != nil {
			seen := map[string]bool{}
			for _, r := range w.Repos {
				if seen[r.Repo] || r.Via != "" || strings.HasSuffix(r.File, ".tf") {
					continue
				}
				seen[r.Repo] = true
				if types := eventTypes(src, r.Repo, w.Topic); types != "" {
					fmt.Fprintf(out, "Event types in %s's code (which reach this topic is read at the lines given): %s\n", r.Repo, types)
				}
			}
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
		parts, delivery := []string{}, false
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
			if s.Delivery != "" {
				p += " (" + s.Delivery + ")"
				delivery = true
			}
			if len(s.ConfiguredBy) > 0 {
				p += " ← " + strings.Join(s.ConfiguredBy, ", ")
			}
			parts = append(parts, trimRunes(p, 300))
		}
		note := "those the question names first"
		if !delivery {
			note += "; ack deadline and redelivery are not in these snapshots: `kos discover platform` captures them again"
		}
		fmt.Fprintf(out, "Subscriptions on it (platform snapshots, %s): %s\n", note, strings.Join(firstN(parts, 8), "; "))
	} else if len(w.Scopes) > 0 {
		counts := []string{}
		for _, sc := range w.Scopes {
			counts = append(counts, fmt.Sprintf("%s (%d subscriptions, %s)", sc.Scope, sc.Subscriptions, sc.Captured))
		}
		// Every snapshot is accounted for: those that list subscriptions (none on this topic),
		// those that list none (unknown) and those whose capture failed.
		total := len(w.Scopes) + len(w.NoSubs) + len(w.Failed)
		fmt.Fprintf(out, "Subscriptions on it: none in the %d of %d captured scopes that list subscriptions: %s. A subscription created in a project that was not captured does not show here; confirm with `%s`.\n", len(w.Scopes), total, strings.Join(firstN(counts, 6), ", "), w.Confirm)
	}
	// The consumer question answered in one line: who reads it, from the snapshots and the code.
	consumers := 0
	for _, r := range w.Repos {
		if r.Via != "" {
			consumers++
		}
	}
	if len(w.Subscriptions) == 0 && consumers == 0 && len(w.Scopes) > 0 {
		fmt.Fprintf(out, "Consumers: none found — no subscription in the %d scopes that list them, and no repository's configuration names a subscription to it (a consumer in an uncaptured project, or reading the name under another key, would not show).\n", len(w.Scopes))
	}
	if len(w.PresentIn) > 0 {
		fmt.Fprintf(out, "The topic itself is listed in: %s.\n", strings.Join(firstN(w.PresentIn, 8), ", "))
	}
	if len(w.NoSubs) > 0 {
		fmt.Fprintf(out, "Scopes captured without any subscription listed (their subscriptions are unknown, not absent): %s.\n", strings.Join(firstN(w.NoSubs, 8), ", "))
	}
	if len(w.Failed) > 0 {
		fmt.Fprintf(out, "Scopes whose capture failed (nothing known): %s.\n", strings.Join(firstN(w.Failed, 8), ", "))
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

// exitLines lists where a function leaves its main path, each with the note line that names it
// when raw is a note: the paths a note leaves out are the ones its ✓ cannot vouch for.
func exitLines(f discover.Function, raw []byte, ref string, limit int) string {
	if len(f.Exits) == 0 {
		return ""
	}
	var lines, rawLines []string
	if raw != nil {
		lines = strings.Split(fold(string(raw)), "\n")
		rawLines = strings.Split(string(raw), "\n")
	}
	type row struct {
		text    string
		missing bool
	}
	rows, passed := []row{}, []string{}
	for _, e := range f.Exits {
		if e.Propagates() {
			passed = append(passed, fmt.Sprint(e.Line))
			continue
		}
		r := row{text: fmt.Sprintf("- L%d %s", e.Line, e.Kind)}
		if !strings.HasPrefix(e.Kind, "returns") && !strings.HasPrefix(e.Kind, "reaches") || e.Kind == "returns an error" {
			r.text += " `" + trimRunes(strings.ReplaceAll(e.Text, "`", "'"), 90) + "`"
		}
		if e.When != "" {
			r.text += fmt.Sprintf(" — under L%d `%s`", e.At, trimRunes(strings.ReplaceAll(e.When, "`", "'"), 80))
		}
		if lines != nil {
			// The paragraph whose source covers the exit's line describes it best; failing that,
			// a line sharing two of its words, one of them rare in the note.
			cite, span := citingParagraph(rawLines, f.Path, e.Line)
			at, shared := noteLineSharing(lines, e.Names)
			pw := pathWords(e.Names)
			words := strings.Join(pw[:min(4, len(pw))], ", ")
			// A link covering a few lines describes them; one covering a whole function says
			// less than a line that shares the path's words.
			switch {
			case cite > 0 && (spanLines(span) <= 30 || at == 0):
				r.text += fmt.Sprintf(" — note L%d cites %s", cite, span)
			case at > 0:
				r.text += fmt.Sprintf(" — only words shared with note L%d (%s)", at, strings.Join(shared, ", "))
			case words == "":
				r.text += " — ✗"
				r.missing = true
			default:
				// The message it logs is what a reader searches for; word stems only when it logs none.
				label := words
				for _, n := range e.Names {
					if strings.Contains(n, " ") {
						label = "'" + trimRunes(n, 60) + "'"
						break
					}
				}
				r.text += " — ✗ (" + label + ")"
				r.missing = true
			}
		}
		rows = append(rows, r)
	}
	if len(rows) > limit {
		// The paths the note leaves out first: they are what the reader cannot get elsewhere.
		sort.SliceStable(rows, func(a, b int) bool { return rows[a].missing && !rows[b].missing })
		rows = rows[:limit]
	}
	head := fmt.Sprintf("Exits of %s (the whole function, L%d-L%d at %s", discover.DefinedName(f.Name), f.Start, f.End, ref)
	if lines != nil {
		head += "; note Ln cites = the paragraph whose source covers the line · only words shared = a weak link, the note may not describe this path · ✗ = neither"
	}
	out := []string{head + "):"}
	for _, r := range rows {
		out = append(out, r.text)
	}
	if more := len(f.Exits) - len(passed) - len(rows); more > 0 {
		out = append(out, fmt.Sprintf("- … %d more", more))
	}
	if len(passed) > 0 {
		out = append(out, "- L"+strings.Join(firstN(passed, 10), ",L")+" pass on the error of the call before them")
	}
	// A retry after a partial write that a duplicate check turns into a silent success.
	for _, h := range f.Hazards() {
		out = append(out, "- on redelivery: "+h.String())
	}
	// A caller that branches on the result says what each return value leads to.
	name := discover.DefinedName(f.Name)
	for _, c := range f.Callers {
		if m := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\(.*\)\s*\?\s*(.+?)\s*:\s*(.+?)\s*;?$`).FindStringSubmatch(c.Text); m != nil {
			out = append(out, fmt.Sprintf("- its result at %s:%d: true → `%s`, false → `%s`", c.Path, c.Line, trimRunes(m[1], 50), trimRunes(m[2], 50)))
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// noteLineSharing is the note line (1-based) that shares the most of an exit's words, and those
// words; 0 when no line shares two of them (or its only one). Words compare by their first five
// letters, so validating meets validación and duplicate meets duplicado.
func noteLineSharing(lines []string, names []string) (int, []string) {
	words := pathWords(names)
	if len(words) == 0 {
		return 0, nil
	}
	need := min(2, len(words))
	lineWords := make([]map[string]bool, len(lines))
	df := map[string]int{}
	for k, l := range lines {
		lineWords[k] = map[string]bool{}
		for _, w := range pathWords([]string{l}) {
			lineWords[k][w] = true
			df[w]++
		}
	}
	// A word on more than a few lines (the repository's domain: order, sale) says little alone.
	rare := max(3, len(lines)/25)
	best, bestWords := 0, []string(nil)
	for k := range lines {
		shared, specific := []string{}, false
		for _, w := range words {
			if lineWords[k][w] {
				shared = append(shared, w)
				specific = specific || df[w] <= rare
			}
		}
		if len(shared) >= need && specific && len(shared) > len(bestWords) {
			best, bestWords = k+1, shared
		}
	}
	return best, bestWords
}

// pathWords splits names and messages into words (camelCase, snake_case, prose), folded and cut
// to five letters; status codes and exit, ack, nack, panic stay whole.
func pathWords(names []string) []string {
	out, seen := []string{}, map[string]bool{}
	for _, n := range names {
		n = camelBreak.ReplaceAllString(n, "$1 $2")
		for _, w := range regexp.MustCompile(`[\pL\pN]+`).FindAllString(fold(n), -1) {
			switch {
			case statusCode.MatchString(w), w == "ack" || w == "nack" || w == "exit" || w == "panic":
			case runeLen(w) < 4 || pathStop[w]:
				continue
			default:
				w = string([]rune(w)[:min(5, runeLen(w))])
				if logWord[w] {
					continue // Errorln, Warnf, Println: how it logs, not what
				}
			}
			if !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	return out
}

var logWord = map[string]bool{"error": true, "warnl": true, "warnf": true, "warni": true, "infol": true, "infof": true, "debug": true, "print": true, "fatal": true, "logge": true, "conso": true, "sprin": true}

var (
	camelBreak = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	statusCode = regexp.MustCompile(`^[45]\d\d$`)
	pathStop   = map[string]bool{}
)

func init() {
	for _, w := range strings.Fields(`error errors this that true false null undefined return const await async catch
		throw with from when then else function message string number boolean string status response request
		logger level info warn debug timestamp date new promise void self none nil data json stringify log isostring
		each foreach empty isempty length size list item items value values result results get set`) {
		pathStop[w] = true
	}
}

var (
	startupFunc = regexp.MustCompile(`(?i)^(main|init|setup\w*|bootstrap\w*|start(up)?|run|load\w*|read(config|env|properties)\w*|must\w*|new(config|app|server)\w*|configure\w*)$`)
)

// citingParagraph is the first body line of the note that cites, through a footnote, a range of
// path holding line; with the range. A footnote definition cited by no body line counts itself.
func citingParagraph(rawLines []string, path string, line int) (int, string) {
	def := regexp.MustCompile(`^\[\^([^\]]+)\]:`)
	for k, l := range rawLines {
		m := def.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		for _, a := range discover.Anchors(l) {
			if a.Path != path || a.From == 0 || line < a.From || line > max(a.To, a.From) {
				continue
			}
			span := fmt.Sprintf("L%d-L%d", a.From, max(a.To, a.From))
			ref := "[^" + m[1] + "]"
			for j, body := range rawLines {
				if j != k && strings.Contains(body, ref) && !def.MatchString(body) {
					return j + 1, span
				}
			}
			return k + 1, span
		}
	}
	return 0, ""
}

// spanLines is how many lines an "L12-L40" range covers.
func spanLines(span string) int {
	var from, to int
	if _, e := fmt.Sscanf(span, "L%d-L%d", &from, &to); e != nil {
		return 0
	}
	return to - from + 1
}

var eventTypeLiteral = regexp.MustCompile(`(?i)event_?type\w*["']?\s*:?=?\s*["']([A-Za-z][\w.\-]+)["']`)

// eventTypes lists the event type literals a repository's code writes, each at its first line.
// Those sharing the topic's words (transaction-acked: transactionAcknowledged) come first.
func eventTypes(src *discover.Sources, repo, topic string) string {
	hits, in, _, err := src.Search(repo, `[Ee]vent_?[Tt]ype[A-Za-z0-9_]*["']?[[:space:]]*:?=?[[:space:]]*["'][A-Za-z]`, "", false, false, false)
	if err != nil {
		return ""
	}
	// Every site of each literal: the same type declared by a sale and a gift card handler is two.
	sites, order := map[string][]discover.Hit{}, []string{}
	for _, h := range hits {
		if !discover.CodeFile(h.Path) {
			continue
		}
		if m := eventTypeLiteral.FindStringSubmatch(h.Text); m != nil {
			if sites[m[1]] == nil {
				order = append(order, m[1])
			}
			sites[m[1]] = append(sites[m[1]], h)
		}
	}
	rows := []string{}
	for _, lit := range order {
		h := sites[lit][0]
		at := []string{}
		for _, x := range sites[lit] {
			a := fmt.Sprintf("%s:%d", x.Path, x.Line)
			if in[x] != "" {
				a += " in " + in[x]
			}
			at = append(at, a)
		}
		row := fmt.Sprintf("`%s` (%s)", lit, strings.Join(firstN(at, 4), "; "))
		// A constant that holds the type: used by code, or only in comments (a publish commented out).
		if d := declaredConst.FindStringSubmatch(h.Text); d != nil && !strings.EqualFold(d[1], "eventType") {
			if note := constUse(src, repo, d[1], h); note != "" {
				row += " " + note
			}
		}
		rows = append(rows, row)
	}
	stems := []string{}
	for _, w := range regexp.MustCompile(`[a-z]+`).FindAllString(strings.ToLower(topic[strings.LastIndex(topic, ".")+1:]), -1) {
		if len(w) >= 3 {
			stems = append(stems, w[:3])
		}
	}
	score := func(row string) int {
		n, lit := 0, strings.ToLower(row[:strings.Index(row, " (")])
		for _, st := range stems {
			if strings.Contains(lit, st) {
				n++
			}
		}
		return n
	}
	sort.SliceStable(rows, func(a, b int) bool { return score(rows[a]) > score(rows[b]) })
	// When some share the topic's words, those; the rest likely go to other topics and are counted.
	if len(rows) > 0 && score(rows[0]) > 0 {
		k := 0
		for k < len(rows) && score(rows[k]) == score(rows[0]) {
			k++
		}
		if k < len(rows) {
			others := []string{}
			for _, r := range rows[k:] {
				others = append(others, r[:strings.Index(r, " (")])
			}
			return strings.Join(firstN(rows[:k], 8), " · ") + "; not named like this topic, likely for others: " + strings.Join(firstN(others, 8), ", ")
		}
	}
	return strings.Join(firstN(rows, 8), " · ")
}

// processEnders names the functions of a repository that end the running process.
func processEnders(src *discover.Sources, repo string) map[string]bool {
	hits, _ := src.ProcessExits(repo)
	ends := map[string]bool{}
	for _, h := range hits {
		if h.In != "" && !startupFunc.MatchString(h.In) {
			ends[h.In] = true
		}
	}
	return ends
}

// endsThrough says whether a function calls one that ends the process: "calls closePool, which ends
// the process".
func endsThrough(f discover.Function, ends map[string]bool) string {
	for e := range ends {
		if discover.DefinedName(f.Name) != e && strings.Contains(f.Code, e+"(") {
			return "calls " + e + ", which ends the process"
		}
	}
	return ""
}

var declaredConst = regexp.MustCompile(`^\s*(?:(?:export|public|private|static|final|const|var|let|val)\s+)*(?:\w+\s+)?([A-Za-z_]\w*)\s*:?=\s*["']`)

// constUse says when a constant is never used in code beyond its declaration: "— declared, used
// only in comments (L90)" or "— declared, never used".
func constUse(src *discover.Sources, repo, name string, decl discover.Hit) string {
	hits, _, _, err := src.Search(repo, regexp.QuoteMeta(name), "", false, false, true)
	if err != nil {
		return ""
	}
	commented := []string{}
	for _, h := range hits {
		if h.Path == decl.Path && h.Line == decl.Line {
			continue
		}
		t := strings.TrimSpace(h.Text)
		if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") {
			commented = append(commented, fmt.Sprintf("%s:%d", filepath.Base(h.Path), h.Line))
			continue
		}
		return "" // used by code
	}
	if len(commented) > 0 {
		return "— declared, used only in comments (" + strings.Join(firstN(commented, 3), ", ") + ")"
	}
	return "— declared, never used"
}

func commentLine(t string) bool {
	t = strings.TrimSpace(t)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*")
}

var (
	qualifier      = regexp.MustCompile(`([A-Za-z_$][\w$]*)\.$`)
	configHolder   = regexp.MustCompile(`(?i)env|conf|setting|propert|option|param|secret|vars?$`)
	numberedPrefix = regexp.MustCompile(`^\s*\d+│ ?`)
	shortLiteral   = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|` + "`[^`]*`")
)

// settingsIn returns the setting names (MAX_RETRIES) code reads, outside its string literals: a
// query's ARRAY_AGG or ORDER_BY inside a SQL string is no setting.
func settingsIn(code string) []string {
	out, seen, raw := []string{}, map[string]bool{}, false
	for _, l := range strings.Split(code, "\n") {
		l = numberedPrefix.ReplaceAllString(l, "")
		if raw {
			k := strings.Index(l, "`")
			if k < 0 {
				continue
			}
			l, raw = l[k+1:], false
		}
		l = shortLiteral.ReplaceAllString(l, `""`)
		if strings.Count(l, "`")%2 == 1 {
			l, raw = l[:strings.Index(l, "`")], true
		}
		for _, loc := range settingName.FindAllStringIndex(l, -1) {
			w := l[loc[0]:loc[1]]
			// A constant of a library (oracledb.BIND_OUT, http.StatusOK) is no setting; one read from
			// the environment or a config object (process.env.X, this.envs.X, Config.X) is.
			if q := qualifier.FindStringSubmatch(l[:loc[0]]); q != nil && !configHolder.MatchString(q[1]) {
				continue
			}
			if !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	return out
}
