package discover

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Two deterministic defenses against wrong technical claims, using only the vault's own evidence:
// corrections turns every relation discovery could not support into a correction task (fix the note,
// not the reader), and claims checks a draft answer before delivery for resource names no evidence
// knows and for relations discovery contradicts.

type correction struct {
	Note     string   `json:"note"`
	Repo     string   `json:"repo"`
	Relation string   `json:"relation"`
	Target   string   `json:"target"`
	Evidence []string `json:"repository_evidence_names"`
	Action   string   `json:"action"`
}

func loadComparison(vault string) ([]comparison, error) {
	var cmp []comparison
	if e := readState(vault, "comparison.json", &cmp); e != nil {
		return nil, errors.New("no discovery comparison: run `discover run` first")
	}
	return cmp, nil
}

func listCorrections(o options, out io.Writer) error {
	cmp, e := loadComparison(o.vault)
	if e != nil {
		return e
	}
	tasks := []correction{}
	undocumented := 0
	for _, c := range cmp {
		undocumented += len(c.Undocumented)
		for _, d := range c.Discrepancies {
			tasks = append(tasks, correction{Note: c.Note, Repo: c.Repo, Relation: d.Field, Target: d.Target, Evidence: d.Found,
				Action: "verify the relation at the source: retract or correct it, or cite the platform wiring, IaC or repository that supports it; publish through sync (gates and review)"})
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Note+tasks[i].Target < tasks[j].Note+tasks[j].Target })
	return emit(out, map[string]any{"corrections": tasks, "count": len(tasks), "undocumented_resources": undocumented,
		"next": "correct each note on a sync/ branch (synchronize-ecosystem), one branch per repository or a small group"})
}

var (
	claimTick   = regexp.MustCompile("`([^`\n]{3,200})`")
	claimLink   = regexp.MustCompile(`\[\[([^\]|#]+)`)
	resourceTok = regexp.MustCompile(`^[a-z0-9][a-z0-9._\-/:{}|]*$`)
	fileExt     = regexp.MustCompile(`\.[a-z0-9]{1,5}$`)
	braceAlt    = regexp.MustCompile(`\{([^{}]+)\}`)
	nombreRaw   = regexp.MustCompile(`(?m)^nombre-raw:\s*"?([^"\n]+)"?`)
	lineRange   = regexp.MustCompile(`^[0-9]+([-:][0-9]+)*$`)
)

// expandBraces turns "topic-{cl|co|pe}" into its variants.
func expandBraces(s string) []string {
	m := braceAlt.FindStringSubmatchIndex(s)
	if m == nil {
		return []string{s}
	}
	out := []string{}
	for _, alt := range strings.Split(s[m[2]:m[3]], "|") {
		out = append(out, expandBraces(s[:m[0]]+strings.TrimSpace(alt)+s[m[1]:])...)
	}
	return out
}

// knownNames collects every name the vault's evidence knows: facts, platform snapshots and notes.
func knownNames(vault string) (map[string]bool, error) {
	known := map[string]bool{}
	add := func(names ...string) {
		for _, n := range names {
			for _, v := range expandBraces(strings.TrimSpace(n)) {
				if v != "" {
					known[strings.ToLower(v)] = true
					known[strings.ToLower(normalizeResource(v))] = true
				}
			}
		}
	}
	for _, opts := range []map[string]string{dependencyOptions, resourceOptions, entryOptions} {
		for k := range opts {
			add(k) // discovery's own vocabulary (categories, resource types)
		}
	}
	files, _ := filepath.Glob(filepath.Join(vault, stateRel, "facts", "*.json"))
	if len(files) == 0 {
		return nil, errors.New("no discovery facts: run `discover run` first")
	}
	for _, f := range files {
		var rf repoFacts
		if readState(vault, filepath.Join("facts", filepath.Base(f)), &rf) != nil {
			continue
		}
		add(rf.Repo)
		add(rf.ServiceIDs...)
		for _, r := range rf.Resources {
			add(r.Name, r.Topic)
			add(r.Events...)
			for _, ev := range r.Evidence {
				_, scope, _ := strings.Cut(ev.Scope, ":")
				add(ev.Value, ev.Key, scope, filepath.Base(ev.File))
			}
		}
		for _, ev := range rf.Events {
			add(ev.Name, ev.Topic)
		}
	}
	snaps, _ := loadSnapshots(vault)
	for _, s := range snaps {
		add(s.Scope)
		add(s.Topics...)
		for _, sub := range s.Subscriptions {
			add(sub.Name, sub.Topic, sub.DeadLetter)
			for _, kv := range sub.Attributes {
				add(kv[1])
			}
		}
	}
	// Anything the vault itself names (notes, conventions, skills: quoted, linked or frontmatter) is known;
	// wrong relations among known names are the relation check's job, not this one.
	e := filepath.WalkDir(vault, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(p, vault))
		if d.IsDir() {
			if d.Name() == ".git" || strings.HasPrefix(rel, "/.agents/state") || strings.HasPrefix(d.Name(), ".") && d.Name() != ".agents" && p != vault {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		add(strings.TrimSuffix(d.Name(), ".md"))
		b, _ := os.ReadFile(p)
		fm := frontmatterOf(b)
		for k, v := range fm {
			add(k)
			for _, m := range claimLink.FindAllStringSubmatch(v, -1) {
				add(m[1])
			}
		}
		if m := aliasList.FindStringSubmatch(fm["aliases"]); m != nil {
			for _, a := range strings.Split(m[1], ",") {
				add(strings.Trim(strings.TrimSpace(a), `"'`))
			}
		}
		if m := nombreRaw.FindStringSubmatch(string(b)); m != nil {
			add(m[1])
		}
		for _, m := range claimTick.FindAllStringSubmatch(string(b), -1) {
			add(m[1])
		}
		for _, m := range claimLink.FindAllStringSubmatch(string(b), -1) {
			add(m[1])
		}
		return nil
	})
	return known, e
}

