package retrieval

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A note's unit of knowledge is a paragraph: a bullet, a numbered step, a table, a block of prose.
// `read` shows paragraphs, not index chunks, so each shows whole, with its line, once.

type paragraph struct {
	line    int // first line, 1-based
	section string
	text    string
	table   bool
}

var listItem = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s`)

// end is the paragraph's last line.
func (p paragraph) end() int { return p.line + strings.Count(p.text, "\n") }

// renderWithin writes the paragraph with its line, cut at limit characters (0: whole).
func (p paragraph) renderWithin(limit int) string {
	text := p.text
	if limit > 0 && runeLen(text) > limit {
		cut := string([]rune(text)[:limit])
		if k := strings.LastIndexAny(cut, ".;"); k > limit/2 {
			cut = cut[:k+1]
		}
		text = cut + fmt.Sprintf(" … (cut: `kos read --note NAME --lines %d-%d` has it whole)", p.line, p.end())
	}
	if p.table {
		return fmt.Sprintf("L%d\n%s", p.line, text)
	}
	return fmt.Sprintf("L%d %s", p.line, text)
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
