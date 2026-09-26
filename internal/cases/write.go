package cases

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"knowledge-os/internal/config"
	"knowledge-os/internal/devhandoff"
)

// Every change to a case goes through these commands so that all cases keep one structure: the CLI
// assigns record IDs, requires the fields each record needs (a source for evidence, a level for a
// conclusion, an origin for a requirement), logs the change, and refuses a write that would introduce
// a gate error before it reaches the file.

var (
	heading2Line = regexp.MustCompile(`^##\s+(.+?)\s*$`)
	idList       = regexp.MustCompile(`^(?:AC|DH|CH|[EDQRSAF])-\d{3,}$`)
	unsafeName   = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
)

func noteLocale(vault string) string {
	if inst, e := config.LoadInstance(vault); e == nil {
		if l, _ := inst["locale"].(map[string]any); l != nil {
			if v, _ := l["notes"].(string); v == "en" {
				return "en"
			}
		}
	}
	return "es"
}

func today(o options) string {
	if o.date != "" {
		return o.date
	}
	return time.Now().Format("2006-01-02")
}

// findCase resolves a writable case. A published case is versioned knowledge: it changes only on a
// sync branch, where sync verify gates it before it reaches the default branch.
func findCase(vault, id string) (Case, error) {
	if id == "" {
		return Case{}, errors.New("--id is required")
	}
	all, e := List(vault)
	if e != nil {
		return Case{}, e
	}
	for _, c := range all {
		if c.ID != id {
			continue
		}
		if c.Path == "" {
			return c, fmt.Errorf("case %s is %s and has no file to change", id, c.Status)
		}
		if c.Visibility == "published" {
			branch, _ := exec.Command("git", "-C", vault, "rev-parse", "--abbrev-ref", "HEAD").Output()
			if !strings.HasPrefix(strings.TrimSpace(string(branch)), "sync/") {
				return c, fmt.Errorf("case %s is published: change it on a sync branch (sync start --name case-%s)", id, id)
			}
		}
		return c, nil
	}
	return Case{}, fmt.Errorf("no case %s", id)
}

// visible strips comments and fenced blocks, which hold guidance and examples rather than records.
func visible(text string) string {
	return fence.ReplaceAllString(comment.ReplaceAllString(text, ""), "")
}

func definedRecords(text string) map[string]bool {
	out := map[string]bool{}
	for _, l := range strings.Split(visible(text), "\n") {
		if m := recordDef.FindStringSubmatch(l); m != nil {
			out[m[1]] = true
		}
	}
	return out
}

func nextID(text, prefix string) string {
	max := 0
	for id := range definedRecords(text) {
		p, n, _ := strings.Cut(id, "-")
		if p == prefix {
			if v, e := strconv.Atoi(n); e == nil && v > max {
				max = v
			}
		}
	}
	return fmt.Sprintf("%s-%03d", prefix, max+1)
}

// sectionSpan returns the line range [start, end) of the body of the section with key, or -1.
func sectionSpan(lines []string, key string) (int, int) {
	start, inFence := -1, false
	for i, l := range lines {
		if strings.HasPrefix(l, "```") {
			inFence = !inFence
		}
		if inFence {
			continue
		}
		if m := heading2Line.FindStringSubmatch(l); m != nil {
			if start >= 0 {
				return start, i
			}
			if sectionKey(m[1]) == key {
				start = i + 1
			}
		}
	}
	if start >= 0 {
		return start, len(lines)
	}
	return -1, -1
}

// appendToSection adds lines at the end of a section, creating the section (before the log, or at the
// end) when the case lacks it.
func appendToSection(text, key, locale string, add ...string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	start, end := sectionSpan(lines, key)
	if start < 0 {
		block := []string{"## " + sectionNames[locale][key], ""}
		block = append(block, add...)
		at := len(lines)
		if key != "log" {
			if ls, _ := sectionSpan(lines, "log"); ls > 0 {
				at = ls - 1
			}
		}
		out := append([]string{}, lines[:at]...)
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, block...)
		if at < len(lines) {
			out = append(out, "")
		}
		return strings.Join(append(out, lines[at:]...), "\n") + "\n"
	}
	last := end
	for last > start && strings.TrimSpace(lines[last-1]) == "" {
		last--
	}
	out := append([]string{}, lines[:last]...)
	if last > start && !strings.HasPrefix(strings.TrimSpace(lines[last-1]), "- ") {
		out = append(out, "") // after a guidance comment or prose, start the list on its own paragraph
	}
	out = append(out, add...)
	if end < len(lines) {
		out = append(out, "")
	}
	return strings.Join(append(out, lines[end:]...), "\n") + "\n"
}

