package cases

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"documentation-vault/internal/devhandoff"
	"documentation-vault/internal/discover"
)

// Issue is one finding of the case gate: error blocks publication, warning is reported (legacy cases
// report schema findings as warnings), review goes to the reviewer.
type Issue struct {
	Severity string `json:"severity"`
	Where    string `json:"where"`
	Detail   string `json:"detail"`
}

// Result is the gate outcome for one case file.
type Result struct {
	ID      string         `json:"id"`
	Path    string         `json:"path"`
	Type    string         `json:"type"`
	Status  string         `json:"status"`
	Legacy  bool           `json:"legacy,omitempty"`
	OK      bool           `json:"ok"`
	Records map[string]int `json:"records"`
	Issues  []Issue        `json:"issues"`
}

var (
	comment    = regexp.MustCompile(`(?s)<!--.*?-->`)
	fence      = regexp.MustCompile("(?ms)^```.*?^```")
	heading2   = regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`)
	recordDef  = regexp.MustCompile("^\\s*(?:[-*+]\\s+|\\|\\s*|#{2,4}\\s+)?[*_`]*((?:AC|DH|CH|[EDQRSAF])-\\d{3,})\\b")
	recordRef  = regexp.MustCompile(`\b((?:AC|DH|CH|[EDQRSAF])-\d{3,})\b`)
	sourceRef  = regexp.MustCompile("\\]\\(|\\[\\[|https?://|`[^`\\s]*[/.][^`\\s]*`|@[0-9a-f]{7,}|\\b[0-9a-f]{7,40}\\b|#L\\d+|\\b(?:A|E|F)-\\d{3,}\\b|\\b[A-Z][A-Z0-9]{1,9}-\\d{2,}\\b|(?i)snapshot|(?i)\\bquery\\b|(?i)\\bconsulta\\b|(?i)\\b(?:solicitante|requester|usuario|user|reuni[oó]n|meeting)\\b[^\\n]*\\d{4}-\\d{2}-\\d{2}")
	wikilink   = regexp.MustCompile(`\[\[([^\]|#]+)`)
	secret     = regexp.MustCompile(`(?i)PRIVATE KEY-----|\b(?:password|passwd|pwd|secret|token|api[-_]?key)\s*[:=]\s*\S+|\b(?:gh[pousr]_|github_pat_|sk-(?:proj-|svcacct-)?|xox[baprs]-)\S+`)
	localPath  = regexp.MustCompile("(?:^|[^A-Za-z0-9])(?:/(?:Users|home|tmp|var/tmp|private/tmp)/[^\\s`)]+|[A-Za-z]:\\\\[^\\s`]+)")
	wordToken  = regexp.MustCompile(`[\p{L}\p{N}]+`)
	noteFolder = regexp.MustCompile(`^[1-7]\d-`)
	outcomes   = regexp.MustCompile(`^(completed|abandoned|superseded-by:\d{8}-\d{6}-[a-z0-9-]+)$`)
)

func sectionKey(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	for _, names := range sectionNames {
		for k, v := range names {
			if strings.ToLower(v) == t {
				return k
			}
		}
	}
	return ""
}

// vaultIndex holds what check compares a case against: note basenames and 8-word shingles of notes.
type vaultIndex struct {
	notes    map[string]bool
	shingles map[string]string
}

const shingleSize = 8

func shingles(text string) []string {
	w := wordToken.FindAllString(strings.ToLower(text), -1)
	out := []string{}
	for i := 0; i+shingleSize <= len(w); i++ {
		out = append(out, strings.Join(w[i:i+shingleSize], " "))
	}
	return out
}

func indexVault(vault string) vaultIndex {
	ix := vaultIndex{notes: map[string]bool{}, shingles: map[string]string{}}
	_ = filepath.WalkDir(vault, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(vault, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		stem := strings.TrimSuffix(d.Name(), ".md")
		ix.notes[strings.ToLower(stem)] = true
		if noteFolder.MatchString(strings.SplitN(rel, "/", 2)[0]) {
			if b, e := os.ReadFile(p); e == nil {
				for _, s := range shingles(string(b)) {
					if _, ok := ix.shingles[s]; !ok {
						ix.shingles[s] = stem
					}
				}
			}
		}
		return nil
	})
	return ix
}

// Check gates one case file (path relative to the vault or absolute).
func Check(vault, path string) (Result, error) {
	return checkWith(vault, path, indexVault(vault))
}

func checkWith(vault, path string, ix vaultIndex) (Result, error) {
	full := filepathJoin(vault, path)
	b, e := os.ReadFile(full)
	if e != nil {
		return Result{}, e
	}
	return checkContent(vault, full, string(b), ix), nil
}

