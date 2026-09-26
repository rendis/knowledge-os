// Package check implements deterministic vault checks, not evidence assessment.
package check

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

var ErrIssues = errors.New("validation found issues")

// Run accepts links|bases --vault PATH (flags may precede the operation).
func Run(args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "obsidian-binding" {
		return runBinding(args[1:], out)
	}
	root, op := ".", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--vault", "--root":
			i++
			if i >= len(args) {
				return errors.New("--vault requires a path")
			}
			root = args[i]
		case "links", "bases":
			if op != "" {
				return errors.New("specify one check")
			}
			op = args[i]
		default:
			return fmt.Errorf("unknown check argument %q", args[i])
		}
	}
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	st, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return errors.New("vault must be a directory")
	}
	if op == "links" {
		r, e := Links(root)
		if e != nil {
			return e
		}
		if e = json.NewEncoder(out).Encode(r); e != nil {
			return e
		}
		if len(r.Broken)+len(r.AliasTargets)+len(r.Hidden)+len(r.Duplicates)+len(r.Orphans) > 0 {
			return ErrIssues
		}
		return nil
	}
	if op == "bases" {
		r, e := Bases(root)
		if e != nil {
			return e
		}
		if e = json.NewEncoder(out).Encode(r); e != nil {
			return e
		}
		if len(r.Issues) > 0 {
			return ErrIssues
		}
		return nil
	}
	return errors.New("usage: check links|bases --vault PATH")
}

func visible(root, ext string, excludePlan bool) ([]string, error) {
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(d.Name(), ".") || d.Name() == "investigations" || (excludePlan && rel == "plan") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Do not follow symlinks outside the vault (or duplicate an aliased tree).
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.IsDir() && filepath.Ext(path) == ext {
			if ext == ".md" && !excludePlan && (rel == "AGENTS.md" || rel == "CLAUDE.md" || rel == "AGENTS.personal.md") {
				return nil
			}
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}
func read(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("%s: must be UTF-8", path)
	}
	return string(b), nil
}
func frontmatter(text string) (map[string]any, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	m := map[string]any{}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return m, nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			err := yaml.Unmarshal([]byte(strings.Join(lines[1:i], "\n")), &m)
			return m, err
		}
	}
	return m, errors.New("unclosed YAML frontmatter")
}

var fenced = regexp.MustCompile("(?s)```.*?```")
var inline = regexp.MustCompile("`[^`\\n]*`")
var wiki = regexp.MustCompile(`\[\[([^\]|#]+)`)
var markdown = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)(?:\s+[^)]*)?\)`)
var hiddenAgent = regexp.MustCompile(`(^|/)\.agents(?:/|$)`)

type LinkResult struct {
	Broken          []string `json:"broken"`
	AliasTargets    []string `json:"alias_targets"`
	Hidden          []string `json:"hidden"`
	Duplicates      []string `json:"duplicates"`
	Orphans         []string `json:"orphans"`
	OrphanPaths     []string `json:"orphan_paths"`
	ExpectedOrphans []string `json:"expected_orphans"`
}

func Links(root string) (LinkResult, error) {
	r := LinkResult{[]string{}, []string{}, []string{}, []string{}, []string{}, []string{}, []string{}}
	paths, e := visible(root, ".md", false)
	if e != nil {
		return r, e
	}
	basePaths, e := visible(root, ".base", false)
	if e != nil {
		return r, e
	}
	notes, aliases, bodies := map[string]string{}, map[string]string{}, map[string]string{}
	duplicates := map[string][]string{}
	bases := map[string]bool{}
	for _, p := range basePaths {
		bases[filepath.Base(p)] = true
	}
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		stem := strings.TrimSuffix(filepath.Base(p), ".md")
		body, err := read(p)
		if err != nil {
			return r, err
		}
		bodies[rel] = body
		if first, ok := notes[stem]; ok {
			if len(duplicates[stem]) == 0 {
				duplicates[stem] = []string{first}
			}
			duplicates[stem] = append(duplicates[stem], rel)
		} else {
			notes[stem] = rel
		}
		fm, err := frontmatter(body)
		if err != nil {
			return r, fmt.Errorf("%s: %w", rel, err)
		}
		switch a := fm["aliases"].(type) {
		case string:
			if a != "" {
				aliases[a] = stem
			}
		case []any:
			for _, v := range a {
				if v != nil && fmt.Sprint(v) != "" {
					aliases[fmt.Sprint(v)] = stem
				}
			}
		}
	}
	inbound, broken, aliasRows, hidden := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for rel, body := range bodies {
		text := inline.ReplaceAllString(fenced.ReplaceAllString(body, ""), "")
		for _, match := range markdown.FindAllStringSubmatch(text, -1) {
			if hiddenAgent.MatchString(strings.ReplaceAll(match[1], `\`, "/")) {
				hidden[rel+" -> "+match[1]] = true
			}
		}
		for _, match := range wiki.FindAllStringSubmatch(text, -1) {
			target := strings.TrimSpace(match[1])
			if _, ok := notes[target]; ok {
				inbound[target] = true
			} else if bases[target] {
			} else if canonical, ok := aliases[target]; ok {
				aliasRows[rel+" -> alias "+target+" (use "+canonical+")"] = true
			} else {
				broken[rel+" -> "+target] = true
			}
		}
	}
	for stem, path := range notes {
		if inbound[stem] {
			continue
		}
		if stem == "00-Home" || stem == "README" {
			r.ExpectedOrphans = append(r.ExpectedOrphans, path)
		} else {
			r.Orphans = append(r.Orphans, stem)
			r.OrphanPaths = append(r.OrphanPaths, path)
		}
	}
	for stem, paths := range duplicates {
		sort.Strings(paths)
		r.Duplicates = append(r.Duplicates, fmt.Sprintf("%s: %s", stem, strings.Join(paths, ", ")))
	}
	for row := range broken {
		r.Broken = append(r.Broken, row)
	}
	for row := range aliasRows {
		r.AliasTargets = append(r.AliasTargets, row)
	}
	for row := range hidden {
		r.Hidden = append(r.Hidden, row)
	}
	for _, v := range [][]string{r.Broken, r.AliasTargets, r.Hidden, r.Duplicates, r.Orphans, r.OrphanPaths, r.ExpectedOrphans} {
		sort.Strings(v)
	}
	return r, nil
}
