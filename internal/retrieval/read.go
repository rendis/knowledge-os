package retrieval

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"knowledge-os/internal/discover"
)

// ReadBudget is `read`'s default size: a section with its sources, well under a harness's limit.
const ReadBudget = 24000

// ReadOptions select what `read` returns from a note.
type ReadOptions struct {
	Section string // a heading, matched by its folded text
	Lines   string // FROM-TO
	Match   string // keep the paragraphs that hold one of these terms
	Brief   bool   // the paragraphs and one line of source marks
	Budget  int
	Code    bool
}

// Read writes one note, one of its sections or a line range, followed by the footnotes that text
// cites with their state on the reference branch and the cited lines: the note's evidence without
// the 300 lines between a claim and its footnote.
func (i *Index) Read(ctx context.Context, name string, o ReadOptions, w io.Writer) error {
	if o.Budget < 4000 || o.Budget > 200000 {
		return fmt.Errorf("--budget must be between 4000 and 200000 characters")
	}
	notes, err := i.knowledgeNotes(ctx)
	if err != nil {
		return err
	}
	note, err := findNote(notes, name)
	if err != nil {
		return err
	}
	lines := strings.Split(string(note.raw), "\n")
	ranges := [][2]int{}
	parts, field := strings.Split(o.Lines, ","), "lines"
	if o.Lines == "" && strings.ContainsAny(o.Section, "|,") {
		parts, field = strings.FieldsFunc(o.Section, func(r rune) bool { return r == '|' || r == ',' }), "section"
	}
	for _, part := range parts {
		oo := o
		if field == "lines" {
			oo.Lines = strings.TrimSpace(part)
		} else {
			oo.Section = strings.TrimSpace(part)
		}
		f, t, err := readRange(lines, oo)
		if err != nil {
			return fmt.Errorf("%s: %w", note.path, err)
		}
		if f == t && field == "lines" && !strings.Contains(part, "-") {
			// One line may fall on the blank between bullets: take the paragraph nearest to it.
			f, t = max(1, f-2), min(len(lines), t+2)
		}
		ranges = append(ranges, [2]int{f, t})
	}
	sort.Slice(ranges, func(a, b int) bool { return ranges[a][0] < ranges[b][0] })
	from, to := ranges[0][0], ranges[len(ranges)-1][1]
	// A paragraph belongs when it overlaps a range: a single line inside a bullet selects the bullet.
	inRange := func(from, to int) bool {
		for _, r := range ranges {
			if from <= r[1] && to >= r[0] {
				return true
			}
		}
		return false
	}
	out := &packWriter{w: w, budget: o.Budget}
	fmt.Fprintf(out, "# %s\n", note.path)
	if o.Lines == "" {
		// The whole note or a section: its summary and outline. A line range is a follow-up read:
		// the header was in the pack already.
		fmt.Fprintf(out, "\n%s\n", overviewLine(note.stem, note.raw))
		if sections := noteSections(note.raw); len(sections) > 0 {
			fmt.Fprintf(out, "Sections: %s\n", strings.Join(sections, " · "))
		}
	}
	repo, commit := "", ""
	if strings.HasPrefix(note.path, "20-Repos/") {
		if s, ok := discover.RepositoryNoteState(i.Root, note.path); ok {
			writeRepoState(s, out)
			repo, commit = s.Repo, s.Commit
		}
	}
	if strings.HasPrefix(note.path, "25-Topics/") {
		if w, ok := discover.TopicWiring(i.Root, note.stem); ok {
			writeWiring(w, discover.NewSources(i.Root), out, askTerms(o.Match))
		}
	}
	defs := footnoteDefinitions(string(note.raw))
	// Paragraphs with their lines, within the budget but leaving room for their sources.
	var terms []askTerm
	if o.Match != "" {
		terms = askTerms(o.Match)
	}
	textBudget := out.left() * 2 / 3
	var body strings.Builder
	shown, end, section := 0, to, ""
	var prev *paragraph
	folded, foldedDiff := []int{}, []string{}
	flushFolded := func() {
		if len(folded) > 0 {
			ls := []string{}
			for k, l := range folded {
				d := ""
				if foldedDiff[k] != "" {
					d = " [" + foldedDiff[k] + "]"
				}
				ls = append(ls, fmt.Sprintf("L%d%s", l, d))
			}
			fmt.Fprintf(&body, "(+%d alike: %s)\n", len(folded), strings.Join(ls, ", "))
			folded, foldedDiff = nil, nil
		}
	}
	for _, p := range parseParagraphs(note.raw) {
		if !inRange(p.line, p.end()) {
			continue
		}
		if len(terms) > 0 && !matchesAny(p.text, terms) {
			continue
		}
		// A run of near-identical paragraphs (one per environment) reads as the first, then each
		// other's line and what differs in it.
		if prev != nil && p.section == prev.section && !p.table && jaccard(signature(p.text), signature(prev.text)) >= 0.6 {
			folded = append(folded, p.line)
			foldedDiff = append(foldedDiff, difference(p.text, prev.text))
			continue
		}
		flushFolded()
		pp := p
		prev = &pp
		chunk := ""
		if p.section != section {
			section = p.section
			chunk = "\n### " + section + "\n"
		}
		text := p.renderWithin(0) // read never cuts a paragraph
		if other := paragraphRepo(p.text, defs); other != "" && !strings.EqualFold(other, repo) {
			text = strings.Replace(text, fmt.Sprintf("L%d", p.line), fmt.Sprintf("L%d (%s)", p.line, other), 1)
		}
		chunk += "\n" + text + "\n"
		if shown > 0 && runeLen(body.String())+runeLen(chunk) > textBudget {
			end = p.line - 1
			break
		}
		body.WriteString(chunk)
		shown++
	}
	flushFolded()
	if shown == 0 && o.Match == "" {
		return fmt.Errorf("%s: no paragraph in those lines (sections: %s)", note.path, strings.Join(noteSections(note.raw), " · "))
	}
	what := fmt.Sprintf("L%d-L%d", from, end)
	if o.Match != "" {
		what += fmt.Sprintf(", %d paragraphs matching %q", shown, o.Match)
	}
	fmt.Fprintf(out, "\n## %s\n%s", what, body.String())
	if end < to {
		fmt.Fprintf(out, "\n… continues at L%d (`--lines %d-%d`, or `--budget 60000` for the rest in one read).\n", end+1, end+1, to)
	}
	// Paragraphs first: the cited lines get at most a quarter of what remains.
	render := &sourceRenderer{src: discover.NewSources(i.Root), code: o.Code, codeLeft: out.left() / 4, perText: 200, brief: o.Brief}
	render.writeCited(out, body.String(), defs, map[string]bool{}, repo, commit)
	return nil
}