// checkContent gates case content as if it were stored at full (its directory resolves artifacts and
// handoff packages).
func checkContent(vault, full, raw string, ix vaultIndex) Result {
	fm := frontmatter(raw)
	r := Result{ID: fm["id"], Records: map[string]int{}, Issues: []Issue{}}
	r.Path, _ = filepath.Rel(vault, full)
	r.Type, r.Status, _, r.Legacy = normalize(fm)
	strict := "error"
	if r.Legacy {
		strict = "warning"
	}
	add := func(sev, where, detail string) { r.Issues = append(r.Issues, Issue{sev, where, detail}) }

	// Leaks are errors in every schema: the case is shareable content.
	for _, m := range secret.FindAllString(raw, 3) {
		add("error", "credential", "remove the credential value ("+firstRunes(m, 24)+"…); keep secrets out of the case, the overlay only names where they live")
	}
	for _, m := range localPath.FindAllString(raw, 3) {
		add("error", "local path", "`"+strings.TrimSpace(m)+"` is machine-specific; keep it in .investigations-private/<id>/local/ or reference a portable source")
	}

	text := fence.ReplaceAllString(comment.ReplaceAllString(raw, ""), "")
	body := text
	if i := strings.Index(text[min(4, len(text)):], "\n---"); strings.HasPrefix(text, "---\n") && i >= 0 {
		body = text[4+i+4:]
	}

	// Schema.
	if !r.Legacy {
		if !idPattern.MatchString(fm["id"]) {
			add("error", "frontmatter", "id must look like 20260925-091623-short-slug")
		}
		if fm["title"] == "" || fm["created"] == "" {
			add("error", "frontmatter", "title and created are required")
		}
		if fm["type"] != "understanding" && fm["type"] != "development" {
			add("error", "frontmatter", "type must be understanding or development")
		}
		if fm["status"] != "open" && fm["status"] != "closed" {
			add("error", "frontmatter", "status must be open or closed")
		}
		if fm["status"] == "closed" && !outcomes.MatchString(fm["outcome"]) {
			add("error", "frontmatter", "a closed case needs outcome: completed, abandoned or superseded-by:<id>")
		}
		present := map[string]bool{}
		for _, m := range heading2.FindAllStringSubmatch(body, -1) {
			present[sectionKey(m[1])] = true
		}
		for _, s := range requiredSections[r.Type] {
			if !present[s] {
				add("error", "sections", "missing section "+sectionNames["es"][s]+" / "+sectionNames["en"][s])
			}
		}
	}

	// Records: unique definitions, references resolve, evidence cites its source.
	defs := map[string]bool{}
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		m := recordDef.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		id := m[1]
		if defs[id] && !strings.HasPrefix(strings.TrimSpace(l), "|") {
			add(strict, id, "defined more than once")
		}
		defs[id] = true
		r.Records[strings.SplitN(id, "-", 2)[0]]++
		if strings.HasPrefix(id, "E-") {
			// A list or table record runs until a blank line; a heading record runs until the next heading.
			asHeading := strings.HasPrefix(strings.TrimSpace(l), "#")
			block := l
			for j := i + 1; j < len(lines); j++ {
				next := lines[j]
				if strings.HasPrefix(next, "#") || !asHeading && (strings.TrimSpace(next) == "" || recordDef.FindStringSubmatch(next) != nil) {
					break
				}
				block += "\n" + next
			}
			if !sourceRef.MatchString(strings.Replace(block, id, "", 1)) { // its own ID is not a source
				add(strict, id, "evidence without a source: cite the permalink or file@commit, platform snapshot, query, work item, record (E-/A-) or the requester statement with its date")
			}
		}
	}
	dir := filepath.Dir(full)
	for _, m := range recordRef.FindAllStringSubmatch(body, -1) {
		id := m[1]
		if defs[id] || fileRecord(dir, id) {
			continue
		}
		defs[id] = true // report once
		add(strict, id, "referenced but not defined in the case")
	}
	if !r.Legacy && fm["status"] == "closed" && fm["outcome"] == "completed" && r.Records["E"]+r.Records["F"] == 0 {
		add("error", "outcome", "a completed case needs at least one evidence or conclusion record")
	}

	// Links resolve to vault notes.
	for _, m := range wikilink.FindAllStringSubmatch(body, -1) {
		t := strings.ToLower(strings.TrimSpace(m[1]))
		if !ix.notes[t] {
			add(strict, "[["+m[1]+"]]", "link does not resolve to a vault note")
		}
	}

	// A paragraph copied from a note repeats the vault; the case references it instead.
	for _, para := range strings.Split(body, "\n\n") {
		sh := shingles(para)
		if len(sh) < 20 { // about 28 words: shorter overlaps are quotes or shared phrasing
			continue
		}
		hits := map[string]int{}
		for _, s := range sh {
			if n, ok := ix.shingles[s]; ok {
				hits[n]++
			}
		}
		for n, c := range hits {
			if c*10 >= len(sh)*6 {
				add(strict, "[["+n+"]]", fmt.Sprintf("a paragraph repeats %d%% of this note; reference it with [[%s]] and keep only what is new", c*100/len(sh), n))
				break
			}
		}
	}

	// Task packages prepared for handoffs pass their own gate.
	pkgs, _ := filepath.Glob(filepath.Join(dir, "handoffs", "DH-*.md"))
	deps := map[string][]string{}
	for _, pp := range pkgs {
		p, issues, e := devhandoff.CheckPackage(pp)
		if e != nil {
			continue
		}
		for _, i := range issues {
			add(i.Severity, "handoffs/"+filepath.Base(pp), i.Detail)
		}
		deps[p.Handoff] = p.DependsOn
	}
	for id, ds := range deps {
		for _, d := range ds {
			if _, ok := deps[d]; !ok {
				add("error", "handoffs/"+id+".md", "depends-on "+d+", which is not a package of this case")
			}
		}
		if dependsOnItself(id, deps) {
			add("error", "handoffs/"+id+".md", "depends-on forms a cycle")
		}
	}

	// Relations discovery contradicts go to review.
	for _, f := range discover.ContradictedRelations(vault, body) {
		add("review", f[0], f[1])
	}

	r.OK = true
	for _, i := range r.Issues {
		if i.Severity == "error" {
			r.OK = false
		}
	}
	return r
}

