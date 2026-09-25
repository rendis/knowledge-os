package cases

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Sections of the earlier case format, by lowercased title, mapped onto the current sections. Titles
// the current format uses map through sectionKey as well.
var legacySections = map[string]string{
	"resumen de la solicitud": "objective", "request summary": "objective", "solicitud": "objective",
	"estado vigente": "state", "estado actual": "state", "current state": "state",
	"evidencia": "evidence", "evidence": "evidence",
	"superficies afectadas": "changes", "affected surfaces": "changes",
	"handoffs de desarrollo": "handoffs", "development handoffs": "handoffs",
	"historial": "log", "history": "log",
}

// Subsections that move to another section: scope lived under the state, inferences under evidence.
var legacySubsections = map[string]string{
	"objetivo": "objective", "objective": "objective", "alcance": "objective", "scope": "objective",
	"fuera de alcance": "objective", "out of scope": "objective",
	"inferencias": "findings", "inferences": "findings",
}

type h2 struct {
	title string
	lines []string
}

func splitH2(body string) ([]string, []h2) {
	pre, out := []string{}, []h2{}
	inFence := false
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "```") {
			inFence = !inFence
		}
		if m := heading2Line.FindStringSubmatch(l); m != nil && !inFence {
			out = append(out, h2{title: m[1]})
			continue
		}
		if len(out) == 0 {
			pre = append(pre, l)
		} else {
			out[len(out)-1].lines = append(out[len(out)-1].lines, l)
		}
	}
	return pre, out
}

// splitH3 separates the lines before the first ### heading from the ### blocks.
func splitH3(lines []string) ([]string, [][]string) {
	pre, blocks := []string{}, [][]string{}
	for _, l := range lines {
		if strings.HasPrefix(l, "### ") {
			blocks = append(blocks, []string{l})
			continue
		}
		if len(blocks) == 0 {
			pre = append(pre, l)
		} else {
			blocks[len(blocks)-1] = append(blocks[len(blocks)-1], l)
		}
	}
	return pre, blocks
}

func trimBlank(lines []string) string { return strings.Trim(strings.Join(lines, "\n"), "\n ") }

