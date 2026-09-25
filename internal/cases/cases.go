// Package cases keeps investigation cases as plain Markdown: one case file per investigation, edited
// directly, with Git as the history once published. It offers three operations: new (create a case
// from the template of its type), list (find unpublished, published and retired cases) and check (the
// deterministic gate against unsourced evidence, broken references, vault duplication and leaks).
package cases

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"documentation-vault/internal/config"
)

const Help = `investigation COMMAND --vault PATH [options]
  new     --title TEXT --type understanding|development [--source-ref REF]
          Create an unpublished case (.investigations/<id>/investigation.md) from the template
          of its type; reports existing cases with a similar title.
  list    [--id ID]   Unpublished, published and retired cases (legacy cases included).
  check   [--id ID]   Gate a case: every evidence record cites its source, record IDs are unique
          and resolve, links resolve, no credential or local path, no paragraph copied from a
          vault note (reference it instead), relations discovery contradicts go to review.
Edit the case file directly. Publish, absorb and retire on a sync branch (synchronize-ecosystem):
move the case to investigations/, edit notes, and sync verify runs check. All output is JSON.`

var (
	idPattern  = regexp.MustCompile(`^\d{8}-\d{6}-[a-z0-9]+(?:-[a-z0-9]+)*(?:-\d{2})?$`)
	slugWord   = regexp.MustCompile(`[a-z0-9]+`)
	fmLine     = regexp.MustCompile(`^([a-z][a-z0-9-]*):\s*(.*)$`)
	retiredTag = regexp.MustCompile(`(?m)^Retired-Case:\s*(\S+)`)
)

type options struct {
	vault, title, kind, sourceRef, id string
}

// Run executes an investigation command. Legacy verbs are handled by the caller.
func Run(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, e := io.WriteString(out, Help+"\n")
		return e
	}
	verb := args[0]
	var o options
	for i := 1; i < len(args); i++ {
		if i+1 >= len(args) {
			return fmt.Errorf("%s requires a value", args[i])
		}
		v := args[i+1]
		switch args[i] {
		case "--vault":
			o.vault = v
		case "--title":
			o.title = v
		case "--type":
			o.kind = v
		case "--source-ref":
			o.sourceRef = v
		case "--id":
			o.id = v
		default:
			return fmt.Errorf("unknown option %s", args[i])
		}
		i++
	}
	if o.vault == "" {
		return errors.New("--vault is required")
	}
	r, e := config.Resolve(o.vault)
	if e != nil {
		return e
	}
	o.vault, _ = r["vault_root"].(string)
	switch verb {
	case "new":
		return create(o, out)
	case "list":
		cs, e := List(o.vault)
		if e != nil {
			return e
		}
		if o.id != "" {
			for _, c := range cs {
				if c.ID == o.id {
					return emit(out, c)
				}
			}
			return fmt.Errorf("no case %s", o.id)
		}
		return emit(out, map[string]any{"cases": cs, "count": len(cs)})
	case "check":
		return runCheck(o, out)
	}
	return fmt.Errorf("unknown investigation command %q", verb)
}

func emit(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}

// Case is one investigation as found on disk or in Git history.
type Case struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Status     string `json:"status"` // open | closed | retired
	Outcome    string `json:"outcome,omitempty"`
	Visibility string `json:"visibility"` // unpublished | published | retired
	Path       string `json:"path,omitempty"`
	Private    string `json:"private_overlay,omitempty"`
	Local      string `json:"local_store,omitempty"`
	Legacy     bool   `json:"legacy,omitempty"`
	Updated    string `json:"updated,omitempty"`
}

func frontmatter(text string) map[string]string {
	out := map[string]string{}
	if !strings.HasPrefix(text, "---\n") {
		return out
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return out
	}
	for _, l := range strings.Split(text[4:4+end], "\n") {
		if m := fmLine.FindStringSubmatch(l); m != nil {
			out[m[1]] = strings.Trim(strings.TrimSpace(m[2]), `"'`)
		}
	}
	return out
}

// normalize maps the current and legacy schemas onto type, status and outcome.
func normalize(fm map[string]string) (kind, status, outcome string, legacy bool) {
	kind, legacy = fm["type"], fm["type"] == ""
	if legacy {
		switch fm["purpose"] {
		case "development", "mixed":
			kind = "development"
		default:
			kind = "understanding"
		}
	}
	switch fm["status"] {
	case "closed":
		status = "closed"
	default:
		status = "open"
	}
	outcome = fm["outcome"]
	if outcome == "" {
		outcome = fm["closure-outcome"]
	}
	return kind, status, outcome, legacy
}