func dependsOnItself(start string, deps map[string][]string) bool {
	seen := map[string]bool{}
	stack := append([]string{}, deps[start]...)
	for len(stack) > 0 {
		d := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if d == start {
			return true
		}
		if !seen[d] {
			seen[d] = true
			stack = append(stack, deps[d]...)
		}
	}
	return false
}

// fileRecord accepts artifact and story IDs defined as files under artifacts/ or exports/.
func fileRecord(dir, id string) bool {
	if !strings.HasPrefix(id, "A-") && !strings.HasPrefix(id, "S-") {
		return false
	}
	for _, sub := range []string{"artifacts", "exports"} {
		if m, _ := filepath.Glob(filepath.Join(dir, sub, id+"*")); len(m) > 0 {
			return true
		}
	}
	return false
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// CheckIntroduced gates a case changed on a branch: without base content every error fails; with the
// base version only errors absent from it fail, so legacy debt does not block an edit.
func CheckIntroduced(vault, path string, base []byte) (bool, []string, int, error) {
	full := filepathJoin(vault, path)
	b, e := os.ReadFile(full)
	if e != nil {
		return false, nil, 0, e
	}
	var prev *string
	if base != nil {
		s := string(base)
		prev = &s
	}
	ok, introduced, pre := introducedErrors(vault, full, prev, string(b), indexVault(vault))
	return ok, introduced, pre, nil
}

// introducedErrors compares the gate on the next content with the gate on the previous content.
func introducedErrors(vault, full string, prev *string, next string, ix vaultIndex) (bool, []string, int) {
	keys := func(res Result, severities ...string) map[string]bool {
		out := map[string]bool{}
		for _, i := range res.Issues {
			for _, s := range severities {
				if i.Severity == s {
					out[i.Where+"|"+i.Detail] = true
				}
			}
		}
		return out
	}
	r := checkContent(vault, full, next, ix)
	if r.OK {
		return true, nil, 0
	}
	now := keys(r, "error")
	if prev == nil {
		return false, sortedKeys(now), 0
	}
	// A finding the previous version already had is not introduced, whatever its severity there: the
	// earlier format reports its debt as warnings, and migrating it does not create that debt.
	before := keys(checkContent(vault, full, *prev, ix), "error", "warning")
	introduced := []string{}
	for k := range now {
		if !before[k] {
			introduced = append(introduced, k)
		}
	}
	sort.Strings(introduced)
	return len(introduced) == 0, introduced, len(now) - len(introduced)
}

func filepathJoin(vault, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(vault, path)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func runCheck(o options, out io.Writer) error {
	all, e := List(o.vault)
	if e != nil {
		return e
	}
	ix := indexVault(o.vault)
	results := []Result{}
	ok := true
	for _, c := range all {
		if c.Path == "" || o.id != "" && c.ID != o.id {
			continue
		}
		r, e := checkWith(o.vault, c.Path, ix)
		if e != nil {
			return e
		}
		ok = ok && r.OK
		results = append(results, r)
	}
	if o.id != "" && len(results) == 0 {
		return fmt.Errorf("no case %s", o.id)
	}
	if e := emit(out, map[string]any{"ok": ok, "cases": results}); e != nil {
		return e
	}
	if !ok {
		return fmt.Errorf("case gate failed")
	}
	return nil
}
