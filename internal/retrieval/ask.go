package retrieval

import (
	"context"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"knowledge-os/internal/discover"
)

// Ask maps a question onto the vault: every knowledge note that holds the question's terms, the lines
// where they are, and whether the sources those lines cite still read the same on the reference
// branch. It does not choose the answer or copy the notes: the agent reads them, directly or with
// `read`. Every matching note is listed, so the most frequent match is never taken for the whole story.

const (
	askLinesPerTerm = 12
	askNoteLines    = 24 // notes described in full; the rest are named
	askCases        = 5
	// DefaultBudget keeps the output under the ~30 000 characters a harness shows before truncating.
	DefaultBudget = 24000
)

// AskOptions bound the map.
type AskOptions struct {
	Visibility string
	Budget     int
}

type knowledgeNote struct {
	path, stem string
	raw        []byte
	aliases    []string
}

// askMatch is a note that holds some of the question's terms.
type askMatch struct {
	note     *knowledgeNote
	named    bool
	lines    map[string][]int // term display → lines
	distinct int              // distinct selective terms it holds
	weight   float64          // their summed rarity
}

// Ask writes the map for query as Markdown.
func (i *Index) Ask(ctx context.Context, query string, o AskOptions, w io.Writer) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("--query is required")
	}
	if o.Visibility != "all" && o.Visibility != "public" {
		return fmt.Errorf("visibility must be all or public")
	}
	if o.Budget < 4000 || o.Budget > 200000 {
		return fmt.Errorf("--budget must be between 4000 and 200000 characters")
	}
	notes, err := i.knowledgeNotes(ctx)
	if err != nil {
		return err
	}
	if o.Visibility == "public" {
		public, err := i.publicPaths(ctx)
		if err != nil {
			return err
		}
		kept := notes[:0]
		for _, n := range notes {
			if public[n.path] {
				kept = append(kept, n)
			}
		}
		notes = kept
	}
	// The notes the question names by basename or alias; their names are not terms to locate.
	named := map[string]bool{}
	skip := map[string]bool{}
	for _, p := range namedNotes(notes, query) {
		named[p] = true
		for _, n := range notes {
			if n.path != p {
				continue
			}
			for _, name := range append([]string{n.stem}, n.aliases...) {
				f := fold(name)
				skip[f] = true
				for _, x := range words.FindAllString(f, -1) {
					skip[x] = true
				}
			}
		}
	}
	all := askTerms(query)
	if err := i.weigh(ctx, all); err != nil {
		return err
	}
	terms, common, absent := []askTerm{}, []askTerm{}, []string{}
	for _, t := range all {
		switch {
		case skip[t.stem] || skip[t.word]:
		case t.df == 0:
			if t.ident || runeLen(t.word) > 4 {
				absent = append(absent, t.word)
			}
		case t.common:
			common = append(common, t)
		default:
			terms = append(terms, t)
		}
	}
	// Frequent words select only when the question brings nothing rarer.
	selective := terms
	if len(selective) == 0 {
		selective = common
	}
	matches := []*askMatch{}
	for _, n := range notes {
		m := &askMatch{note: n, named: named[n.path], lines: map[string][]int{}}
		for _, t := range selective {
			if ls := termLines(n.raw, t); len(ls) > 0 {
				m.lines[t.display()] = ls
				m.distinct++
				m.weight += t.weight
			}
		}
		if m.named || m.distinct > 0 {
			matches = append(matches, m)
		}
	}
	sort.SliceStable(matches, func(a, b int) bool {
		x, y := matches[a], matches[b]
		if x.named != y.named {
			return x.named
		}
		if x.distinct != y.distinct {
			return x.distinct > y.distinct
		}
		if x.weight != y.weight {
			return x.weight > y.weight
		}
		return lineCount(x) > lineCount(y)
	})
	out := &packWriter{w: w, budget: o.Budget}
	fmt.Fprintf(out, "# Vault map\n\nQuestion: %s\nVault: %s\n", strings.TrimSpace(query), i.Root)
	writeNamedSubscriptions(out, i.Root, query, all)
	if len(absent) > 0 {
		fmt.Fprintf(out, "\nNo note contains: %s. What they name is absent from the notes or called differently: try a synonym, the code or the sources beyond the vault.\n", strings.Join(absent, ", "))
	}
	if len(common) > 0 && len(terms) > 0 {
		parts := []string{}
		for _, t := range common {
			parts = append(parts, fmt.Sprintf("`%s` (%d passages)", t.display(), t.df))
		}
		fmt.Fprintf(out, "Too frequent to locate anything, left out: %s.\n", strings.Join(parts, ", "))
	}
	if len(matches) == 0 {
		fmt.Fprintln(out, "\nNo note names or holds the question's terms. Try an identifier from the code, or the sources beyond the vault below.")
	} else {
		fmt.Fprintf(out, "\n## Notes holding the question's terms (%d, all of them; most terms first)\n", len(matches))
	}
	src := discover.NewSources(i.Root)
	tail := i.mapTail(ctx, query, o.Visibility, selective, matches)
	for k, m := range matches {
		entry := i.mapEntry(m, selective, src)
		if k == askNoteLines || runeLen(entry) > out.left()-runeLen(tail)-200 {
			rest := []string{}
			for _, x := range matches[k:] {
				rest = append(rest, fmt.Sprintf("%s (%d)", x.note.path, x.distinct))
			}
			fmt.Fprintf(out, "\n- %d more, fewer terms each: %s\n", len(rest), trimRunes(strings.Join(rest, "; "), max(out.left()-runeLen(tail)-200, 200)))
			break
		}
		fmt.Fprint(out, entry)
	}
	// A topic the question names: who publishes and consumes it, from the code and the platform.
	for _, m := range matches {
		if m.named && strings.HasPrefix(m.note.path, "25-Topics/") && out.left() > runeLen(tail)+1500 {
			if wr, ok := discover.TopicWiring(i.Root, m.note.stem); ok {
				fmt.Fprintf(out, "\n## Wiring of %s\n", m.note.stem)
				writeWiring(wr, src, out, selective)
			}
		}
	}
	fmt.Fprint(out, tail)
	return nil
}