func readCase(vault, dir, visibility string) (Case, bool) {
	p := filepath.Join(dir, "investigation.md")
	b, e := os.ReadFile(p)
	if e != nil {
		return Case{}, false
	}
	fm := frontmatter(string(b))
	c := Case{ID: fm["id"], Title: fm["title"], Visibility: visibility}
	if c.ID == "" {
		c.ID = filepath.Base(dir)
	}
	c.Type, c.Status, c.Outcome, c.Legacy = normalize(fm)
	c.Path, _ = filepath.Rel(vault, p)
	if st, e := os.Stat(p); e == nil {
		c.Updated = st.ModTime().UTC().Format(time.RFC3339)
	}
	priv := filepath.Join(vault, ".investigations-private", c.ID)
	if _, e := os.Stat(filepath.Join(priv, "private.md")); e == nil {
		c.Private, _ = filepath.Rel(vault, filepath.Join(priv, "private.md"))
	}
	if _, e := os.Stat(filepath.Join(priv, "local")); e == nil {
		c.Local, _ = filepath.Rel(vault, filepath.Join(priv, "local"))
	}
	return c, true
}

// List returns every case: unpublished, published, and retired ones recorded in Git history
// (Retired-Case trailers) or in the legacy retirement register.
func List(vault string) ([]Case, error) {
	out := []Case{}
	seen := map[string]bool{}
	for _, store := range []struct{ dir, visibility string }{{".investigations", "unpublished"}, {"investigations", "published"}} {
		entries, _ := os.ReadDir(filepath.Join(vault, store.dir))
		for _, d := range entries {
			if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				continue
			}
			if c, ok := readCase(vault, filepath.Join(vault, store.dir, d.Name()), store.visibility); ok {
				out = append(out, c)
				seen[c.ID] = true
			}
		}
	}
	for _, id := range retiredIDs(vault) {
		if !seen[id] {
			out = append(out, Case{ID: id, Status: "retired", Visibility: "retired"})
			seen[id] = true
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func retiredIDs(vault string) []string {
	ids := []string{}
	cmd := exec.Command("git", "-C", vault, "log", "--format=%B", "--grep=^Retired-Case:")
	if b, e := cmd.Output(); e == nil {
		for _, m := range retiredTag.FindAllStringSubmatch(string(b), -1) {
			ids = append(ids, m[1])
		}
	}
	// Legacy register: "- {json}" lines with an "id" field.
	if b, e := os.ReadFile(filepath.Join(vault, "investigations", "retired.md")); e == nil {
		for _, l := range strings.Split(string(b), "\n") {
			var m map[string]any
			if strings.HasPrefix(l, "- ") && json.Unmarshal([]byte(l[2:]), &m) == nil {
				if id, _ := m["id"].(string); id != "" {
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

func slug(title string) string {
	words := slugWord.FindAllString(strings.ToLower(asciiFold(title)), -1)
	if len(words) > 6 {
		words = words[:6]
	}
	if len(words) == 0 {
		return "case"
	}
	return strings.Join(words, "-")
}

func asciiFold(s string) string {
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n", "Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ñ", "n")
	return r.Replace(s)
}

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range slugWord.FindAllString(strings.ToLower(asciiFold(s)), -1) {
		if len(w) > 3 {
			out[w] = true
		}
	}
	return out
}

func create(o options, out io.Writer) error {
	if strings.TrimSpace(o.title) == "" {
		return errors.New("--title is required")
	}
	if o.kind != "understanding" && o.kind != "development" {
		return errors.New("--type must be understanding (explain, reconstruct, diagnose) or development (requirements and changes to build)")
	}
	existing, _ := List(o.vault)
	similar := []map[string]string{}
	mine := words(o.title)
	for _, c := range existing {
		theirs := words(c.Title)
		common := 0
		for w := range mine {
			if theirs[w] {
				common++
			}
		}
		if len(mine) > 0 && common*2 >= len(mine) {
			similar = append(similar, map[string]string{"id": c.ID, "title": c.Title, "status": c.Status, "visibility": c.Visibility})
		}
	}
	now := time.Now()
	id := now.Format("20060102-150405") + "-" + slug(o.title)
	for n := 2; ; n++ {
		if _, e := os.Stat(filepath.Join(o.vault, ".investigations", id)); os.IsNotExist(e) {
			break
		}
		id = fmt.Sprintf("%s-%s-%02d", now.Format("20060102-150405"), slug(o.title), n)
	}
	locale := "es"
	if inst, e := config.LoadInstance(o.vault); e == nil {
		if l, _ := inst["locale"].(map[string]any); l != nil {
			if v, _ := l["notes"].(string); v == "en" {
				locale = "en"
			}
		}
	}
	dir := filepath.Join(o.vault, ".investigations", id)
	if e := os.MkdirAll(dir, 0o755); e != nil {
		return e
	}
	text := template(locale, o.kind, id, o.title, o.sourceRef, now.Format("2006-01-02"))
	if e := os.WriteFile(filepath.Join(dir, "investigation.md"), []byte(text), 0o644); e != nil {
		return e
	}
	rel, _ := filepath.Rel(o.vault, filepath.Join(dir, "investigation.md"))
	return emit(out, map[string]any{"id": id, "path": rel, "type": o.kind, "similar_cases": similar,
		"next": "edit the case file directly; run `investigation check --id " + id + "` after each update"})
}
