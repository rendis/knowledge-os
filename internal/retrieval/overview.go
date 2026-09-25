package retrieval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Overview renders one line per knowledge note (10–70 folders): name, type, system, direct
// relations and the first sentence of its body. It is computed from the Markdown on every call,
// so it is never stale, and lets the agent pick the notes to open before searching.

var (
	ovFolder   = regexp.MustCompile(`^[1-7]\d-`)
	ovField    = regexp.MustCompile(`(?m)^([\w-]+):\s*(.*)$`)
	ovLink     = regexp.MustCompile(`\[\[([^\]|#]+)`)
	ovSentence = regexp.MustCompile(`^(.{20,220}?[.!?])(\s|$)`)
	ovNoise    = regexp.MustCompile("\\[\\^[^\\]]+\\]|\\[\\[([^\\]|#]+)(?:\\|[^\\]]*)?\\]\\]|`")
)

var ovEdges = []string{"sistema", "topico", "gatillado-por", "publica-en", "consume-de", "lee-de", "escribe-en", "usa-infra", "participa-en", "compuesto-por", "implementado-por"}

type ovNote struct {
	folder, stem, line string
}

func overviewLine(stem string, raw []byte) string {
	t := string(raw)
	fm, body := "", t
	if strings.HasPrefix(t, "---") {
		if end := strings.Index(t[3:], "\n---"); end >= 0 {
			fm, body = t[3:3+end], t[3+end+4:]
		}
	}
	fields := map[string]string{}
	for _, m := range ovField.FindAllStringSubmatch(fm, -1) {
		fields[m[1]] = strings.TrimSpace(m[2])
	}
	parts := []string{"[[" + stem + "]]"}
	if tipo := strings.Trim(fields["tipo"], `"'`); tipo != "" {
		parts = append(parts, "("+tipo+")")
	}
	rels := []string{}
	for _, k := range ovEdges {
		targets := []string{}
		for _, m := range ovLink.FindAllStringSubmatch(fields[k], -1) {
			targets = append(targets, strings.TrimSpace(m[1]))
		}
		if len(targets) > 0 {
			if len(targets) > 6 {
				targets = append(targets[:6], fmt.Sprintf("+%d", len(targets)-6))
			}
			rels = append(rels, k+": "+strings.Join(targets, ", "))
		}
	}
	summary := ""
	for _, para := range strings.Split(body, "\n\n") {
		p := strings.TrimSpace(para)
		if p == "" || strings.HasPrefix(p, "#") || strings.HasPrefix(p, "|") || strings.HasPrefix(p, "```") || strings.HasPrefix(p, "[^") || strings.HasPrefix(p, ">") || strings.HasPrefix(p, "-") {
			continue
		}
		p = ovNoise.ReplaceAllString(strings.Join(strings.Fields(p), " "), "$1")
		if m := ovSentence.FindStringSubmatch(p); m != nil {
			summary = m[1]
		} else if r := []rune(p); len(r) > 220 {
			summary = string(r[:220]) + "…"
		} else {
			summary = p
		}
		break
	}
	line := strings.Join(parts, " ")
	if summary != "" {
		line += " — " + summary
	}
	if len(rels) > 0 {
		line += " {" + strings.Join(rels, "; ") + "}"
	}
	return line
}

// discoveryFlags reads the last `discover run` comparison (local state) and returns, per note path,
// the relations the repository evidence does not support, so readers verify them before use.
func discoveryFlags(root string) map[string]string {
	var cmp []struct {
		Note          string `json:"note"`
		Discrepancies []struct {
			Field  string   `json:"field"`
			Target string   `json:"target"`
			Found  []string `json:"discovered"`
		} `json:"discrepancies"`
	}
	b, e := readFileBounded(filepath.Join(root, ".agents", "state", "discovery", "comparison.json"), 16_000_000)
	if e != nil || json.Unmarshal(b, &cmp) != nil {
		return nil
	}
	flags := map[string]string{}
	for _, c := range cmp {
		parts := []string{}
		for _, d := range c.Discrepancies {
			p := d.Field + ": " + d.Target
			if len(d.Found) > 0 {
				found := d.Found
				if len(found) > 4 {
					found = found[:4]
				}
				p += " (repository evidence names " + strings.Join(found, ", ") + ")"
			}
			parts = append(parts, p)
		}
		if len(parts) > 0 {
			flags[filepath.ToSlash(c.Note)] = " ⚠ unsupported by the last discover run, verify before use: " + strings.Join(parts, "; ")
		}
	}
	return flags
}

// WriteOverview prints the vault overview as compact Markdown.
func WriteOverview(root, folder string, out io.Writer) error {
	notes := []ovNote{}
	flags := discoveryFlags(root)
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		top := strings.SplitN(rel, "/", 2)[0]
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || !ovFolder.MatchString(top)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") || folder != "" && top != folder {
			return nil
		}
		raw, e := readFileBounded(p, 512_000)
		if e != nil {
			return nil
		}
		stem := strings.TrimSuffix(filepath.Base(p), ".md")
		notes = append(notes, ovNote{folder: top, stem: stem, line: overviewLine(stem, raw) + flags[rel]})
		return nil
	})
	if e != nil {
		return e
	}
	sort.Slice(notes, func(i, j int) bool {
		if notes[i].folder != notes[j].folder {
			return notes[i].folder < notes[j].folder
		}
		return strings.ToLower(notes[i].stem) < strings.ToLower(notes[j].stem)
	})
	current := ""
	fmt.Fprintf(out, "# Vault overview (%d notes; open a note by its basename)\n", len(notes))
	for _, n := range notes {
		if n.folder != current {
			current = n.folder
			fmt.Fprintf(out, "\n## %s\n", current)
		}
		fmt.Fprintln(out, "- "+n.line)
	}
	return nil
}

func readFileBounded(p string, limit int64) ([]byte, error) {
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return readBounded(f, limit)
}
