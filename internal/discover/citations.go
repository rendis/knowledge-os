package discover

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	footnoteParts = regexp.MustCompile(`^(\[\^[^\]]+\]:\s*)(.+?)( — .*)$`)
	markdownLink  = regexp.MustCompile(`^\[[^\]]*\]\((https://github\.com/[^)\s]+)\)$`)
)

// ownNames are the repository names a note's permalinks may use for its own repository: its stem and aliases.
func ownNames(notePath string, fm map[string]string) map[string]bool {
	names := map[string]bool{strings.ToLower(strings.TrimSuffix(filepath.Base(notePath), ".md")): true}
	if m := aliasList.FindStringSubmatch(fm["aliases"]); m != nil {
		for _, a := range strings.Split(m[1], ",") {
			if a = strings.ToLower(strings.Trim(strings.TrimSpace(a), `"'`)); a != "" {
				names[a] = true
			}
		}
	}
	return names
}

// shortenCitations rewrites the footnotes of a repository note whose every link is a permalink to the note's own
// repository at its commit-analizado into the short form `path#Lfrom-Lto, … — text`. A footnote that also cites
// another repository or another commit keeps its permalinks, so each footnote has one form.
func shortenCitations(text, notePath string, fm map[string]string) (string, int) {
	commit := strings.Trim(fm["commit-analizado"], `"'`)
	if commit == "" {
		return text, 0
	}
	names := ownNames(notePath, fm)
	count := 0
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		m := footnoteParts.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		short := []string{}
		for _, item := range strings.Split(m[2], ", ") {
			l := markdownLink.FindStringSubmatch(strings.TrimSpace(item))
			if l == nil {
				short = nil
				break
			}
			p := permalink.FindStringSubmatch(l[1])
			if p == nil || p[0] != l[1] || !names[strings.ToLower(p[2])] || !strings.HasPrefix(p[3], commit) && !strings.HasPrefix(commit, p[3]) {
				short = nil
				break
			}
			ref := p[4]
			switch {
			case p[5] != "" && p[6] != "":
				ref += "#L" + p[5] + "-L" + p[6]
			case p[5] != "":
				ref += "#L" + p[5]
			}
			if r := relativeRef.FindStringSubmatch(ref); r == nil || !shortRefLooksLikePath(r[1], r[2] != "") {
				short = nil // a path the short form cannot express keeps its permalink
				break
			}
			short = append(short, ref)
		}
		if len(short) > 0 {
			lines[i] = m[1] + strings.Join(short, ", ") + m[3]
			count++
		}
	}
	return strings.Join(lines, "\n"), count
}

// runShorten applies shortenCitations to repository notes in place.
func runShorten(o options, out io.Writer) error {
	if len(o.notes) == 0 {
		return errors.New("--note is required")
	}
	results := []map[string]any{}
	for _, n := range o.notes {
		full := n
		if !filepath.IsAbs(full) {
			full = filepath.Join(o.vault, n)
		}
		b, e := os.ReadFile(full)
		if e != nil {
			return e
		}
		fm := frontmatterOf(b)
		if _, ok := fm["commit-analizado"]; !ok {
			return fmt.Errorf("%s is not a repository note", n)
		}
		text, count := shortenCitations(string(b), full, fm)
		if count > 0 {
			if e := os.WriteFile(full, []byte(text), 0o644); e != nil {
				return e
			}
		}
		results = append(results, map[string]any{"note": n, "shortened": count})
	}
	return emit(out, map[string]any{"notes": results})
}