// replaceSection sets the body of a section, keeping its guidance comment.
func replaceSection(text, key, locale, body string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	start, end := sectionSpan(lines, key)
	if start < 0 {
		return appendToSection(text, key, locale, body)
	}
	keep := []string{""}
	for _, l := range lines[start:end] {
		if strings.HasPrefix(strings.TrimSpace(l), "<!--") && strings.HasSuffix(strings.TrimSpace(l), "-->") {
			keep = append(keep, l, "")
		}
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, keep...)
	out = append(out, strings.TrimSpace(body))
	if end < len(lines) {
		out = append(out, "")
	}
	return strings.Join(append(out, lines[end:]...), "\n") + "\n"
}

// annotate appends a note to the line that defines a record.
func annotate(text, id, note string) string {
	lines := strings.Split(text, "\n")
	inComment := false
	for i, l := range lines {
		if strings.Contains(l, "<!--") {
			inComment = true
		}
		if !inComment {
			if m := recordDef.FindStringSubmatch(l); m != nil && m[1] == id {
				lines[i] = strings.TrimRight(l, " ") + " — " + note
				return strings.Join(lines, "\n")
			}
		}
		if strings.Contains(l, "-->") {
			inComment = false
		}
	}
	return text
}

func setFrontmatter(text, key, value string) string {
	if !strings.HasPrefix(text, "---\n") {
		return text
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return text
	}
	head := strings.Split(text[4:4+end], "\n")
	out := []string{}
	done := false
	for _, l := range head {
		if m := fmLine.FindStringSubmatch(l); m != nil && m[1] == key {
			if value != "" && !done {
				out = append(out, key+": "+value)
				done = true
			}
			continue
		}
		out = append(out, l)
	}
	if value != "" && !done {
		out = append(out, key+": "+value)
	}
	return "---\n" + strings.Join(out, "\n") + text[4+end:]
}

// sentence closes s with a full stop unless it already ends one; a closing bracket or quote ends a
// sentence only when the text inside it does ("(E-001)" gets one, "(see E-001.)" does not).
func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(s, ")]\"'”’»"))
	if !strings.ContainsRune(".?!:", r) {
		s += "."
	}
	return s
}

func clause(s string) string { return strings.TrimRight(strings.TrimSpace(s), ".") }

func splitIDs(s string) []string {
	out := []string{}
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// requireRefs checks that every ID exists in the case and has one of the prefixes.
func requireRefs(text, flag, value string, prefixes ...string) ([]string, error) {
	ids := splitIDs(value)
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s is required", flag)
	}
	defs := definedRecords(text)
	for _, id := range ids {
		if !idList.MatchString(id) {
			return nil, fmt.Errorf("%s: %q is not a record ID", flag, id)
		}
		p, _, _ := strings.Cut(id, "-")
		okPrefix := false
		for _, want := range prefixes {
			okPrefix = okPrefix || p == want
		}
		if !okPrefix {
			return nil, fmt.Errorf("%s accepts %s records, not %s", flag, strings.Join(prefixes, "-/")+"-", id)
		}
		if !defs[id] {
			return nil, fmt.Errorf("%s: %s is not defined in the case", flag, id)
		}
	}
	return ids, nil
}