// convertLegacy rewrites a case of the earlier format into the current structure. Content is kept;
// only frontmatter and section layout change.
func convertLegacy(raw, locale, date string) (string, map[string]any) {
	fm := frontmatter(raw)
	kind, status, outcome, _ := normalize(fm)
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		if end := strings.Index(raw[4:], "\n---"); end >= 0 {
			body = strings.TrimPrefix(raw[4+end+4:], "\n")
		}
	}
	if fm["purpose"] == "mixed" {
		kind = "understanding"
		for id := range definedRecords(body) {
			if strings.HasPrefix(id, "DH-") {
				kind = "development"
			}
		}
	}
	pre, sections := splitH2(body)
	content := map[string][]string{}
	extras := []h2{}
	mapped := map[string]string{}
	for _, s := range sections {
		t := strings.ToLower(strings.TrimSpace(s.title))
		key := legacySections[t]
		if key == "" {
			key = sectionKey(s.title)
		}
		if key == "changes" && kind != "development" {
			key = ""
		}
		if key == "" {
			extras = append(extras, s)
			mapped[s.title] = "kept"
			continue
		}
		mapped[s.title] = key
		if key == "state" || key == "evidence" {
			p, blocks := splitH3(s.lines)
			content[key] = append(content[key], trimBlank(p))
			for _, b := range blocks {
				sub := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(b[0], "### ")))
				if dest := legacySubsections[sub]; dest != "" && (dest == "objective") == (key == "state") {
					content[dest] = append(content[dest], trimBlank(b))
					mapped[s.title+" / "+strings.TrimPrefix(b[0], "### ")] = dest
					continue
				}
				content[key] = append(content[key], trimBlank(b))
			}
			continue
		}
		content[key] = append(content[key], trimBlank(s.lines))
	}
	var b strings.Builder
	b.WriteString("---\nid: " + fm["id"] + "\ntitle: \"" + strings.ReplaceAll(fm["title"], `"`, `'`) + "\"\ntype: " + kind + "\nstatus: " + status + "\n")
	if status == "closed" {
		b.WriteString("outcome: " + outcome + "\n")
	}
	created := fm["created"]
	if created == "" && len(fm["created-at"]) >= 10 {
		created = fm["created-at"][:10]
	}
	if created == "" {
		created = date
	}
	b.WriteString("created: " + created + "\n")
	if fm["source-ref"] != "" {
		b.WriteString("source-ref: \"" + strings.ReplaceAll(fm["source-ref"], `"`, `'`) + "\"\n")
	}
	b.WriteString("migrated: " + date + "\n---\n\n")
	if p := trimBlank(pre); p != "" {
		b.WriteString(p + "\n")
	} else {
		b.WriteString("# " + fm["title"] + "\n")
	}
	write := func(title, text string) {
		b.WriteString("\n## " + title + "\n")
		if text != "" {
			b.WriteString("\n" + text + "\n")
		}
	}
	for _, key := range typeSections[kind] {
		if key == "log" {
			continue
		}
		parts := []string{}
		for _, p := range content[key] {
			if p != "" {
				parts = append(parts, p)
			}
		}
		if len(parts) == 0 {
			write(sectionNames[locale][key], "<!-- "+guidance[locale][key]+" -->")
			continue
		}
		write(sectionNames[locale][key], strings.Join(parts, "\n\n"))
	}
	// Mapped content the type does not template (acceptance criteria in an understanding case) is kept
	// under its own section: migration never drops content.
	inType := map[string]bool{}
	for _, key := range typeSections[kind] {
		inType[key] = true
	}
	for _, key := range []string{"objective", "state", "requirements", "evidence", "findings", "changes", "acceptance", "decisions", "questions", "handoffs", "absorption"} {
		if !inType[key] && trimBlank(content[key]) != "" {
			write(sectionNames[locale][key], strings.Join(content[key], "\n\n"))
		}
	}
	for _, s := range extras {
		write(s.title, trimBlank(s.lines))
	}
	logText := strings.Join(content["log"], "\n\n")
	if logText != "" {
		logText += "\n"
	}
	write(sectionNames[locale]["log"], logText+"- "+date+" — "+labels[locale]["migrated"])
	dropped := []string{}
	for k := range fm {
		switch k {
		case "id", "title", "type", "status", "outcome", "closure-outcome", "created", "created-at", "source-ref", "purpose":
		default:
			dropped = append(dropped, k)
		}
	}
	return b.String(), map[string]any{"type": kind, "sections": mapped, "dropped_fields": dropped}
}

func migrate(o options, out io.Writer) error {
	all, e := List(o.vault)
	if e != nil {
		return e
	}
	locale := noteLocale(o.vault)
	ix := indexVault(o.vault)
	results := []map[string]any{}
	for _, c := range all {
		if !c.Legacy || c.Path == "" || o.id != "" && c.ID != o.id {
			continue
		}
		full := filepath.Join(o.vault, c.Path)
		raw, e := os.ReadFile(full)
		if e != nil {
			return e
		}
		next, res := convertLegacy(string(raw), locale, today(o))
		r := checkContent(o.vault, full, next, ix)
		counts := map[string]int{}
		sample := []string{}
		for _, i := range r.Issues {
			counts[i.Severity]++
			if i.Severity == "error" && len(sample) < 5 {
				sample = append(sample, i.Where+": "+i.Detail)
			}
		}
		res["id"], res["path"], res["issues"], res["error_sample"] = c.ID, c.Path, counts, sample
		if o.apply {
			if c.Visibility == "published" {
				if _, e := findCase(o.vault, c.ID); e != nil {
					return e
				}
			} else {
				backup := filepath.Join(o.vault, ".investigations-private", c.ID, "local", "legacy-investigation.md")
				if e := os.MkdirAll(filepath.Dir(backup), 0o755); e != nil {
					return e
				}
				if e := os.WriteFile(backup, raw, 0o644); e != nil {
					return e
				}
				res["backup"], _ = filepath.Rel(o.vault, backup)
			}
			if e := os.WriteFile(full, []byte(next), 0o644); e != nil {
				return e
			}
		}
		results = append(results, res)
	}
	if o.id != "" && len(results) == 0 {
		return errors.New("no case " + o.id + " in the earlier format")
	}
	res := map[string]any{"applied": o.apply, "cases": results, "count": len(results)}
	if !o.apply {
		res["next"] = "review the mapping, then repeat with --apply (a published case migrates on a sync branch); errors reported are existing debt: sources to cite, links to fix"
	}
	return emit(out, res)
}