func lineCount(m *askMatch) int {
	n := 0
	for _, ls := range m.lines {
		n += len(ls)
	}
	return n
}

// mapEntry describes one matching note: what it is, where the terms are and whether the sources of
// those paragraphs still hold.
func (i *Index) mapEntry(m *askMatch, terms []askTerm, src *discover.Sources) string {
	var b strings.Builder
	label := ""
	if m.named {
		label = " (named)"
	}
	fmt.Fprintf(&b, "\n- `%s`%s — %s\n", m.note.path, label, trimRunes(overviewLine(m.note.stem, m.note.raw), 220))
	where := []string{}
	for _, t := range terms {
		if ls, ok := m.lines[t.display()]; ok {
			nums := firstN(intsToStrings(ls), askLinesPerTerm)
			if len(ls) > askLinesPerTerm {
				nums = append(nums, fmt.Sprintf("… (%d)", len(ls)))
			}
			where = append(where, "`"+t.display()+"` L"+strings.Join(nums, ","))
		}
	}
	if len(where) > 0 {
		fmt.Fprintf(&b, "  Lines: %s\n", strings.Join(where, " · "))
	}
	if line := sourcesLine(m, terms, src); line != "" {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	if strings.HasPrefix(m.note.path, "20-Repos/") {
		if s, ok := discover.RepositoryNoteState(i.Root, m.note.path); ok {
			fmt.Fprintf(&b, "  %s\n", repoStateLine(s))
		}
	}
	return b.String()
}

// sourcesLine states whether the paragraphs holding the terms still stand on their sources: how many
// cite lines that read the same on the reference branch, which ones changed, which cannot be checked
// here and which cite nothing.
func sourcesLine(m *askMatch, terms []askTerm, src *discover.Sources) string {
	defs := footnoteDefinitions(string(m.note.raw))
	if len(defs) == 0 {
		return "Sources: the note cites no footnotes; its lines are claims to confirm in the code."
	}
	current, unknown, uncited := 0, 0, 0
	changed := []string{}
	ref := ""
	for _, p := range parseParagraphs(m.note.raw) {
		if len(terms) > 0 && !matchesAny(p.text, terms) {
			continue
		}
		ids := footnoteRef.FindAllStringSubmatch(p.text, -1)
		if len(ids) == 0 {
			uncited++
			continue
		}
		state := "current"
		for _, id := range ids {
			for _, a := range discover.Anchors(defs[id[1]]) {
				st := src.State(a)
				switch {
				case st.Status == "changed":
					state, ref = "changed", st.Ref
				case st.Status == "unknown" && state == "current":
					state = "unknown"
				}
			}
		}
		switch state {
		case "changed":
			changed = append(changed, fmt.Sprintf("L%d", p.line))
		case "unknown":
			unknown++
		default:
			current++
		}
	}
	parts := []string{}
	if current > 0 {
		parts = append(parts, fmt.Sprintf("%d ✓", current))
	}
	if len(changed) > 0 {
		parts = append(parts, fmt.Sprintf("⚠ %s (cited lines changed on %s: read them there)", strings.Join(firstN(changed, 8), ","), ref))
	}
	if unknown > 0 {
		parts = append(parts, fmt.Sprintf("%d ? (not checkable here)", unknown))
	}
	if uncited > 0 {
		parts = append(parts, fmt.Sprintf("%d cite nothing", uncited))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Paragraphs with these terms: " + strings.Join(parts, " · ")
}

// repoStateLine is a repository note's freshness and what the last discovery run found against it.
func repoStateLine(s discover.NoteState) string {
	parts := []string{"Repository " + s.Repo}
	switch {
	case s.Unknown != "":
		parts = append(parts, "freshness unknown: "+s.Unknown)
	case len(s.StaleCited) > 0:
		parts = append(parts, fmt.Sprintf("STALE: cited files changed up to %s (%s): %s", s.Ref, s.Head, strings.Join(firstN(s.StaleCited, 5), ", ")))
	case s.ChangedFiles > 0:
		parts = append(parts, fmt.Sprintf("fresh for its citations (%d uncited files changed up to %s)", s.ChangedFiles, s.Ref))
	default:
		parts = append(parts, "current: "+s.Ref+" is at the analyzed commit")
	}
	if s.CheckoutHead != "" {
		ref := strings.TrimPrefix(strings.TrimPrefix(s.Ref, "refs/remotes/"), "refs/heads/")
		parts = append(parts, fmt.Sprintf("the checkout is at %s, not %s: read code at %s (`kos code --repo %s` or `git show %s:PATH`)", s.CheckoutHead, ref, ref, s.Repo, ref))
	}
	if len(s.Discrepancies) > 0 {
		ds := []string{}
		for _, d := range s.Discrepancies {
			ds = append(ds, d.Field+" [["+d.Target+"]]")
		}
		parts = append(parts, "unsupported by the code: "+strings.Join(firstN(ds, 4), ", "))
	}
	if len(s.Undocumented) > 0 {
		parts = append(parts, fmt.Sprintf("%d resources in the code the note does not name", len(s.Undocumented)))
	}
	return strings.Join(parts, " · ")
}

// writeNamedSubscriptions writes what the platform snapshots say about a subscription the question names.
func writeNamedSubscriptions(out io.Writer, root, query string, terms []askTerm) {
	for _, t := range terms {
		if len(t.phrase) == 0 && !strings.Contains(t.word, "-") && !strings.Contains(t.word, ".") {
			continue
		}
		name := strings.Join(strings.Fields(t.word), "")
		for _, w := range compound.FindAllString(query, -1) {
			if fold(w) == t.word {
				name = w
			}
		}
		for _, sub := range discover.Subscription(root, name) {
			line := fmt.Sprintf("Subscription %s @%s (captured %s): reads topic %s", sub.Name, sub.Scope, sub.Captured, sub.Topic)
			if sub.Filter != "" {
				line += "; filter " + sub.Filter
			}
			if sub.DeadLetter != "" {
				line += "; dead letter " + sub.DeadLetter
			} else {
				line += "; no dead letter"
			}
			if sub.Push != "" {
				line += "; push " + sub.Push
			}
			if sub.Delivery != "" {
				line += "; " + sub.Delivery
			}
			fmt.Fprintln(out, trimRunes(line, 400))
		}
	}
}

// mapTail is the cases that share the question's terms, the sources beyond the vault and the legend.
func (i *Index) mapTail(ctx context.Context, query, visibility string, terms []askTerm, matches []*askMatch) string {
	var b strings.Builder
	lines, seen := []string{}, map[string]bool{}
	if len(terms) > 0 {
		// A case counts when it holds half of the question's terms (at least two).
		need := max(2, (len(terms)+1)/2)
		if len(terms) < 2 {
			need = 1
		}
		held := map[string]map[string]bool{}
		order := []string{}
		rows, err := i.db.QueryContext(ctx, `SELECT path,body FROM passages WHERE passages MATCH ? AND (?='all' OR visibility='public') ORDER BY path,rowid`, matchExpr(terms), visibility)
		if err == nil {
			for rows.Next() {
				var path, body string
				if rows.Scan(&path, &body) != nil || !isCase(path) {
					continue
				}
				if held[path] == nil {
					held[path] = map[string]bool{}
					order = append(order, path)
				}
				f := fold(body)
				for _, t := range terms {
					if t.in(f) {
						held[path][t.stem] = true
					}
				}
			}
			rows.Close()
		}
		sort.SliceStable(order, func(a, c int) bool { return len(held[order[a]]) > len(held[order[c]]) })
		for _, p := range order {
			dir := strings.Join(strings.SplitN(p, "/", 3)[:2], "/")
			if len(held[p]) >= need && !seen[dir] {
				seen[dir] = true
				lines = append(lines, fmt.Sprintf("- %s (%s)", dir, origin(p)))
			}
		}
	}
	for _, m := range matches {
		if !m.named {
			continue
		}
		ptrs, _, _ := i.investigationPointers(ctx, m.note.stem)
		for _, p := range ptrs {
			dir := filepath.ToSlash(filepath.Dir(p.Path))
			if !seen[dir] {
				seen[dir] = true
				lines = append(lines, fmt.Sprintf("- %s — %s (%s)", dir, trimRunes(p.Title, 90), p.Origin))
			}
		}
	}
	if len(lines) > askCases {
		lines = append(lines[:askCases], fmt.Sprintf("- … %d more", len(lines)-askCases))
	}
	if len(lines) > 0 {
		fmt.Fprintln(&b, "\n## Investigations that mention this (case context; load read-only through manage-investigation)")
		fmt.Fprintln(&b, strings.Join(lines, "\n"))
	}
	writeSources(i.Root, &b)
	fmt.Fprintln(&b, "\n## Reading this map\n\n- It lists every note holding the terms; it does not say which one answers. Read the lines it gives, in the file or with `kos read --note NAME --lines FROM-TO` (which checks each footnote those lines cite), and follow each note whose lines bear on the question: two notes can describe two paths.\n- ✓ the cited lines read the same on the reference branch; ⚠ they changed since the cited commit: read them there before relying on the claim; ? not checkable here. A paragraph that cites nothing is a claim to confirm in the code.\n- Before delivering an answer that names topics, subscriptions, events or repositories, run `kos discover claims --file -` on the draft.")
	return b.String()
}

// publicPaths is the set of notes whose passages are public.
func (i *Index) publicPaths(ctx context.Context) (map[string]bool, error) {
	rows, err := i.db.QueryContext(ctx, `SELECT DISTINCT path FROM passages WHERE visibility='public'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out[p] = true
	}
	return out, rows.Err()
}

func intsToStrings(xs []int) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

// termLines returns the body lines of a note that contain the term, footnote definitions excluded.
func termLines(raw []byte, t askTerm) []int {
	out := []int{}
	lines := strings.Split(string(raw), "\n")
	start := 0
	if len(lines) > 0 && lines[0] == "---" {
		for n := 1; n < len(lines); n++ {
			if lines[n] == "---" {
				start = n + 1
				break
			}
		}
	}
	for n := start; n < len(lines); n++ {
		l := lines[n]
		if strings.HasPrefix(l, "[^") || strings.TrimSpace(l) == "" {
			continue
		}
		if t.in(fold(l)) {
			out = append(out, n+1)
		}
	}
	return out
}

// askTerm is one word, identifier or quoted phrase of the question. Words match by a prefix that
// covers their inflections (deduplica → deduplicación), since the index has no stemmer; identifiers
// and phrases match exactly.
type askTerm struct {
	word   string   // as folded from the question
	stem   string   // what a folded line must contain
	phrase []string // the index tokens of an identifier or quoted phrase
	ident  bool
	weight float64 // inverse passage frequency: a word every passage has decides nothing
	df     int     // passages that contain it, over the whole index
	common bool    // in more than a sixth of the passages: it orders, it does not select
}

// in reports whether a folded text holds the term: a short word only as a whole word (ack, not
// stack), a longer word by its stem at the start of a word, an identifier or phrase anywhere.
func (t askTerm) in(folded string) bool {
	if runeLen(t.stem) > 3 {
		if t.ident || len(t.phrase) > 0 {
			return strings.Contains(folded, t.stem)
		}
		// A word's stem starts a word: firma matches firmado, never confirmar.
		for from := 0; ; {
			k := strings.Index(folded[from:], t.stem)
			if k < 0 {
				return false
			}
			start := from + k
			if start == 0 || !isWordByte(folded[start-1]) {
				return true
			}
			from = start + 1
		}
	}
	for from := 0; ; {
		k := strings.Index(folded[from:], t.stem)
		if k < 0 {
			return false
		}
		start, end := from+k, from+k+len(t.stem)
		if (start == 0 || !isWordByte(folded[start-1])) && (end == len(folded) || !isWordByte(folded[end])) {
			return true
		}
		from = start + 1
	}
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_' || b >= 0x80
}

func (t askTerm) expr() string {
	switch {
	case len(t.phrase) > 0:
		return `"` + strings.Join(t.phrase, " ") + `"`
	case t.stem != t.word:
		return `"` + t.stem + `"*`
	}
	return `"` + t.word + `"`
}

func (t askTerm) display() string {
	if t.stem != t.word && len(t.phrase) == 0 {
		return t.stem + "*"
	}
	return t.stem
}

func matchExpr(terms []askTerm) string {
	exprs := []string{}
	for _, t := range terms {
		exprs = append(exprs, t.expr())
	}
	return strings.Join(exprs, " OR ")
}

var (
	quoted   = regexp.MustCompile(`"([^"]+)"`)
	compound = regexp.MustCompile(`[\p{L}\p{N}_]+(?:[-./:][\p{L}\p{N}_]+)*`)
)

// askTerms keeps what can discriminate: quoted phrases, identifiers (with - . / : _, digits or inner
// capitals, or all capitals) whole, other words of more than three letters by their stem.
func askTerms(query string) []askTerm {
	out := []askTerm{}
	seen := map[string]bool{}
	add := func(t askTerm) {
		if !seen[t.stem] {
			seen[t.stem] = true
			out = append(out, t)
		}
	}
	for _, m := range quoted.FindAllStringSubmatch(query, 16) {
		f := strings.Join(strings.Fields(fold(m[1])), " ")
		if toks := words.FindAllString(f, -1); len(toks) > 0 {
			add(askTerm{word: f, stem: f, phrase: toks, ident: true})
		}
	}
	rest := quoted.ReplaceAllString(query, " ")
	for _, w := range compound.FindAllString(rest, 64) {
		f := fold(w)
		toks := words.FindAllString(f, -1)
		if len(toks) > 1 {
			add(askTerm{word: f, stem: f, phrase: toks, ident: true})
			continue
		}
		ident := identifier(w)
		if runeLen(f) <= 2 && !ident {
			continue
		}
		t := askTerm{word: f, stem: f, ident: ident}
		if r := []rune(f); len(r) >= 6 && !ident {
			t.stem = string(r[:max(5, len(r)-3)])
		}
		add(t)
	}
	// Inflections of one word (reintento, reintentos) are one term: the shorter stem covers both.
	kept := []askTerm{}
	for x, t := range out {
		covered := false
		for y, u := range out {
			if x != y && len(t.phrase) == 0 && len(u.phrase) == 0 && !t.ident && !u.ident && u.stem != t.stem && strings.HasPrefix(t.stem, u.stem) {
				covered = true
			}
		}
		if !covered {
			kept = append(kept, t)
		}
	}
	return kept
}

// identifier: digits or an underscore, a capital after the first letter, or two or more capitals.
func identifier(w string) bool {
	upper := 0
	for k, c := range []rune(w) {
		switch {
		case unicode.IsDigit(c) || c == '_':
			return true
		case unicode.IsUpper(c):
			upper++
			if k > 0 {
				return true
			}
		}
	}
	return upper > 1
}

// weigh sets each term's passage frequency and inverse frequency over the whole index.
func (i *Index) weigh(ctx context.Context, terms []askTerm) error {
	var total float64
	if err := i.db.QueryRowContext(ctx, `SELECT count(*) FROM passages`).Scan(&total); err != nil {
		return err
	}
	for k := range terms {
		if err := i.db.QueryRowContext(ctx, `SELECT count(*) FROM passages WHERE passages MATCH ?`, terms[k].expr()).Scan(&terms[k].df); err != nil {
			return err
		}
		terms[k].weight = math.Log((total + 1) / (float64(terms[k].df) + 1))
		terms[k].common = float64(terms[k].df)*6 > total
	}
	return nil
}

// knowledgeNotes loads every knowledge note once: the map reads names, aliases, lines and sections
// from them.
func (i *Index) knowledgeNotes(ctx context.Context) ([]*knowledgeNote, error) {
	rows, err := i.db.QueryContext(ctx, `SELECT path FROM files ORDER BY path`)
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return nil, err
		}
		if !isCase(p) && !strings.HasPrefix(p, "90-Meta/") && strings.Contains(p, "/") {
			paths = append(paths, p)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []*knowledgeNote{}
	for _, p := range paths {
		raw, e := readFileBounded(filepath.Join(i.Root, p), maxFileBytes)
		if e != nil {
			continue
		}
		out = append(out, &knowledgeNote{path: p, stem: strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)), raw: raw, aliases: noteAliases(raw)})
	}
	return out, nil
}

