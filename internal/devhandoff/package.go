// Package devhandoff prepares a repository worktree for one atomic development task defined in a
// development investigation, so an agent working there knows exactly what to do without the user
// passing context. The task package lives in the case (handoffs/DH-NNN.md); start copies it into the
// worktree's .handoff/ (excluded from Git locally), a stable managed segment in AGENTS.md tells any
// harness how to work with it, and progress is the branch itself plus .handoff/deltas.md.
package devhandoff

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Package is a parsed task package.
type Package struct {
	Path       string            `json:"path"`
	Handoff    string            `json:"handoff"`
	Case       string            `json:"case"`
	Repository string            `json:"repository"`
	Base       string            `json:"base"`
	Branch     string            `json:"branch"`
	Title      string            `json:"title"`
	DependsOn  []string          `json:"depends_on,omitempty"`
	SHA256     string            `json:"sha256"`
	Fields     map[string]string `json:"-"`
	Text       string            `json:"-"`
}

// Issue is one package gate finding.
type Issue struct {
	Severity string `json:"severity"` // error | warning
	Detail   string `json:"detail"`
}

var (
	fmLine     = regexp.MustCompile(`^([a-z][a-z0-9-]*):\s*(.*)$`)
	handoffID  = regexp.MustCompile(`^DH-\d{3,}$`)
	branchName = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)
	heading1   = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	heading2   = regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`)
	wikilink   = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	comment    = regexp.MustCompile(`(?s)<!--.*?-->`)
	secret     = regexp.MustCompile(`(?i)PRIVATE KEY-----|\b(?:password|passwd|pwd|secret|token|api[-_]?key)\s*[:=]\s*\S+|\b(?:gh[pousr]_|github_pat_|sk-(?:proj-|svcacct-)?|xox[baprs]-)\S+`)
	localPath  = regexp.MustCompile("(?:^|[^A-Za-z0-9])(?:/(?:Users|home|tmp|var/tmp|private/tmp)/[^\\s`)]+|[A-Za-z]:\\\\[^\\s`]+)")
	wordToken  = regexp.MustCompile(`[\p{L}\p{N}]+`)
)

// Package sections in both note locales; the first three are required and must not be empty.
var sections = []struct {
	key      string
	names    []string
	required bool
}{
	{"task", []string{"Tarea", "Task"}, true},
	{"changes", []string{"Cambios", "Changes"}, true},
	{"acceptance", []string{"Criterios de aceptación", "Acceptance criteria"}, true},
	{"context", []string{"Contexto necesario", "Required context"}, false},
	{"out", []string{"Fuera de alcance", "Out of scope"}, false},
	{"questions", []string{"Preguntas abiertas", "Open questions"}, false},
	{"references", []string{"Referencias", "References"}, false},
}

func frontmatter(text string) (map[string]string, string) {
	out := map[string]string{}
	if !strings.HasPrefix(text, "---\n") {
		return out, text
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return out, text
	}
	for _, l := range strings.Split(text[4:4+end], "\n") {
		if m := fmLine.FindStringSubmatch(l); m != nil {
			out[m[1]] = strings.Trim(strings.TrimSpace(m[2]), `"'`)
		}
	}
	rest := text[4+end+4:]
	return out, strings.TrimPrefix(rest, "\n")
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// sectionBodies maps a section key to its body text (comments removed).
func sectionBodies(body string) map[string]string {
	out := map[string]string{}
	idx := heading2.FindAllStringSubmatchIndex(body, -1)
	for i, m := range idx {
		title := strings.TrimSpace(body[m[2]:m[3]])
		end := len(body)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		for _, s := range sections {
			for _, n := range s.names {
				if strings.EqualFold(n, title) {
					out[s.key] = strings.TrimSpace(comment.ReplaceAllString(body[m[1]:end], ""))
				}
			}
		}
	}
	return out
}

