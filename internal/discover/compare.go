package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Comparison of discovered facts with the vault's repository notes. Only deterministic
// relations are judged here (topics, subscriptions and event types); everything else is
// left to the documented review.

var edgeFields = []string{"gatillado-por", "publica-en", "consume-de", "lee-de", "escribe-en", "usa-infra"}
var wikilink = regexp.MustCompile(`\[\[([^\]|#]+)`)
var fmLine = regexp.MustCompile(`(?m)^([\w-]+):\s*(.*)$`)

type noteEdge struct {
	Field, Target, Folder, Type string
	Names                       []string
}

type comparison struct {
	Repo          string        `json:"repo"`
	Note          string        `json:"note"`
	Supported     []string      `json:"supported"`
	Discrepancies []discrepancy `json:"discrepancies"`
	Undocumented  []string      `json:"undocumented"`
}

type discrepancy struct {
	Field  string   `json:"field"`
	Target string   `json:"target"`
	Found  []string `json:"discovered"`
	Detail string   `json:"detail"`
}

type vaultNote struct {
	path, folder string
	fm           map[string]string
}

func frontmatterOf(b []byte) map[string]string {
	t := string(b)
	out := map[string]string{}
	if !strings.HasPrefix(t, "---") {
		return out
	}
	end := strings.Index(t[3:], "\n---")
	if end < 0 {
		return out
	}
	for _, m := range fmLine.FindAllStringSubmatch(t[3:3+end], -1) {
		out[m[1]] = strings.TrimSpace(m[2])
	}
	return out
}

func loadNotes(vault string) (map[string]vaultNote, error) {
	notes := map[string]vaultNote{}
	e := filepath.WalkDir(vault, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		rel, _ := filepath.Rel(vault, p)
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || !regexp.MustCompile(`^[1-7]\d-`).MatchString(strings.SplitN(filepath.ToSlash(rel), "/", 2)[0])) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		stem := strings.TrimSuffix(filepath.Base(p), ".md")
		notes[stem] = vaultNote{path: filepath.ToSlash(rel), folder: strings.SplitN(filepath.ToSlash(rel), "/", 2)[0], fm: frontmatterOf(b)}
		return nil
	})
	return notes, e
}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range regexp.MustCompile(`[^a-z0-9]+`).Split(strings.ToLower(s), -1) {
		if t != "" {
			out[t] = true
		}
	}
	return out
}

func subset(a, b map[string]bool) bool {
	if len(a) == 0 {
		return false
	}
	for t := range a {
		if !b[t] {
			return false
		}
	}
	return true
}

// nameMatch accepts per-country or per-environment variants of a documented logical name.
func nameMatch(doc, found string) bool {
	d, f := tokens(doc), tokens(normalizeResource(found))
	// The documented logical name is contained in the physical one (country, environment or
	// -sub suffixes), or differs from it by at most one token.
	return subset(d, f) || subset(f, d) && len(f) >= 2 && len(d)-len(f) <= 1
}

// names are the note's stem, raw name and aliases; alternatives in braces (topic-{cl|co|pe}) expand to
// each variant, so a per-country resource matches the logical name the note documents.
func (n vaultNote) names(stem string) []string {
	out := []string{stem}
	add := func(name string) {
		for _, v := range expandBraces(name) {
			if len(strings.Trim(v, "-_. ")) >= 6 { // a variant as short as "svc-" would match any resource
				out = append(out, v)
			}
		}
	}
	if raw := strings.Trim(n.fm["nombre-raw"], `"'`); raw != "" {
		add(raw)
	}
	if m := regexp.MustCompile(`\[(.*)\]`).FindStringSubmatch(n.fm["aliases"]); m != nil {
		for _, a := range strings.Split(m[1], ",") {
			if a = strings.Trim(strings.TrimSpace(a), `"'`); a != "" {
				add(a)
			}
		}
	}
	return out
}