// resourceCandidate reports whether a quoted token looks like a resource name (topic, subscription,
// event carrier, repository, bucket) rather than code, a file path, a key or a version.
func resourceCandidate(tok string) bool {
	switch {
	case len(tok) < 6 || !resourceShaped(tok) || !resourceTok.MatchString(tok):
		return false
	case strings.Contains(tok, "://") || strings.Contains(tok, "...") || hexRef.MatchString(strings.TrimRight(tok, ".…")) || versionRef.MatchString(tok) || lineRange.MatchString(tok):
		return false
	case strings.Contains(tok, "/") && !strings.HasPrefix(tok, "projects/"):
		return false // repository paths are checked by note gates, not here
	case fileExt.MatchString(tok) && !strings.Contains(tok, "-"):
		return false
	}
	return true
}

func isKnown(known map[string]bool, name string) bool {
	n := strings.ToLower(name)
	if known[n] || known[strings.ToLower(normalizeResource(name))] {
		return true
	}
	for _, v := range expandBraces(n) {
		if known[v] {
			return true
		}
	}
	for k := range known {
		if len(k) >= 6 && (nameMatch(k, n) || nameMatch(n, k)) {
			return true
		}
	}
	return false
}

type claimFlag struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

func checkClaims(vault, text string) (map[string]any, error) {
	known, e := knownNames(vault)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	unknown := []claimFlag{}
	checked := 0
	candidates := []string{}
	for _, m := range claimTick.FindAllStringSubmatch(text, -1) {
		candidates = append(candidates, strings.TrimSpace(m[1]))
	}
	for _, m := range claimLink.FindAllStringSubmatch(text, -1) {
		candidates = append(candidates, strings.TrimSpace(m[1]))
	}
	for _, c := range candidates {
		if seen[c] || !resourceCandidate(c) {
			continue
		}
		seen[c] = true
		checked++
		if !isKnown(known, c) {
			unknown = append(unknown, claimFlag{c, "no discovery fact, platform snapshot or note names it; verify it at the source or remove it"})
		}
	}
	contradicted := []claimFlag{}
	if cmp, e := loadComparison(vault); e == nil {
		lower := strings.ToLower(text)
		for _, c := range cmp {
			stem := strings.ToLower(strings.TrimSuffix(filepath.Base(c.Note), ".md"))
			for _, d := range c.Discrepancies {
				target := strings.ToLower(d.Target)
				if strings.Contains(lower, target) && (strings.Contains(lower, stem) || strings.Contains(lower, strings.ToLower(c.Repo))) {
					contradicted = append(contradicted, claimFlag{stem + " " + d.Field + " " + d.Target,
						fmt.Sprintf("the note declares this relation but the repository's evidence names %s; state it as unverified or confirm it at the source", strings.Join(firstList(d.Found, 4), ", "))})
				}
			}
		}
	}
	return map[string]any{"ok": len(unknown) == 0 && len(contradicted) == 0, "names_checked": checked, "unknown_names": unknown, "contradicted_relations": contradicted}, nil
}

func runClaims(o options, out io.Writer) error {
	if o.file == "" {
		return errors.New("--file is required (a draft answer, or - for standard input)")
	}
	var b []byte
	var e error
	if o.file == "-" {
		b, e = io.ReadAll(io.LimitReader(os.Stdin, 4_000_000))
	} else {
		b, e = os.ReadFile(o.file)
	}
	if e != nil {
		return e
	}
	r, e := checkClaims(o.vault, string(b))
	if e != nil {
		return e
	}
	return emit(out, r)
}

// ContradictedRelations returns the note relations discovery could not support that a text mentions,
// as name/detail pairs. It is empty when no discovery comparison exists yet.
func ContradictedRelations(vault, text string) [][2]string {
	r, e := checkClaims(vault, text)
	if e != nil {
		return nil
	}
	out := [][2]string{}
	if flags, ok := r["contradicted_relations"].([]claimFlag); ok {
		for _, f := range flags {
			out = append(out, [2]string{f.Name, f.Detail})
		}
	}
	return out
}