// namedNotes returns the notes whose basename or alias the question writes whole, longest first.
func namedNotes(notes []*knowledgeNote, query string) []string {
	q := fold(query)
	type hit struct {
		path string
		size int
	}
	hits := []hit{}
	for _, n := range notes {
		best := 0
		for _, name := range append([]string{n.stem}, n.aliases...) {
			name = fold(strings.TrimSpace(name))
			if runeLen(name) >= 3 && containsName(q, name) && len(name) > best {
				best = len(name)
			}
		}
		if best > 0 {
			hits = append(hits, hit{n.path, best})
		}
	}
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].size != hits[b].size {
			return hits[a].size > hits[b].size
		}
		return hits[a].path < hits[b].path
	})
	out := []string{}
	for _, h := range hits {
		out = append(out, h.path)
	}
	return out
}

// noteAliases reads the frontmatter's aliases, a string or a list.
func noteAliases(raw []byte) []string {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil
	}
	var meta struct {
		Aliases any `yaml:"aliases"`
	}
	if yaml.Unmarshal([]byte(text[4:4+end]), &meta) != nil {
		return nil
	}
	switch a := meta.Aliases.(type) {
	case string:
		return []string{a}
	case []any:
		out := []string{}
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// containsName reports whether name occurs in text as a whole name, not inside a longer word.
func containsName(text, name string) bool {
	word := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
	}
	for from := 0; ; {
		k := strings.Index(text[from:], name)
		if k < 0 {
			return false
		}
		start, end := from+k, from+k+len(name)
		before, after := ' ', ' '
		if start > 0 {
			r := []rune(text[:start])
			before = r[len(r)-1]
		}
		if end < len(text) {
			after = []rune(text[end:])[0]
		}
		// A sentence's closing full stop is not part of the name.
		if after == '.' && (end+1 == len(text) || text[end+1] == ' ') {
			after = ' '
		}
		if !word(before) && !word(after) {
			return true
		}
		from = start + 1
	}
}

// fold lowercases and removes the accents the index also ignores.
func fold(s string) string {
	return accents.Replace(strings.ToLower(s))
}

var accents = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n", "à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u", "â", "a", "ê", "e", "î", "i", "ô", "o", "û", "u", "ç", "c", "ã", "a", "õ", "o")

func parseNotePassages(path string, raw []byte) []passage {
	d, err := parseDocument(path, raw)
	if err != nil {
		return nil
	}
	return d.Passages
}

func isCase(path string) bool {
	return strings.HasPrefix(path, "investigations/") || strings.HasPrefix(path, ".investigations/") || strings.HasPrefix(path, ".investigations-private/")
}

func origin(path string) string {
	switch {
	case strings.HasPrefix(path, "investigations/"):
		return "published-investigation"
	case strings.HasPrefix(path, ".investigations-private/"):
		return "private-investigation"
	}
	return "local-investigation"
}