// mutate applies one change to a case: the change is computed in memory, gated, logged and written.
func mutate(o options, out io.Writer, change func(c Case, text, locale string) (string, []string, map[string]any, error), after func(c Case) error) error {
	c, e := findCase(o.vault, o.id)
	if e != nil {
		return e
	}
	full := filepath.Join(o.vault, c.Path)
	b, e := os.ReadFile(full)
	if e != nil {
		return e
	}
	prev := string(b)
	locale := noteLocale(o.vault)
	next, logParts, res, e := change(c, prev, locale)
	if e != nil {
		return e
	}
	if len(logParts) > 0 {
		next = appendToSection(next, "log", locale, "- "+today(o)+" — "+strings.Join(logParts, "; "))
	}
	ok, introduced, _, gate := introducedErrors(o.vault, full, &prev, next, indexVault(o.vault))
	if !ok {
		_ = emit(out, map[string]any{"ok": false, "id": c.ID, "introduced": introduced})
		return errors.New("the change would introduce gate errors; nothing was written")
	}
	if res == nil {
		res = map[string]any{}
	}
	review := []Issue{}
	for _, i := range gate.Issues {
		if i.Severity == "review" {
			review = append(review, i)
		}
	}
	if len(review) > 0 {
		res["gate_review"] = review // not blocking: shown on every write until addressed
	}
	if o.dryRun {
		res["ok"], res["applied"], res["id"], res["path"] = true, false, c.ID, c.Path
		return emit(out, res)
	}
	if after != nil {
		if e := after(c); e != nil {
			return e
		}
	}
	if e := os.WriteFile(full, []byte(next), 0o644); e != nil {
		return e
	}
	if res == nil {
		res = map[string]any{}
	}
	res["ok"], res["id"], res["path"] = true, c.ID, c.Path
	return emit(out, res)
}

// casePackage reads a task package that belongs to the case (under its handoffs/ directory).
func casePackage(vault string, c Case, pkg string) (devhandoff.Package, error) {
	if pkg == "" {
		return devhandoff.Package{}, errors.New("--package is required: handoffs/DH-NNN.md inside the case")
	}
	dir := filepath.Dir(filepath.Join(vault, c.Path))
	full := pkg
	if !filepath.IsAbs(full) {
		full = filepath.Join(dir, pkg)
		if _, e := os.Stat(full); e != nil {
			full = filepath.Join(vault, pkg)
		}
	}
	if filepath.Dir(full) != filepath.Join(dir, "handoffs") {
		return devhandoff.Package{}, errors.New("the package must live in the case's handoffs/ directory")
	}
	p, issues, e := devhandoff.CheckPackage(full)
	if e != nil {
		return p, e
	}
	for _, i := range issues {
		if i.Severity == "error" {
			return p, errors.New("package fails its gate: " + i.Detail)
		}
	}
	if p.Case != c.ID {
		return p, fmt.Errorf("the package names case %q, not %s", p.Case, c.ID)
	}
	return p, nil
}

func setState(o options, out io.Writer) error {
	if strings.TrimSpace(o.text) == "" {
		return errors.New("--text is required: what is known, what is missing and the next step, citing record IDs")
	}
	return mutate(o, out, func(c Case, text, loc string) (string, []string, map[string]any, error) {
		if len(definedRecords(text)) > 0 && !recordRef.MatchString(o.text) {
			return "", nil, nil, errors.New("the current state must cite the records it summarizes (E-, F-, Q-, D-…)")
		}
		return replaceSection(text, "state", loc, o.text), []string{labels[loc]["state"]}, nil, nil
	}, nil)
}

func closeCase(o options, out io.Writer) error {
	if !outcomes.MatchString(o.outcome) {
		return errors.New("--outcome must be completed, abandoned or superseded-by:<id>")
	}
	if strings.TrimSpace(o.reason) == "" {
		return errors.New("--reason is required: why the case closes and the limits that remain")
	}
	return mutate(o, out, func(c Case, text, loc string) (string, []string, map[string]any, error) {
		if c.Status == "closed" {
			return "", nil, nil, errors.New("the case is already closed")
		}
		text = setFrontmatter(setFrontmatter(text, "status", "closed"), "outcome", o.outcome)
		return text, []string{labels[loc]["closed"] + " (" + o.outcome + "): " + clause(o.reason)}, nil, nil
	}, nil)
}

func reopen(o options, out io.Writer) error {
	if strings.TrimSpace(o.reason) == "" {
		return errors.New("--reason is required: the new evidence or request that reopens the case")
	}
	return mutate(o, out, func(c Case, text, loc string) (string, []string, map[string]any, error) {
		if c.Status != "closed" {
			return "", nil, nil, errors.New("the case is not closed")
		}
		text = setFrontmatter(setFrontmatter(text, "status", "open"), "outcome", "")
		return text, []string{labels[loc]["reopened"] + ": " + clause(o.reason)}, nil, nil
	}, nil)
}