// readRange resolves --section or --lines to 1-based inclusive line numbers; neither means the note.
func readRange(lines []string, o ReadOptions) (int, int, error) {
	if o.Lines != "" {
		parts := strings.SplitN(o.Lines, "-", 2)
		from, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		to := from
		var e2 error
		if len(parts) == 2 {
			to, e2 = strconv.Atoi(strings.TrimSpace(parts[1]))
		}
		if e1 != nil || e2 != nil || from < 1 || to < from {
			return 0, 0, fmt.Errorf("--lines must be FROM-TO, e.g. 300-340")
		}
		return from, min(to, len(lines)), nil
	}
	if o.Section == "" {
		return 1, len(lines), nil
	}
	want := fold(strings.TrimSpace(strings.TrimLeft(o.Section, "# ")))
	start, level := 0, 0
	headings := []string{}
	fence := false
	for n, l := range lines {
		if strings.HasPrefix(l, "```") {
			fence = !fence
		}
		if fence || !strings.HasPrefix(l, "#") {
			continue
		}
		lv := len(l) - len(strings.TrimLeft(l, "#"))
		title := strings.TrimSpace(l[lv:])
		if start == 0 {
			headings = append(headings, title)
			if strings.Contains(fold(title), want) {
				start, level = n+1, lv
			}
			continue
		}
		if lv <= level {
			return start, n, nil
		}
	}
	if start == 0 {
		return 0, 0, fmt.Errorf("no section matches %q; sections: %s", o.Section, strings.Join(firstN(headings, 20), " · "))
	}
	return start, len(lines), nil
}

// findNote resolves a basename, alias or path to a note, suggesting near names when none matches.
func findNote(notes []*knowledgeNote, name string) (*knowledgeNote, error) {
	var note *knowledgeNote
	f := fold(strings.TrimSuffix(strings.TrimSpace(name), ".md"))
	for _, n := range notes {
		if fold(strings.TrimSuffix(n.path, ".md")) == f || fold(n.stem) == f {
			note = n
			break
		}
	}
	if note == nil {
		for _, n := range notes {
			for _, a := range n.aliases {
				if fold(a) == f {
					note = n
				}
			}
		}
	}
	if note == nil {
		candidates := []string{}
		for _, n := range notes {
			if strings.Contains(fold(n.stem), f) {
				candidates = append(candidates, n.stem)
			}
		}
		if len(candidates) > 0 {
			return nil, fmt.Errorf("no note named %q; did you mean: %s", name, strings.Join(firstN(candidates, 8), ", "))
		}
		return nil, fmt.Errorf("no note named %q (use a basename, an alias or a path)", name)
	}
	return note, nil
}

func matchesAny(text string, terms []askTerm) bool {
	f := fold(text)
	for _, t := range terms {
		if t.in(f) {
			return true
		}
	}
	return false
}

// paragraphRepo is the one repository all the footnotes of a paragraph cite, or "".
func paragraphRepo(text string, defs map[string]string) string {
	cites := []citation{}
	for _, m := range footnoteRef.FindAllStringSubmatch(text, -1) {
		if d, ok := defs[m[1]]; ok {
			cites = append(cites, citation{def: d})
		}
	}
	return citedRepo(cites)
}
