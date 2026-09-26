package retrieval

import (
	"bytes"
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

// Ask assembles, in one call, what an agent needs to answer a question from the vault: where every
// word of the question occurs across the notes, the notes the question names or matches with their
// relevant passages, each passage's sources with their state on the reference branch and the cited
// lines themselves, each repository note's checkout and freshness, the neighbouring notes, the cases
// that mention them and the sources beyond the vault. The output stays within a character budget so a
// harness never truncates it.

const (
	askParagraphsNote  = 4
	askParagraphsNamed = 10
	askParagraphRunes  = 1600
	askCodeNames       = 6
	askCodePerNote     = 3
	askNeighbours      = 10
	askCases           = 5
	askHitNotes        = 6
	askHitLines        = 12
	askHitRunes        = 3000
	askHitLineRunes    = 420
	// DefaultBudget keeps a pack under the ~30 000 characters a harness shows before truncating.
	DefaultBudget = 24000
)

// AskOptions bound a pack.
type AskOptions struct {
	Notes      int
	Visibility string
	Budget     int
	Code       bool
	Focus      string // only this note
	Brief      bool   // paragraphs with their source marks only
}

type askNote struct {
	path, stem string
	query      string
	named      bool
	minor      bool // another note, when the question names a repository
	score      float64
}

type knowledgeNote struct {
	path, stem string
	raw        []byte
	aliases    []string
}

// Ask writes the evidence pack for query as Markdown.
func (i *Index) Ask(ctx context.Context, query string, o AskOptions, w io.Writer) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("--query is required")
	}
	if o.Notes < 1 || o.Notes > 8 {
		return fmt.Errorf("--notes must be between 1 and 8")
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
	byPath := map[string]*knowledgeNote{}
	for _, n := range notes {
		byPath[n.path] = n
	}
	// 1. The notes the question names by basename or alias, and the words the question brings.
	named := namedNotes(notes, query)
	if o.Focus != "" {
		f, err := findNote(notes, o.Focus)
		if err != nil {
			return err
		}
		named, o.Notes = []string{f.path}, 1
	}
	all := askTerms(query)
	if err := i.weigh(ctx, all); err != nil {
		return err
	}
	skip := map[string]bool{}
	for _, p := range named {
		n := byPath[p]
		for _, name := range append([]string{n.stem}, n.aliases...) {
			f := fold(name)
			skip[f] = true
			for _, w := range words.FindAllString(f, -1) {
				skip[w] = true
			}
		}
	}
	terms := []askTerm{}
	for _, t := range all {
		if !skip[t.stem] && !skip[t.word] {
			terms = append(terms, t)
		}
	}
	// 2. Rank notes: named, then one hop from a named note, then by BM25 over the other words.
	ranked := map[string]*askNote{}
	order := []*askNote{}
	get := func(path string) *askNote {
		if n, ok := ranked[path]; ok {
			return n
		}
		n := &askNote{path: path, stem: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), query: query}
		ranked[path] = n
		order = append(order, n)
		return n
	}
	for _, p := range named {
		get(p).named = true
	}
	casePaths := []string{}
	caseTerms := map[string]bool{}
	if len(terms) > 0 {
		counts := map[string]int{}
		rows, err := i.db.QueryContext(ctx, `SELECT path,body,bm25(passages,0,5,4,3,1,0,0,0,0) FROM passages
  WHERE passages MATCH ? AND (?='all' OR visibility='public') ORDER BY bm25(passages,0,5,4,3,1,0,0,0,0),path,rowid LIMIT 300`, matchExpr(terms), o.Visibility)
		if err != nil {
			return err
		}
		for rows.Next() {
			var path, body string
			var rank float64
			if err = rows.Scan(&path, &body, &rank); err != nil {
				rows.Close()
				return err
			}
			if strings.HasPrefix(strings.TrimSpace(body), "[^") {
				continue // the footnote list is cited through the passages that use it
			}
			if isCase(path) {
				if counts[path] == 0 {
					casePaths = append(casePaths, path)
				}
				counts[path]++
				f := fold(body)
				for _, t := range terms {
					if t.in(f) {
						caseTerms[path+"\x00"+t.stem] = true
					}
				}
				continue
			}
			if byPath[path] == nil || counts[path] == askParagraphsNamed {
				continue
			}
			counts[path]++
			get(path).score += rank
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	// One hop from a named note: a typed relation to it (a repository that publishes to or consumes
	// the named topic) belongs in the pack whatever its words; a plain link only when it matches.
	near, role := map[string]bool{}, map[string]bool{}
	stemPath := map[string]string{}
	for _, n := range notes {
		stemPath[n.stem] = n.path
	}
	for _, p := range named {
		if nb, e := i.Neighbors(ctx, byPath[p].stem); e == nil {
			for _, x := range nb.Outgoing {
				near[x.Target] = true
			}
			for _, x := range nb.Incoming {
				near[x.Source] = true
				if x.Field != "body" && x.Field != "sistema" && stemPath[x.Source] != "" {
					role[x.Source] = true
					get(stemPath[x.Source])
				}
			}
		}
	}
	tier := func(n *askNote) int {
		switch {
		case n.named:
			return 0
		case role[n.stem] || near[n.stem] && n.score < 0:
			return 1
		}
		return 2
	}
	if len(role) > 0 && o.Focus == "" {
		o.Notes = max(o.Notes, min(len(named)+len(role), 6))
	}
	sort.SliceStable(order, func(a, b int) bool {
		if ta, tb := tier(order[a]), tier(order[b]); ta != tb {
			return ta < tb
		}
		return order[a].score < order[b].score
	})
	namesRepo := false
	for _, p := range named {
		namesRepo = namesRepo || strings.HasPrefix(p, "20-Repos/")
	}
	for _, n := range order {
		n.minor = namesRepo && !n.named && !role[n.stem]
	}
	top := order
	if o.Focus != "" {
		top = []*askNote{ranked[named[0]]}
	}
	if len(top) > o.Notes {
		top = top[:o.Notes]
	}
	caseScore := map[string]int{}
	for k := range caseTerms {
		caseScore[k[:strings.Index(k, "\x00")]]++
	}
	// A case counts when it shares half of the question's words (at least two).
	need := max(2, (len(terms)+1)/2)
	if len(terms) < 2 {
		need = 1
	}
	kept := []string{}
	for _, p := range casePaths {
		if caseScore[p] >= need {
			kept = append(kept, p)
		}
	}
	sort.SliceStable(kept, func(a, b int) bool { return caseScore[kept[a]] > caseScore[kept[b]] })
	casePaths = kept
	// 3. The tail is written first so the passages get what remains of the budget.
	var tail bytes.Buffer
	shown := map[string]bool{}
	for _, n := range top {
		shown[n.stem] = true
	}
	i.writeTail(ctx, &tail, top, shown, casePaths, byPath)
	out := &packWriter{w: w, budget: o.Budget - runeLen(tail.String())}
	fmt.Fprintf(out, "# Evidence pack\n\nQuestion: %s\nVault: %s\n", strings.TrimSpace(query), i.Root)
	writeHitMap(out, all, notes, shown)
	// A subscription the question names: what it reads, straight from the platform snapshots.
	for _, t := range all {
		if len(t.phrase) == 0 && !strings.Contains(t.word, "-") && !strings.Contains(t.word, ".") {
			continue
		}
		name := strings.Join(strings.Fields(t.word), "")
		for _, w := range compound.FindAllString(query, -1) {
			if fold(w) == t.word {
				name = w
			}
		}
		for _, sub := range discover.Subscription(i.Root, name) {
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
			fmt.Fprintln(out, trimRunes(line, 400))
		}
	}
	if len(top) == 0 {
		fmt.Fprintln(out, "\nNo note names or matches the question. Try an identifier from the code, or follow the sources beyond the vault below.")
	}
	render := &sourceRenderer{src: discover.NewSources(i.Root), code: o.Code, codeLeft: o.Budget * 3 / 10, perText: 6, brief: o.Brief, covered: map[string]bool{}}
	for k, n := range top {
		if out.left() < 900 {
			rest := []string{}
			for _, m := range top[k:] {
				rest = append(rest, m.path)
			}
			fmt.Fprintf(out, "\nAlso matching, not shown for the budget: %s (`kos read --note NAME`).\n", strings.Join(rest, "; "))
			break
		}
		if err := i.writeAskNote(ctx, out, n, byPath[n.path], terms, render, len(top)-k); err != nil {
			return err
		}
	}
	// Terms no shown paragraph holds: where the pack is silent, with the lines to read.
	gaps := []string{}
	for _, t := range terms {
		if !render.covered[t.stem] {
			where := []string{}
			for _, n := range top {
				if ls := termLines(byPath[n.path].raw, t); len(ls) > 0 {
					nums := []string{}
					for _, l := range firstN(intsToStrings(ls), 6) {
						nums = append(nums, l)
					}
					where = append(where, n.stem+" L"+strings.Join(nums, ","))
				}
			}
			if len(where) > 0 {
				gaps = append(gaps, "`"+t.display()+"` ("+strings.Join(where, "; ")+")")
			} else {
				gaps = append(gaps, "`"+t.display()+"` (in none of these notes)")
			}
		}
	}
	if len(gaps) > 0 && out.left() > 120 {
		line := fmt.Sprintf("\nNot in any paragraph shown: %s. Read those lines, or treat that part of the question as not answered by the notes.\n", strings.Join(gaps, "; "))
		fmt.Fprint(out, trimRunes(line, out.left()-2))
	}
	_, err = io.Copy(w, &tail)
	return err
}

func intsToStrings(xs []int) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

func (i *Index) writeAskNote(ctx context.Context, out *packWriter, n *askNote, note *knowledgeNote, terms []askTerm, render *sourceRenderer, remaining int) error {
	raw := note.raw
	fmt.Fprintf(out, "\n## %s\n\n%s\n", n.path, overviewLine(n.stem, raw))
	if nb, err := i.Neighbors(ctx, n.stem); err == nil {
		incoming := []string{}
		for _, e := range nb.Incoming {
			if e.Field != "body" {
				incoming = append(incoming, e.Source+" ("+e.Field+")")
			} else {
				incoming = append(incoming, e.Source)
			}
		}
		if len(incoming) > 0 {
			fmt.Fprintf(out, "Linked from: %s\n", strings.Join(firstN(incoming, 10), ", "))
		}
	}
	if sections := noteSections(raw); len(sections) > 0 {
		fmt.Fprintf(out, "Sections: %s\n", strings.Join(sections, " · "))
	}
	// Where each term is in this note, common ones included: the lines to read when a paragraph
	// did not make the pack.
	where := []string{}
	for _, t := range terms {
		if ls := termLines(raw, t); len(ls) > 0 {
			nums := []string{}
			for _, l := range ls {
				nums = append(nums, fmt.Sprint(l))
			}
			where = append(where, "`"+t.display()+"` L"+strings.Join(firstN(nums, 14), ","))
		}
	}
	if len(where) > 0 {
		fmt.Fprintf(out, "Terms in this note: %s\n", strings.Join(where, " · "))
	}
	if len(discover.Anchors(string(raw))) == 0 {
		fmt.Fprintln(out, "Sources: this note cites no permalinks, so its paragraphs carry no ✓ or ⚠; paths it names are checked under `Files named`, and its claims are confirmed with `kos code`.")
	}
	repo, commit := "", ""
	if strings.HasPrefix(n.path, "20-Repos/") {
		if s, ok := discover.RepositoryNoteState(i.Root, n.path); ok {
			writeRepoState(s, out)
			repo, commit = s.Repo, s.Commit
		}
	}
	if strings.HasPrefix(n.path, "25-Topics/") {
		if w, ok := discover.TopicWiring(i.Root, n.stem); ok {
			writeWiring(w, render.src, out, terms)
		}
	}
	k := askParagraphsNote
	switch {
	case n.named:
		k = askParagraphsNamed
	case n.minor:
		k = 2 // the question names a repository: other notes add context, not the answer
	}
	paras, more := selectParagraphs(raw, terms, k)
	if len(paras) == 0 {
		// Named without a lexical match: its opening paragraphs state what it is.
		paras, _ = selectParagraphs(raw, nil, 2)
	}
	defs := footnoteDefinitions(string(raw))
	seen := map[string]bool{}
	// One note takes at most two fifths of what remains, so the next notes still get their paragraphs.
	// A note takes its share of what remains (at most two fifths), so every note of the pack shows
	// its best paragraphs; the last one takes what is left.
	room := min(max(min(out.left()*2/5, out.left()*3/(2*remaining+1)), 2500), out.left()-300)
	if remaining == 1 {
		room = out.left() - 300
	}
	// 1. The most relevant paragraphs with their sources, while they fit.
	type block struct {
		p     paragraph
		cites []citation
		more  []string
		files string
		code  map[int]string // citation index → cited lines
		size  int
	}
	byScore := append([]paragraph{}, paras...)
	sort.SliceStable(byScore, func(a, b int) bool { return byScore[a].score > byScore[b].score })
	blocks, used := []*block{}, 0
	// A repository note keeps a third of its room for the code that answers the question.
	textRoom := room
	if repo != "" && render.code {
		textRoom = room * 2 / 3
	}
	for _, p := range byScore {
		b := &block{p: p, code: map[int]string{}}
		b.cites, b.more = render.cited(p.text, defs, seen, repo, commit, render.perText, 1<<30)
		b.files = render.filesLine(p.text, repo, commit)
		b.size = runeLen(p.render()) + 20 + runeLen(b.files)
		for _, c := range b.cites {
			b.size += runeLen(c.line) + 1
		}
		if len(b.more) > 0 {
			b.size += runeLen(alsoCited(b.more))
		}
		if len(blocks) > 0 && used+b.size > textRoom {
			more = append(more, p.line)
			continue
		}
		used += b.size
		blocks = append(blocks, b)
	}
	// 2. The cited lines of the best paragraphs, a few per note, in what room remains.
	if render.code {
		shown := 0
		for _, b := range blocks {
			if shown == askCodePerNote {
				break
			}
			for k, c := range b.cites {
				code, ok := render.snippet(c.def, focusTerms(b.p.text))
				if ok && runeLen(code) <= render.codeLeft && used+runeLen(code) <= room {
					b.code[k] = code
					used += runeLen(code)
					render.codeLeft -= runeLen(code)
					shown++
					break
				}
			}
		}
	}
	// 3. In note order, under their sections; a paragraph whose sources are all in another
	// repository says which, so "who does what" reads off the pack.
	sort.SliceStable(blocks, func(a, b int) bool { return blocks[a].p.line < blocks[b].p.line })
	section := ""
	for _, b := range blocks {
		if b.p.section != section {
			section = b.p.section
			fmt.Fprintf(out, "\n### %s\n", section)
		}
		text := b.p.render()
		if other := citedRepo(b.cites); other != "" && !strings.EqualFold(other, repo) {
			text = strings.Replace(text, fmt.Sprintf("L%d", b.p.line), fmt.Sprintf("L%d (%s)", b.p.line, other), 1)
		}
		fmt.Fprintf(out, "\n%s\n", text)
		f := fold(b.p.text)
		for _, t := range terms {
			if t.in(f) {
				render.covered[t.stem] = true
			}
		}
		if render.brief {
			if len(b.cites)+len(b.more) > 0 {
				fmt.Fprintln(out, briefSources(b.cites, b.more))
			}
		} else {
			lines := []string{}
			for k, c := range b.cites {
				lines = append(lines, c.line)
				if code, ok := b.code[k]; ok {
					lines = append(lines, code)
				}
			}
			if len(b.more) > 0 {
				lines = append(lines, alsoCited(b.more))
			}
			if len(lines) > 0 {
				fmt.Fprintln(out, "Sources:\n"+strings.Join(lines, "\n"))
			}
		}
		if b.files != "" {
			fmt.Fprintln(out, b.files)
		}
	}
	// 4. The code: where the question's words are in the repository's functions, then the code names
	// of the best paragraphs, each looked up in the repository their sources cite.
	sort.SliceStable(blocks, func(a, b int) bool { return blocks[a].p.score > blocks[b].p.score })
	// The code sections count against this note's room too, not only against the code budget.
	codeBefore := render.codeLeft
	render.codeLeft = min(render.codeLeft, max(room-used, 0))
	settings := []string{}
	if repo != "" {
		// The code files the shown paragraphs cite in this repository.
		prefer := map[string]bool{}
		for _, b := range blocks {
			for _, c := range b.cites {
				for _, a := range discover.Anchors(c.def) {
					if strings.EqualFold(a.Repo[strings.Index(a.Repo, "/")+1:], repo) {
						prefer[a.Path] = true
					}
				}
			}
		}
		settings = render.writeMatchingCode(out, repo, terms, prefer)
	}
	byRepo, order, seenNames, fromQuery := map[string][]string{}, []string{}, map[string]bool{}, map[string]bool{}
	add := func(r, name string, query bool) {
		if r == "" || seenNames[name] {
			return
		}
		if _, ok := byRepo[r]; !ok {
			order = append(order, r)
		}
		if len(byRepo[r]) < askCodeNames {
			seenNames[name] = true
			fromQuery[name] = fromQuery[name] || query
			byRepo[r] = append(byRepo[r], name)
		}
	}
	for _, t := range terms {
		if t.ident && len(t.phrase) == 0 && codeName.MatchString(t.word) {
			for _, w := range words.FindAllString(n.query, -1) {
				if fold(w) == t.word {
					add(repo, w, true)
				}
			}
		}
	}
	for _, name := range settings {
		add(repo, name, true)
	}
	for _, b := range blocks {
		r := citedRepo(b.cites)
		if r == "" {
			r = repo
		}
		for _, name := range codeNames(b.p.text, askCodeNames, map[string]bool{}) {
			add(r, name, false)
		}
	}
	for _, r := range order {
		render.writeCode(out, r, byRepo[r], fromQuery)
	}
	render.codeLeft = codeBefore - (min(codeBefore, max(room-used, 0)) - render.codeLeft)
	if len(more) > 0 {
		sort.Ints(more)
		ls := []string{}
		for _, l := range more {
			ls = append(ls, fmt.Sprint(l))
		}
		match := []string{}
		for _, t := range terms {
			match = append(match, t.word)
		}
		fmt.Fprintf(out, "\nMore matching paragraphs at L%s: `kos read --note \"%s\" --match \"%s\"`.\n", strings.Join(firstN(ls, 12), ","), n.stem, strings.Join(firstN(match, 8), " "))
	}
	return nil
}

// writeTail writes the related notes, the cases, the sources beyond the vault and how to go on.
func (i *Index) writeTail(ctx context.Context, out io.Writer, top []*askNote, shown map[string]bool, casePaths []string, byPath map[string]*knowledgeNote) {
	neighbours := []string{}
	for _, n := range top {
		nb, err := i.Neighbors(ctx, n.stem)
		if err != nil {
			continue
		}
		for _, e := range nb.Outgoing {
			if !shown[e.Target] {
				shown[e.Target] = true
				neighbours = append(neighbours, e.Target)
			}
		}
		for _, e := range nb.Incoming {
			if !shown[e.Source] {
				shown[e.Source] = true
				neighbours = append(neighbours, e.Source)
			}
		}
	}
	stems := map[string]*knowledgeNote{}
	for _, n := range byPath {
		stems[n.stem] = n
	}
	existing := []string{}
	for _, s := range neighbours {
		if stems[s] != nil {
			existing = append(existing, s) // a link without a note ([[HTTP]]) relates nothing
		}
	}
	neighbours = existing
	if len(neighbours) > 0 {
		fmt.Fprintln(out, "\n## Related notes (one hop)")
		for k, s := range neighbours {
			if k == askNeighbours {
				fmt.Fprintf(out, "- … %d more (`kos links --node NAME`)\n", len(neighbours)-askNeighbours)
				break
			}
			if n, ok := stems[s]; ok {
				fmt.Fprintln(out, "- "+trimRunes(overviewLine(s, n.raw), 190))
			}
		}
	}
	lines := []string{}
	seen := map[string]bool{}
	for _, p := range casePaths {
		dir := strings.Join(strings.SplitN(p, "/", 3)[:2], "/")
		if !seen[dir] {
			seen[dir] = true
			lines = append(lines, fmt.Sprintf("- %s (%s)", dir, origin(p)))
		}
	}
	for _, n := range top {
		ptrs, _, _ := i.investigationPointers(ctx, n.stem)
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
		fmt.Fprintln(out, "\n## Investigations that mention this (case context; load read-only through manage-investigation)")
		fmt.Fprintln(out, strings.Join(lines, "\n"))
	}
	writeSources(i.Root, out)
	fmt.Fprintln(out, "\n## Reading this pack\n\n- ✓ the cited lines read the same on the reference branch; ⚠ they changed since the cited commit: read them there before relying on the claim; ? not checkable here. `L57` is the line of a paragraph in its note.\n- The paragraphs are the best matches, not the whole note: the hit map and each note's *Terms in this note* give the other lines; `kos read --note NAME --lines FROM-TO` returns them with their sources.\n- Before delivering an answer that names topics, subscriptions, events or repositories, run `kos discover claims` on the draft.")
}

// writeHitMap writes where each word of the question occurs across the knowledge notes: the recall
// a reader otherwise gets from grep, in a few lines.
func writeHitMap(out *packWriter, terms []askTerm, notes []*knowledgeNote, inPack map[string]bool) {
	if len(terms) == 0 {
		return
	}
	var b strings.Builder
	absent, common := []string{}, []string{}
	for _, t := range terms {
		if t.df == 0 {
			if t.ident || runeLen(t.word) > 4 {
				absent = append(absent, t.word)
			}
			continue
		}
		type hit struct {
			stem  string
			lines []int
		}
		hits := []hit{}
		for _, n := range notes {
			ls := termLines(n.raw, t)
			if len(ls) > 0 {
				hits = append(hits, hit{n.stem, ls})
			}
		}
		if len(hits) == 0 {
			continue
		}
		sort.SliceStable(hits, func(a, c int) bool {
			if inPack[hits[a].stem] != inPack[hits[c].stem] {
				return inPack[hits[a].stem] // the notes the pack shows first: their lines are the next read
			}
			return len(hits[a].lines) > len(hits[c].lines)
		})
		// A word in a third of the notes locates nothing; its count is enough.
		if len(hits) >= 8 && len(hits)*3 > len(notes) {
			common = append(common, fmt.Sprintf("`%s` (%d notes)", t.display(), len(hits)))
			continue
		}
		parts, size := []string{}, 0
		for k, h := range hits {
			if k == askHitNotes || size > askHitLineRunes {
				parts = append(parts, fmt.Sprintf("+%d notes", len(hits)-k))
				break
			}
			ls := []string{}
			for x, l := range h.lines {
				if x == askHitLines {
					ls = append(ls, "…")
					break
				}
				ls = append(ls, fmt.Sprint(l))
			}
			part := fmt.Sprintf("%s ×%d L%s", h.stem, len(h.lines), strings.Join(ls, ","))
			parts = append(parts, part)
			size += runeLen(part)
		}
		fmt.Fprintf(&b, "- `%s` — %s\n", t.display(), strings.Join(parts, "; "))
	}
	text := b.String()
	if runeLen(text) > askHitRunes {
		text = trimRunes(text, askHitRunes) + "\n"
	}
	if text != "" {
		fmt.Fprint(out, "\n## Where the words are (every knowledge note, line numbers)\n\n"+text)
	}
	if len(common) > 0 {
		fmt.Fprintf(out, "Too frequent to locate, used only to rank: %s; each note below lists their lines.\n", strings.Join(common, ", "))
	}
	if len(absent) > 0 {
		fmt.Fprintf(out, "No note contains: %s. What they name is absent from the vault or called differently; try a synonym or the sources beyond the vault.\n", strings.Join(absent, ", "))
	}
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

// knowledgeNotes loads every knowledge note once: the pack reads names, aliases, lines and sections
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