// ReadPackage parses a task package without judging it.
func ReadPackage(path string) (Package, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return Package{}, e
	}
	fm, body := frontmatter(string(b))
	p := Package{Path: path, Handoff: fm["handoff"], Case: fm["case"], Repository: fm["repository"], Base: fm["base"], Branch: fm["branch"],
		SHA256: digest(b), Fields: fm, Text: string(b)}
	if m := heading1.FindStringSubmatch(body); m != nil {
		p.Title = m[1]
	}
	for _, d := range strings.FieldsFunc(fm["depends-on"], func(r rune) bool { return r == ',' || r == ' ' || r == '[' || r == ']' }) {
		p.DependsOn = append(p.DependsOn, d)
	}
	return p, nil
}

// CheckPackage is the deterministic gate for a task package: identity, the three required sections,
// self-sufficiency (no vault links: the implementing agent has no vault), no credential or local path,
// and a size that keeps the task atomic.
func CheckPackage(path string) (Package, []Issue, error) {
	p, e := ReadPackage(path)
	if e != nil {
		return p, nil, e
	}
	issues := []Issue{}
	add := func(sev, d string) { issues = append(issues, Issue{sev, d}) }
	if !handoffID.MatchString(p.Handoff) {
		add("error", "frontmatter handoff must be DH-NNN")
	}
	for _, k := range []string{"case", "repository", "base", "branch"} {
		if p.Fields[k] == "" {
			add("error", "frontmatter "+k+" is required")
		}
	}
	if p.Branch != "" && (!branchName.MatchString(p.Branch) || strings.Contains(p.Branch, "..")) {
		add("error", "branch is not a valid Git branch name")
	}
	for _, d := range p.DependsOn {
		if !handoffID.MatchString(d) || d == p.Handoff {
			add("error", "depends-on lists other handoffs of the case as DH-NNN: "+d)
		}
	}
	if p.Title == "" {
		add("error", "the package needs a title (# heading) that names the task")
	}
	_, body := frontmatter(p.Text)
	bodies := sectionBodies(body)
	for _, s := range sections {
		if s.required && bodies[s.key] == "" {
			add("error", "section "+s.names[0]+" / "+s.names[1]+" is required and must not be empty")
		}
	}
	clean := comment.ReplaceAllString(body, "")
	for _, m := range wikilink.FindAllStringSubmatch(clean, 5) {
		add("error", "[["+m[1]+"]] points into the vault, which the implementing agent does not have: write the needed fact here or cite a permalink")
	}
	for _, m := range credentials(clean, 3) {
		add("error", "credential value in the package ("+string([]rune(m)[:min(20, len([]rune(m)))])+"…): remove it")
	}
	for _, m := range localPath.FindAllString(clean, 3) {
		add("error", "`"+strings.TrimSpace(m)+"` is a local path of another machine: use a repository path or permalink")
	}
	if n := len(wordToken.FindAllString(clean, -1)); n > 2000 {
		add("warning", "the package has "+itoa(n)+" words: keep only what this repository's task needs (atomic handoff)")
	}
	return p, issues, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func blocking(issues []Issue) error {
	msgs := []string{}
	for _, i := range issues {
		if i.Severity == "error" {
			msgs = append(msgs, i.Detail)
		}
	}
	if len(msgs) > 0 {
		return errors.New("task package fails its gate: " + strings.Join(msgs, "; "))
	}
	return nil
}

func relOrAbs(base, p string) string {
	if r, e := filepath.Rel(base, p); e == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return p
}

// envReference matches a value that names where a secret comes from instead of holding it.
var envReference = regexp.MustCompile(`(?i)[:=]\s*["'` + "`" + `]?(?:process\.env|os\.(?:environ|getenv)|\$\{|\$[A-Z_]|<|env\(|secret(?:s)?\.|vault:|\*{3,})`)

// credentials returns up to n credential values in text, skipping references to where a secret lives.
func credentials(text string, n int) []string {
	out := []string{}
	for _, m := range secret.FindAllString(text, -1) {
		if envReference.MatchString(m) {
			continue
		}
		out = append(out, m)
		if len(out) == n {
			break
		}
	}
	return out
}
