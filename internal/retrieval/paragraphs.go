package retrieval

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"knowledge-os/internal/discover"
)

// A note's unit of knowledge is a paragraph: a bullet, a numbered step, a table, a block of prose.
// The pack selects paragraphs, not index chunks, so each shows whole, with its line, once.

type paragraph struct {
	line    int // first line, 1-based
	section string
	text    string
	table   bool
	score   float64
	similar []int    // lines of near-identical paragraphs folded into this one
	differ  []string // for each folded paragraph, the words it has that this one lacks
	via     int      // for a paragraph with none of the terms: the matching paragraph whose code it cites
}

const paragraphsPerSection = 4

var listItem = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s`)

// render writes the paragraph with its line, so a claim can be cited and read again.
// render writes the paragraph whole up to a generous limit; a longer one is cut at a sentence with
// the command that reads it whole, so a pack never ends a claim mid-sentence without saying so.
func (p paragraph) render() string { return p.renderWithin(askParagraphRunes) }

// end is the paragraph's last line.
func (p paragraph) end() int { return p.line + strings.Count(p.text, "\n") }

// renderWithin writes the paragraph cut at limit characters (0: whole), and for each folded alike
// paragraph its line and what differs in it (L46 [production/acco]), so an environment or country
// that behaves differently is never hidden by the fold.
func (p paragraph) renderWithin(limit int) string {
	text := p.text
	if limit > 0 && runeLen(text) > limit {
		cut := string([]rune(text)[:limit])
		if k := strings.LastIndexAny(cut, ".;"); k > limit/2 {
			cut = cut[:k+1]
		}
		text = cut + fmt.Sprintf(" … (cut: `kos read --note NAME --lines %d-%d` has it whole)", p.line, p.end())
	}
	extra := ""
	if len(p.similar) > 0 {
		ls := []string{}
		for k, l := range p.similar {
			d := ""
			if k < len(p.differ) && p.differ[k] != "" {
				d = " [" + p.differ[k] + "]"
			}
			ls = append(ls, fmt.Sprintf("L%d%s", l, d))
		}
		extra = fmt.Sprintf(" (+%d alike: %s)", len(p.similar), strings.Join(firstN(ls, 14), ", "))
	}
	head := fmt.Sprintf("L%d", p.line)
	if p.via > 0 {
		head += fmt.Sprintf(" (cites the code of L%d)", p.via)
	}
	if p.table {
		return fmt.Sprintf("%s%s\n%s", head, extra, text)
	}
	return fmt.Sprintf("%s %s%s", head, text, extra)
}

// parseParagraphs splits a note body into paragraphs with their section path.
func parseParagraphs(raw []byte) []paragraph {
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	start := 0
	if len(lines) > 0 && lines[0] == "---" {
		for n := 1; n < len(lines); n++ {
			if lines[n] == "---" {
				start = n + 1
				break
			}
		}
	}
	out := []paragraph{}
	var heads [6]string
	section := ""
	var cur *paragraph
	flush := func() {
		if cur != nil && strings.TrimSpace(cur.text) != "" {
			cur.text = strings.TrimRight(cur.text, "\n")
			out = append(out, *cur)
		}
		cur = nil
	}
	fence := false
	for n := start; n < len(lines); n++ {
		l := lines[n]
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "```") {
			if !fence {
				flush()
				cur = &paragraph{line: n + 1, section: section}
			}
			fence = !fence
			cur.text += l + "\n"
			if !fence {
				flush()
			}
			continue
		}
		if fence {
			cur.text += l + "\n"
			continue
		}
		if strings.Contains(l, "<!--") {
			// Machine markers (<!-- connection:… -->) are not knowledge.
			l = strings.TrimRight(ovComment.ReplaceAllString(l, ""), " ")
			if strings.TrimSpace(l) == "" {
				flush() // a marker line starts a new block
				continue
			}
			trimmed = strings.TrimSpace(l)
		}
		switch {
		case trimmed == "" || strings.HasPrefix(l, "[^"):
			flush()
			continue
		case strings.HasPrefix(l, "#"):
			flush()
			level := len(l) - len(strings.TrimLeft(l, "#"))
			if level >= 1 && level <= 6 {
				heads[level-1] = strings.TrimSpace(l[level:])
				for k := level; k < 6; k++ {
					heads[k] = ""
				}
				parts := []string{}
				for _, h := range heads[1:] {
					if h != "" {
						parts = append(parts, h)
					}
				}
				section = strings.Join(parts, " > ")
				if section == "" {
					section = heads[0]
				}
			}
			continue
		case strings.HasPrefix(trimmed, "|"):
			if cur == nil || !cur.table {
				flush()
				cur = &paragraph{line: n + 1, section: section, table: true}
			}
		case listItem.MatchString(l):
			flush()
			cur = &paragraph{line: n + 1, section: section}
		case cur == nil || cur.table:
			flush()
			cur = &paragraph{line: n + 1, section: section}
		}
		cur.text += l + "\n"
	}
	flush()
	return out
}

