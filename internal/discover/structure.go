package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// G4: whether a repository note is usable as a map, not only whether its citations hold. Its findings are
// errors: a synced note must be free of them, and a note touched only to re-anchor must not add any
// (sync verify keeps pre-existing ones as debt until the note's next sync).
//
//   - G4-relation: a messaging resource the repository names has a topic or event note, but no relation field
//     declares it (producer `publica-en`, consumer `gatillado-por`) and `Limitaciones y desconocimientos`
//     does not explain it. Without the typed relation, "who publishes or consumes X" has no answer in the
//     vault itself.
//   - G4-structure: a core section says nothing was observed while cited claims sit in other sections, so a
//     reader of that section is told the opposite of what the note knows.
//   - G4-duplicate: a paragraph repeats another note's text; one note owns a fact and the others link it.

var (
	coreSections  = []string{"Gatillo", "Qué hace", "Entradas y salidas", "Persistencia y datos"}
	notObserved   = regexp.MustCompile(`(?i)no observad|not observed`)
	footnoteRef   = regexp.MustCompile(`\[\^[^\]]+\]`)
	listMarker    = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+(?:\[[ xX]\]\s+)?`)
	nonWord       = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	canonicalRoot = regexp.MustCompile(`^[1-7]\d-`)
	blankLine     = regexp.MustCompile(`\n\s*\n`)
)

// sections splits a note body into its level-2 sections, in order.
func sections(body string) (titles []string, texts map[string]string) {
	texts = map[string]string{}
	cur := ""
	var b strings.Builder
	flush := func() {
		if cur != "" {
			texts[cur] += b.String()
		}
		b.Reset()
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			cur = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			titles = append(titles, cur)
			continue
		}
		if footnoteDef.MatchString(line) {
			continue // definitions close the note; they belong to no section
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	flush()
	return titles, texts
}

// emptyCoreSections reports core sections that only state an absence while most cited claims (footnote
// references or permalinks) sit outside the core sections, for example all behavior under the
// infrastructure section. A library whose Gatillo is empty while Qué hace carries the claims is fine.
func emptyCoreSections(body string) []checkIssue {
	titles, texts := sections(body)
	empty := []string{}
	core, other, largest, largestRefs := 0, 0, "", 0
	for _, t := range titles {
		text := texts[t]
		refs := len(footnoteRef.FindAllString(text, -1)) + len(permalink.FindAllString(text, -1))
		words := len(strings.Fields(footnoteRef.ReplaceAllString(text, "")))
		switch {
		case slices.Contains(coreSections, t) && words < 30 && notObserved.MatchString(text):
			empty = append(empty, t)
		case slices.Contains(coreSections, t):
			core += refs
		case t != "Limitaciones y desconocimientos" && t != "Relaciones":
			other += refs
			if refs > largestRefs {
				largest, largestRefs = t, refs
			}
		}
	}
	if len(empty) == 0 || other < 5 || other <= core {
		return nil
	}
	out := []checkIssue{}
	for _, s := range empty {
		out = append(out, checkIssue{"G4-structure", "error", s, fmt.Sprintf(
			"%s states that nothing was observed while %d cited claims sit outside the core sections (%d in %s); place each claim under the section it answers, or confirm the absence",
			s, other, largestRefs, largest)})
	}
	return out
}

// undeclaredRelations lists topic and event notes that the repository's messaging resources name but the
// note neither declares in a relation field nor explains under Limitaciones y desconocimientos.
func undeclaredRelations(vault string, fm map[string]string, body string, f repoFacts) []checkIssue {
	if len(f.Languages) == 0 {
		return nil // infrastructure repositories declare many resources; cell-wide gaps cover them
	}
	notes, e := loadNotes(vault)
	if e != nil {
		return nil
	}
	addressed := map[string]bool{}
	for _, field := range edgeFields {
		for _, m := range wikilink.FindAllStringSubmatch(fm[field], -1) {
			addressed[strings.TrimSpace(m[1])] = true
		}
	}
	_, texts := sections(body)
	for _, m := range wikilink.FindAllStringSubmatch(texts["Limitaciones y desconocimientos"], -1) {
		addressed[strings.TrimSpace(m[1])] = true
	}
	type use struct {
		directions map[string]bool
		names      []string
	}
	uses := map[string]*use{}
	for _, r := range f.Resources {
		own := false
		for _, ev := range r.Evidence {
			own = own || ev.Kind == "config" || ev.Kind == "code"
		}
		if !isMessaging(r.Type) || !own {
			continue
		}
		candidates := append([]string{r.Name}, r.Events...)
		if r.Topic != "" {
			candidates = append(candidates, r.Topic)
		}
		for stem, n := range notes {
			if n.folder != "25-Topics" || addressed[stem] {
				continue
			}
			// The note's logical name must be contained in the physical one (country, environment or -sub
			// suffixes); the looser nameMatch would also pair a topic with its -deadletter or eod- variant.
			hit := false
			for _, d := range n.names(stem) {
				for _, c := range candidates {
					hit = hit || subset(tokens(d), tokens(normalizeResource(c)))
				}
			}
			if !hit {
				continue
			}
			u := uses[stem]
			if u == nil {
				u = &use{directions: map[string]bool{}}
				uses[stem] = u
			}
			u.directions[r.Direction] = true
			u.names = appendUnique(u.names, normalizeResource(r.Name))
		}
	}
	out := []checkIssue{}
	for _, stem := range sortedKeys(uses) {
		u := uses[stem]
		field := "`publica-en` (it publishes) or `gatillado-por` (it consumes)"
		switch {
		case u.directions["publish"] && !u.directions["consume"]:
			field = "`publica-en`"
		case u.directions["consume"] && !u.directions["publish"]:
			field = "`gatillado-por`"
		}
		out = append(out, checkIssue{"G4-relation", "error", "[[" + stem + "]]", fmt.Sprintf(
			"the repository names %s but no relation field declares [[%s]]; declare it in %s, or explain under Limitaciones y desconocimientos why it is not a relation",
			strings.Join(firstList(u.names, 3), ", "), stem, field)})
	}
	return out
}

func normalizeParagraph(p string) string {
	p = strings.TrimSpace(p)
	p = listMarker.ReplaceAllString(p, "")
	p = footnoteRef.ReplaceAllString(p, " ")
	return strings.TrimSpace(nonWord.ReplaceAllString(strings.ToLower(p), " "))
}

// paragraphs are the note body's prose blocks and list items long enough to state a fact.
func paragraphs(text string) []string {
	body := text
	if i := strings.Index(text[min(3, len(text)):], "\n---"); strings.HasPrefix(text, "---") && i >= 0 {
		body = text[3+i+4:]
	}
	out := []string{}
	for _, raw := range blankLine.Split(body, -1) {
		kept := []string{}
		for _, line := range strings.Split(raw, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") && !footnoteDef.MatchString(line) {
				kept = append(kept, line)
			}
		}
		block := strings.Join(kept, "\n")
		items := []string{block}
		if listMarker.MatchString(strings.TrimSpace(block)) {
			items = strings.Split(block, "\n")
		}
		for _, it := range items {
			if n := normalizeParagraph(it); len(strings.Fields(n)) >= 20 {
				out = append(out, n)
			}
		}
	}
	return out
}

// duplicateParagraphs reports, per topic, flow, integration or other non-repository note, the paragraphs of
// this note it repeats verbatim. Two repository notes may state the same fact about their own identical
// configuration (a shared pipeline template), so repository notes are not compared with each other.
func duplicateParagraphs(vault, notePath string, text []byte) []checkIssue {
	own := map[string]bool{}
	for _, p := range paragraphs(string(text)) {
		own[p] = true
	}
	if len(own) == 0 {
		return nil
	}
	self := strings.TrimSuffix(filepath.Base(notePath), ".md")
	repeated := map[string]int{}
	_ = filepath.WalkDir(vault, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		rel, _ := filepath.Rel(vault, p)
		if d.IsDir() {
			top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || !canonicalRoot.MatchString(top) || top == "20-Repos") {
				return filepath.SkipDir
			}
			return nil
		}
		stem := strings.TrimSuffix(d.Name(), ".md")
		if !strings.HasSuffix(p, ".md") || stem == self {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil
		}
		seen := map[string]bool{}
		for _, q := range paragraphs(string(b)) {
			if own[q] && !seen[q] {
				seen[q] = true
				repeated[stem]++
			}
		}
		return nil
	})
	stems := sortedKeys(repeated)
	sort.SliceStable(stems, func(i, j int) bool { return repeated[stems[i]] > repeated[stems[j]] })
	out := []checkIssue{}
	for _, stem := range stems {
		out = append(out, checkIssue{"G4-duplicate", "error", "[[" + stem + "]]", fmt.Sprintf(
			"%d paragraph(s) repeat text of [[%s]]; keep the fact in the note that owns it and link that note from the other", repeated[stem], stem)})
	}
	return out
}

// CopiesIntroduced gates a topic, flow, integration or other non-repository note changed on a sync branch:
// the paragraphs it copies verbatim from a repository note, minus those its base version already copied.
func CopiesIntroduced(vault, note string, base []byte) ([]string, error) {
	text, e := os.ReadFile(filepath.Join(vault, note))
	if e != nil {
		return nil, e
	}
	owners := map[string][]string{}
	_ = filepath.WalkDir(filepath.Join(vault, "20-Repos"), func(p string, d os.DirEntry, e error) error {
		if e != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		if b, e := os.ReadFile(p); e == nil {
			for _, q := range paragraphs(string(b)) {
				owners[q] = appendUnique(owners[q], strings.TrimSuffix(d.Name(), ".md"))
			}
		}
		return nil
	})
	before := map[string]bool{}
	if base != nil {
		for _, q := range paragraphs(string(base)) {
			before[q] = true
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, q := range paragraphs(string(text)) {
		if o := owners[q]; len(o) > 0 && !before[q] && !seen[q] {
			seen[q] = true
			words := strings.Fields(q)
			out = append(out, fmt.Sprintf("\"%s…\" copies [[%s]]", strings.Join(words[:min(8, len(words))], " "), strings.Join(o, "]], [[")))
		}
	}
	return out, nil
}