func compareNotes(vault string, facts []repoFacts) ([]comparison, error) {
	notes, e := loadNotes(vault)
	if e != nil {
		return nil, e
	}
	out := []comparison{}
	for _, f := range facts {
		if f.Note == "" {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(f.Note), ".md")
		n, ok := notes[stem]
		if !ok {
			continue
		}
		c := comparison{Repo: f.Repo, Note: f.Note, Supported: []string{}, Discrepancies: []discrepancy{}, Undocumented: []string{}}
		found := []string{}
		for _, r := range f.Resources {
			if isMessaging(r.Type) {
				found = append(found, r.Name)
				if r.Topic != "" {
					found = append(found, r.Topic)
				}
				found = append(found, r.Events...)
			}
		}
		for _, ev := range f.Events {
			found = append(found, ev.Name)
		}
		documented := []string{}
		for _, field := range edgeFields {
			for _, m := range wikilink.FindAllStringSubmatch(n.fm[field], -1) {
				target := strings.TrimSpace(m[1])
				tn, ok := notes[target]
				if !ok || tn.folder != "25-Topics" {
					continue
				}
				names := tn.names(target)
				documented = append(documented, names...)
				hit := ""
				for _, x := range found {
					for _, d := range names {
						if nameMatch(d, x) {
							hit = x
						}
					}
				}
				if hit != "" {
					c.Supported = appendUnique(c.Supported, field+": "+target)
					continue
				}
				c.Discrepancies = append(c.Discrepancies, discrepancy{Field: field, Target: target, Found: uniqueSorted(found, 8),
					Detail: "the note declares this relation but no configuration, IaC, code literal or platform wiring of this repository names it"})
			}
		}
		groups := map[string][]string{}
		for _, r := range f.Resources {
			if len(f.Languages) == 0 {
				break // infrastructure-only repositories declare many resources; gaps are judged cell-wide
			}
			if !isMessaging(r.Type) {
				continue
			}
			matched := false
			for _, d := range documented {
				matched = matched || nameMatch(d, r.Name) || (r.Topic != "" && nameMatch(d, r.Topic))
			}
			if !matched {
				k := r.Type + ": " + logicalName(normalizeResource(r.Name))
				groups[k] = appendUnique(groups[k], normalizeResource(r.Name))
			}
		}
		for k, names := range groups {
			sort.Strings(names)
			label := k + " (" + names[0]
			if len(names) > 1 {
				label += fmt.Sprintf(" +%d variants", len(names)-1)
			}
			c.Undocumented = append(c.Undocumented, label+")")
		}
		sort.Strings(c.Undocumented)
		out = append(out, c)
	}
	return out, nil
}

// logicalName groups physical variants: short tokens (country, environment, -sub) are the
// usual variation points, so they are dropped from the grouping key.
func logicalName(n string) string {
	keep := []string{}
	for _, t := range regexp.MustCompile(`[^a-z0-9]+`).Split(strings.ToLower(n), -1) {
		if len(t) > 3 {
			keep = append(keep, t)
		}
	}
	if len(keep) == 0 {
		return n
	}
	return strings.Join(keep, "-")
}

// cellGaps lists logical messaging resources found anywhere in the cell that no topic note names.
func cellGaps(vault string, facts []repoFacts) ([]map[string]any, error) {
	notes, e := loadNotes(vault)
	if e != nil {
		return nil, e
	}
	names := []string{}
	for stem, n := range notes {
		if n.folder == "25-Topics" {
			names = append(names, n.names(stem)...)
		}
	}
	type group struct {
		typ      string
		variants map[string]bool
		repos    map[string]bool
	}
	groups := map[string]*group{}
	for _, f := range facts {
		for _, r := range f.Resources {
			if !isMessaging(r.Type) {
				continue
			}
			covered := false
			for _, d := range names {
				covered = covered || nameMatch(d, r.Name) || r.Topic != "" && nameMatch(d, r.Topic)
			}
			if covered {
				continue
			}
			k := r.Type + ": " + logicalName(normalizeResource(r.Name))
			g := groups[k]
			if g == nil {
				g = &group{typ: r.Type, variants: map[string]bool{}, repos: map[string]bool{}}
				groups[k] = g
			}
			g.variants[normalizeResource(r.Name)] = true
			g.repos[f.Repo] = true
		}
	}
	out := []map[string]any{}
	for k, g := range groups {
		out = append(out, map[string]any{"logical": k, "variants": firstN(g.variants, 20), "repositories": firstN(g.repos, 20)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["logical"].(string) < out[j]["logical"].(string) })
	return out, nil
}

func uniqueSorted(xs []string, n int) []string {
	m := map[string]bool{}
	for _, x := range xs {
		m[normalizeResource(x)] = true
	}
	return firstN(m, n)
}