// selectParagraphs returns up to k paragraphs that cover the most weighted terms, at most a few per
// section, near-identical ones folded together, in note order; and the lines of the other matches.
// Without terms it returns the note's first k paragraphs.
func selectParagraphs(raw []byte, terms []askTerm, k int) ([]paragraph, []int) {
	all := parseParagraphs(raw)
	if len(terms) == 0 {
		if len(all) > k {
			all = all[:k]
		}
		return all, nil
	}
	// Covering more of the question's terms comes first; a term's rarity only orders equal coverage,
	// so a word the question asks for is never drowned by being frequent in the vault.
	top := 0.0
	for _, t := range terms {
		top = max(top, t.weight)
	}
	// Terms whose stems contain one another (dedupl, duplic) name one thing: counted once.
	group := make([]int, len(terms))
	for a := range terms {
		group[a] = a
		for b := 0; b < a; b++ {
			if strings.Contains(terms[a].stem, terms[b].stem) || strings.Contains(terms[b].stem, terms[a].stem) {
				group[a] = group[b]
				break
			}
		}
	}
	cands := []paragraph{}
	for _, p := range all {
		f := fold(p.text)
		best := map[int]float64{}
		for x, t := range terms {
			if t.in(f) {
				// A word a sixth of the vault holds (sale, venta) says little about this paragraph.
				v := 1 + t.weight/max(top, 1)
				if t.common {
					v = 0.3
				}
				best[group[x]] = max(best[group[x]], v)
			}
		}
		for _, v := range best {
			p.score += v
		}
		if p.score > 0 {
			if p.table {
				p.text = matchingRows(p.text, terms)
			}
			cands = append(cands, p)
		}
	}
	sort.SliceStable(cands, func(a, b int) bool { return cands[a].score > cands[b].score })
	cands = append(cands, sharingSources(raw, all, cands)...)
	sort.SliceStable(cands, func(a, b int) bool { return cands[a].score > cands[b].score })
	// A section gives at most its share of k, never less than a few: a note answering in one section
	// fills k from it, one answering in many spreads.
	sections := map[string]bool{}
	for _, c := range cands {
		sections[c.section] = true
	}
	cap := max(paragraphsPerSection, (k+len(sections)-1)/max(len(sections), 1))
	chosen, more := []paragraph{}, []int{}
	perSection := map[string]int{}
	sigs := [][]string{}
	for _, c := range cands {
		sig := signature(c.text)
		folded := false
		for x := range chosen {
			if jaccard(sig, sigs[x]) >= 0.6 {
				chosen[x].similar = append(chosen[x].similar, c.line)
				chosen[x].differ = append(chosen[x].differ, difference(c.text, chosen[x].text))
				folded = true
				break
			}
		}
		if folded {
			continue
		}
		if len(chosen) == k || perSection[c.section] == cap {
			more = append(more, c.line)
			continue
		}
		perSection[c.section]++
		chosen = append(chosen, c)
		sigs = append(sigs, sig)
	}
	sort.SliceStable(chosen, func(a, b int) bool { return chosen[a].line < chosen[b].line })
	for x := range chosen {
		sort.Ints(chosen[x].similar)
	}
	return chosen, more
}

