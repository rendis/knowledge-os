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
Every change to a case goes through these commands; each one is gated and logged, and a
change that would introduce a gate error is refused. All output is JSON.
  new      --title TEXT --type understanding|development --objective TEXT [--source-ref REF]
           [--id ID --date YYYY-MM-DD]   recreate an earlier case with its identity and opening date
           Open an unpublished case from the request (neutral, formalized) and report
           existing cases with a similar title.
  list     [--id ID]   Unpublished, published and retired cases.
  check    [--id ID]   The gate: sourced evidence, unique and resolving record IDs, resolving
           links, no credential or local path, no paragraph copied from a vault note, handoff
           packages complete and citing defined requirements.
  add      --id ID --kind KIND --text TEXT [fields] [--resolves Q-NNN] [--supersedes ID]
           evidence     --source SRC --level demonstrated|observed [--limits TEXT] [--file PATH...]
           finding      --level demonstrated|inferred --from E-NNN,... | --level unresolved --missing TEXT
                        [--for-vault [[note]]] [--file PATH...]
           question     --resolve-by TEXT
           decision     --by ROLE
           requirement  --origin TEXT                       (development)
           handoff      --package handoffs/DH-NNN.md        (development)
           --file copies the file into artifacts/ named after the record, refusing credentials.
  state    --id ID --text TEXT   Rewrite the current state; it cites the records it summarizes.
  absorb   --id ID --finding F-NNN   Mark a finding for the vault as absorbed (published case,
           on the sync branch that changes its note).
  close    --id ID --outcome completed|abandoned|superseded-by:ID --reason TEXT
  reopen   --id ID --reason TEXT
A published case changes only on a sync branch; publish, absorb and retire follow
synchronize-ecosystem, and sync verify runs check. Handoff progress comes in with
handoff reconcile.`

var (
	idPattern  = regexp.MustCompile(`^\d{8}-\d{6}-[a-z0-9]+(?:-[a-z0-9]+)*(?:-\d{2})?$`)
	slugWord   = regexp.MustCompile(`[a-z0-9]+`)
	fmLine     = regexp.MustCompile(`^([a-z][a-z0-9-]*):\s*(.*)$`)
	retiredTag = regexp.MustCompile(`(?m)^Retired-Case:\s*(\S+)`)
)

type options struct {
	vault, title, kind, sourceRef, id, objective, date                 string
	recKind, text, source, level, limits, from, missing, resolveBy, by string
	origin, pkg, forVault, finding, resolves, supersedes               string
	outcome, reason, worktree                                          string
	files                                                              []string
	apply, dryRun                                                      bool
}

// Run executes an investigation command.
func Run(args []string, out io.Writer) error {
	if len(args) == 0 || contains(args, "--help") || contains(args, "-h") {
		_, e := io.WriteString(out, Help+"\n")
		return e
	}
	verb := args[0]
	var o options
	fields := map[string]*string{"--vault": &o.vault, "--title": &o.title, "--type": &o.kind, "--source-ref": &o.sourceRef,
		"--id": &o.id, "--objective": &o.objective, "--date": &o.date, "--kind": &o.recKind, "--text": &o.text,
		"--source": &o.source, "--level": &o.level, "--limits": &o.limits, "--from": &o.from, "--missing": &o.missing,
		"--resolve-by": &o.resolveBy, "--by": &o.by, "--origin": &o.origin, "--package": &o.pkg, "--for-vault": &o.forVault,
		"--finding": &o.finding, "--resolves": &o.resolves, "--supersedes": &o.supersedes, "--outcome": &o.outcome,
		"--reason": &o.reason, "--worktree": &o.worktree}
	for i := 1; i < len(args); i++ {
		if args[i] == "--apply" {
			o.apply = true
			continue
		}
		if args[i] == "--file" && i+1 < len(args) { // repeatable: one artifact made of several files
			o.files = append(o.files, args[i+1])
			i++
			continue
		}
		f, ok := fields[args[i]]
		if !ok {
			return fmt.Errorf("unknown option %s", args[i])
		}
		if i+1 >= len(args) {
			return fmt.Errorf("%s requires a value", args[i])
		}
		*f = args[i+1]
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
	case "add":
		return add(o, out)
	case "state":
		return setState(o, out)
	case "absorb":
		return absorb(o, out)
	case "close":
		return closeCase(o, out)
	case "reopen":
		return reopen(o, out)
	case "reconcile":
		return reconcile(o, out)
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
	Private    string `json:"private,omitempty"` // sensitive notes and scratch, never shared
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

// normalize reads type, status and outcome; anything but closed is open.
func normalize(fm map[string]string) (kind, status, outcome string) {
	status = "open"
	if fm["status"] == "closed" {
		status = "closed"
	}
	return fm["type"], status, fm["outcome"]
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
	c.Type, c.Status, c.Outcome = normalize(fm)
	c.Path, _ = filepath.Rel(vault, p)
	if st, e := os.Stat(p); e == nil {
		c.Updated = st.ModTime().UTC().Format(time.RFC3339)
	}
	priv := filepath.Join(vault, ".investigations-private", c.ID)
	if st, e := os.Stat(priv); e == nil && st.IsDir() {
		c.Private, _ = filepath.Rel(vault, priv)
	}
	return c, true
}

// List returns every case: unpublished, published, and retired ones recorded in Git history
// (Retired-Case trailers).
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
	if strings.TrimSpace(o.objective) == "" {
		return errors.New("--objective is required: the request formalized in neutral language (what is needed, the expected result, scope)")
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
	if o.id != "" {
		// Recreating a case keeps its identity: the ID must be well formed and free in every store.
		if !caseIDFormat.MatchString(o.id) {
			return errors.New("--id must look like 20260717-162648-short-title")
		}
		for _, c := range existing {
			if c.ID == o.id {
				return fmt.Errorf("case %s already exists (%s): move the earlier version aside first", o.id, c.Visibility)
			}
		}
		id = o.id
	}
	locale := noteLocale(o.vault)
	dir := filepath.Join(o.vault, ".investigations", id)
	if e := os.MkdirAll(dir, 0o755); e != nil {
		return e
	}
	text := template(locale, o.kind, id, o.title, o.sourceRef, today(o))
	text = replaceSection(text, "objective", locale, o.objective)
	text = appendToSection(text, "log", locale, "- "+today(o)+" — "+labels[locale]["opened"])
	if e := os.WriteFile(filepath.Join(dir, "investigation.md"), []byte(text), 0o644); e != nil {
		return e
	}
	rel, _ := filepath.Rel(o.vault, filepath.Join(dir, "investigation.md"))
	return emit(out, map[string]any{"id": id, "path": rel, "type": o.kind, "similar_cases": similar,
		"next": "record evidence, conclusions, questions and decisions with `investigation add --id " + id + "`; keep the current state with `investigation state`"})
}

var caseIDFormat = regexp.MustCompile(`^\d{8}-\d{6}-[a-z0-9][a-z0-9-]*$`)

func contains(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}