// sharingSources returns the paragraphs that hold none of the terms but cite a code file several
// matches cite: the same mechanism told in other words ("consulta antes de notificar" next to
// "deduplicación"). Configuration and files a quarter of the paragraphs cite relate nothing.
func sharingSources(raw []byte, all, cands []paragraph) []paragraph {
	defs := footnoteDefinitions(string(raw))
	files := func(text string) map[string]bool {
		out := map[string]bool{}
		for _, m := range footnoteRef.FindAllStringSubmatch(text, -1) {
			for _, a := range discover.Anchors(defs[m[1]]) {
				if discover.CodeFile(a.Path) {
					out[a.Path] = true
				}
			}
		}
		return out
	}
	citing := map[string]int{}
	for _, p := range all {
		for f := range files(p.text) {
			citing[f]++
		}
	}
	// A code file cited mostly by matching paragraphs (two or more) is where the mechanism lives; a
	// paragraph citing it tells more of the same mechanism.
	hub, hubLine := map[string]float64{}, map[string]paragraph{}
	strong := []paragraph{} // matches through at least one word that is not common in the vault
	for _, c := range cands {
		if c.score >= 1 {
			strong = append(strong, c)
		}
	}
	cands = strong
	for _, c := range cands {
		for f := range files(c.text) {
			if citing[f]*4 <= len(all) {
				hub[f] += c.score
				if b, ok := hubLine[f]; !ok || c.score > b.score {
					hubLine[f] = c
				}
			}
		}
	}
	matchedCiting := map[string]int{}
	for _, c := range cands {
		for f := range files(c.text) {
			matchedCiting[f]++
		}
	}
	matched := map[int]bool{}
	for _, c := range cands {
		matched[c.line] = true
	}
	out := []paragraph{}
	for _, p := range all {
		if matched[p.line] {
			continue
		}
		for f := range files(p.text) {
			// Specific to the matches: at least two of them cite it and they are most of its citers.
			// Never above the best match it follows: it tells more of that mechanism, it does not lead.
			if v := min(hub[f]*0.6, hubLine[f].score*0.8); matchedCiting[f] >= 2 && matchedCiting[f]*2 >= citing[f] && v > p.score {
				p.via, p.score = hubLine[f].line, v
			}
		}
		if p.via > 0 {
			out = append(out, p)
		}
	}
	return out
}

// matchingRows keeps a table's header and the rows that hold a term.
func matchingRows(table string, terms []askTerm) string {
	rows := strings.Split(table, "\n")
	if len(rows) <= 4 {
		return table
	}
	out := rows[:2]
	for _, r := range rows[2:] {
		f := fold(r)
		for _, t := range terms {
			if t.in(f) {
				out = append(out, r)
				break
			}
		}
	}
	if len(out) == 2 {
		return table
	}
	return strings.Join(out, "\n")
}

// difference lists, in order, the tokens of text that base lacks (at most four): what makes an
// alike paragraph different (production/acco, outlet).
func difference(text, base string) string {
	have := map[string]bool{}
	for _, w := range strings.Fields(fold(base)) {
		have[strings.Trim(w, ".,;:()`[]")] = true
	}
	out := []string{}
	for _, w := range strings.Fields(text) {
		k := strings.Trim(fold(w), ".,;:()`[]")
		if k != "" && !have[k] && !strings.HasPrefix(k, "^") && !strings.HasPrefix(k, "[^") {
			have[k] = true
			out = append(out, strings.Trim(w, ".,;:()`[]"))
			if len(out) == 4 {
				break
			}
		}
	}
	return strings.Join(out, " ")
}

func signature(text string) []string {
	set := map[string]bool{}
	for _, w := range words.FindAllString(fold(text), -1) {
		if len(w) >= 3 {
			set[w] = true
		}
	}
	out := make([]string, 0, len(set))
	for w := range set {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter, i, j := 0, 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			inter++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}
