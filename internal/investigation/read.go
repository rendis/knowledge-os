package investigation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var idPattern = regexp.MustCompile(`^\d{8}-\d{6}-[a-z0-9]+(?:-[a-z0-9]+)*(?:-\d{2})?$`)
var keyPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var secretPattern = regexp.MustCompile(`(?i)PRIVATE KEY-----|\b(?:password|passwd|pwd|secret|token|api[-_]?key)\s*[:=]\s*\S+|\b(?:gh[pousr]_|github_pat_|sk-(?:proj-|svcacct-)?|xox[baprs]-)\S+`)
var localPattern = regexp.MustCompile("(?:^|[^A-Za-z0-9])(?:/(?:Users|home|tmp|var/tmp)/[^\\s`]+|[A-Za-z]:\\\\[^\\s`]+)")
var requiredFields = []string{"id", "title", "dedupe-key", "status", "created-at", "updated-at", "source-type", "source-ref", "requester-role", "export-intent", "purpose", "vault-outcome", "learning-outcome"}
var requiredSections = [][]string{{"Request summary", "Resumen de la solicitud"}, {"Current state", "Estado actual", "Estado vigente"}, {"References and attachments", "Referencias y adjuntos"}, {"Evidence", "Evidencia"}, {"Affected surfaces", "Superficies afectadas"}, {"Development handoffs", "Handoffs de desarrollo"}, {"Open questions", "Preguntas abiertas"}, {"Decisions", "Decisiones"}, {"Acceptance criteria", "Criterios de aceptación"}, {"Readiness", "Preparación"}, {"History", "Historial"}}

type record struct {
	path, text, visibility string
	fields                 map[string]any
}

func (r record) get(k string) string { v, _ := r.fields[k].(string); return v }
func (r record) lineage() []string   { a, _ := r.fields["consolidated-from"].([]string); return a }
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func scalar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		if s[0] == '"' {
			var v any
			if json.Unmarshal([]byte(s), &v) == nil {
				if x, ok := v.(string); ok {
					return x
				}
			}
		}
		return s[1 : len(s)-1]
	}
	return s
}

var fmLine = regexp.MustCompile(`^([a-z][a-z0-9-]*):(?:\s*(.*))?$`)
var fmItem = regexp.MustCompile(`^\s{2}-\s+(.+?)\s*$`)

func frontmatter(text string) (map[string]any, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return nil, errors.New("frontmatter missing")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, errors.New("frontmatter not closed")
	}
	fields := map[string]any{}
	for i := 1; i < end; {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		m := fmLine.FindStringSubmatch(lines[i])
		if m == nil {
			return nil, fmt.Errorf("invalid frontmatter line %d", i+1)
		}
		if _, ok := fields[m[1]]; ok {
			return nil, fmt.Errorf("duplicate frontmatter key %s", m[1])
		}
		if m[2] != "" {
			fields[m[1]] = scalar(m[2])
			i++
			continue
		}
		items := []string{}
		i++
		for i < end {
			item := fmItem.FindStringSubmatch(lines[i])
			if item == nil {
				break
			}
			items = append(items, scalar(item[1]))
			i++
		}
		fields[m[1]] = items
	}
	return fields, nil
}
func headings(text string) []string {
	a := []string{}
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "## ") {
			a = append(a, strings.TrimSpace(l[3:]))
		}
	}
	return a
}
func section(text string, aliases []string) string {
	in := false
	a := []string{}
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "## ") {
			if in {
				break
			}
			in = contains(aliases, strings.TrimSpace(l[3:]))
			continue
		}
		if in {
			a = append(a, l)
		}
	}
	return strings.TrimSpace(strings.Join(a, "\n"))
}
func legacy(r record) bool {
	for _, k := range requiredFields {
		v := r.fields[k]
		if v == nil || v == "" {
			return true
		}
		if a, ok := v.([]string); ok && len(a) == 0 {
			return true
		}
	}
	h := headings(r.text)
	return (contains(h, "Original request") || contains(h, "Solicitud original")) && !(contains(h, "Request summary") || contains(h, "Resumen de la solicitud"))
}
func noSymlink(path string, allowMissing bool) error {
	st, e := os.Lstat(path)
	if allowMissing && errors.Is(e, fs.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink is not allowed: %s", path)
	}
	return nil
}
func readText(path string) (string, error) {
	if e := noSymlink(path, false); e != nil {
		return "", e
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("not UTF-8: %s", path)
	}
	return string(b), nil
}
func records(root string) ([]record, []map[string]string, error) {
	all := []record{}
	old := []map[string]string{}
	for _, store := range []struct{ p, v string }{{root, "published"}, {filepath.Join(filepath.Dir(root), ".investigations"), "unpublished"}} {
		if e := noSymlink(store.p, true); e != nil {
			return nil, nil, e
		}
		dirs, e := os.ReadDir(store.p)
		if errors.Is(e, fs.ErrNotExist) {
			continue
		}
		if e != nil {
			return nil, nil, e
		}
		for _, d := range dirs {
			if strings.HasPrefix(d.Name(), ".") {
				continue
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil, nil, fmt.Errorf("case directory is symlink: %s", d.Name())
			}
			if !d.IsDir() {
				continue
			}
			p := filepath.Join(store.p, d.Name(), "investigation.md")
			text, e := readText(p)
			if errors.Is(e, fs.ErrNotExist) {
				continue
			}
			if e != nil {
				return nil, nil, e
			}
			f, e := frontmatter(text)
			r := record{p, text, store.v, f}
			if store.v == "unpublished" && (e != nil || legacy(r)) {
				old = append(old, map[string]string{"id": d.Name(), "path": p})
				continue
			}
			if e != nil {
				return nil, nil, e
			}
			all = append(all, r)
		}
	}
	ids, keys := map[string]string{}, map[string]string{}
	for _, r := range all {
		if v, ok := ids[r.get("id")]; ok && v != r.visibility {
			return nil, nil, errors.New("investigation exists in both stores")
		}
		ids[r.get("id")] = r.visibility
		if k := r.get("dedupe-key"); k != "" {
			if id, ok := keys[k]; ok && id != r.get("id") {
				return nil, nil, errors.New("dedupe key identifies multiple cases")
			}
			keys[k] = r.get("id")
		}
	}
	return all, old, nil
}
func digest(b []byte) string  { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func timestamp(s string) bool { _, e := time.Parse(time.RFC3339Nano, s); return e == nil }
func publicSafe(dir string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("shareable content contains symlink")
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return errors.New("shareable content contains special file")
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		if !utf8.Valid(b) {
			return nil
		}
		if secretPattern.Match(b) || localPattern.Match(b) {
			return fmt.Errorf("shareable content contains credential or absolute local path: %s", p)
		}
		return nil
	})
}
func privateSafe(text, id string) error {
	f, e := frontmatter(text)
	if e != nil {
		return e
	}
	if len(f) != 3 || f["id"] != id || f["authority"] != "private-overlay" {
		return errors.New("invalid private overlay identity or authority")
	}
	s, _ := f["updated-at"].(string)
	if !timestamp(s) {
		return errors.New("invalid private overlay timestamp")
	}
	h := headings(text)
	aliases := [][]string{{"Sensitive context", "Contexto sensible"}, {"Private references", "Referencias privadas"}, {"History", "Historial"}}
	if len(h) != 3 {
		return errors.New("invalid private overlay sections")
	}
	for i, a := range aliases {
		if !contains(a, h[i]) {
			return errors.New("invalid private overlay section order")
		}
	}
	if secretPattern.MatchString(text) {
		return errors.New("private overlay contains credential")
	}
	return historyDates(text)
}
func historyDates(text string) error {
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, "; recorded by ") || strings.Contains(l, "; registrado por ") {
			s, _, ok := strings.Cut(strings.TrimPrefix(l, "- "), " — ")
			if !ok || !timestamp(s) {
				return errors.New("attributed history requires timestamp with offset")
			}
		}
	}
	return nil
}

func git(root string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", filepath.Dir(root)}, args...)...).Output()
}

// pythonJSON preserves the ledger's historical sorted JSON with spaces and Unicode.
func pythonJSON(v any) ([]byte, error) { return pythonCanonical(v, false, true) }

var retirementKeys = []string{"id", "title", "snapshot-commit", "public-sha256", "retired-at", "recorded-by", "reason", "source", "dependency-review", "absorption-review", "summary", "destinations", "consolidated-from"}

func retired(root string) ([]map[string]any, error) {
	p := filepath.Join(root, "retired.md")
	s, e := readText(p)
	if errors.Is(e, fs.ErrNotExist) {
		return []map[string]any{}, nil
	}
	if e != nil {
		return nil, e
	}
	const header = "# Retired investigations\n\n"
	if !strings.HasPrefix(s, header) {
		return nil, errors.New("invalid retirement register header")
	}
	entries := []map[string]any{}
	ids := map[string]bool{}
	body := strings.TrimSuffix(strings.TrimPrefix(s, header), "\n")
	if body == "" {
		return entries, nil
	}
	for _, l := range strings.Split(body, "\n") {
		var m map[string]any
		if !strings.HasPrefix(l, "- ") || json.Unmarshal([]byte(l[2:]), &m) != nil || len(m) != len(retirementKeys) {
			return nil, errors.New("invalid retirement record")
		}
		for _, k := range retirementKeys {
			v, ok := m[k]
			if !ok {
				return nil, errors.New("missing retirement field")
			}
			if k == "destinations" || k == "consolidated-from" {
				a, ok := v.([]any)
				if !ok {
					return nil, errors.New("invalid retirement list")
				}
				for _, x := range a {
					v, ok := x.(string)
					if !ok || strings.TrimSpace(v) == "" || strings.ContainsAny(v, "\r\n") || secretPattern.MatchString(v) || localPattern.MatchString(v) {
						return nil, errors.New("invalid retirement list entry")
					}
				}
			} else {
				v, ok := v.(string)
				if !ok || strings.TrimSpace(v) == "" || strings.ContainsAny(v, "\r\n") || secretPattern.MatchString(v) || localPattern.MatchString(v) {
					return nil, errors.New("invalid retirement field value")
				}
			}
		}
		canonical, _ := pythonJSON(m)
		if l != "- "+string(canonical) {
			return nil, errors.New("retirement record is not canonical")
		}
		id := m["id"].(string)
		if !timestamp(m["retired-at"].(string)) || !regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`).MatchString(m["snapshot-commit"].(string)) || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(m["public-sha256"].(string)) {
			return nil, errors.New("invalid retirement identity")
		}
		lineage := []string{id}
		for _, x := range m["consolidated-from"].([]any) {
			lineage = append(lineage, x.(string))
		}
		for _, x := range lineage {
			if !idPattern.MatchString(x) || ids[x] {
				return nil, errors.New("invalid or duplicate retirement lineage")
			}
			ids[x] = true
			if _, e := os.Lstat(filepath.Join(root, x)); !errors.Is(e, fs.ErrNotExist) {
				return nil, errors.New("retired identity has public directory")
			}
		}
		entries = append(entries, m)
	}
	return entries, nil
}
func retirementView(root string, m map[string]any) (map[string]any, error) {
	top, e := git(root, "rev-parse", "--show-toplevel")
	if e != nil {
		return nil, errors.New("retirement requires Git evidence")
	}
	base := strings.TrimSpace(string(top))
	ledger, e := filepath.Rel(base, filepath.Join(root, "retired.md"))
	if e != nil || strings.HasPrefix(ledger, "..") {
		return nil, errors.New("retirement path outside Git root")
	}
	casePath, _ := filepath.Rel(base, filepath.Join(root, m["id"].(string)))
	ledger = filepath.ToSlash(ledger)
	casePath = filepath.ToSlash(casePath)
	canonical, _ := pythonJSON(m)
	marker := "- " + string(canonical) + "\n"
	snapshot, _ := git(root, "rev-parse", m["snapshot-commit"].(string)+":"+casePath)
	log, _ := git(root, "log", "--format=%H", "--", ":(top,literal)"+ledger)
	var commit any
	for _, c := range strings.Fields(string(log)) {
		content, _ := git(root, "show", c+":"+ledger)
		parent, _ := git(root, "show", c+"^:"+ledger)
		tree, _ := git(root, "rev-parse", c+"^:"+casePath)
		present, _ := git(root, "ls-tree", "--full-tree", c, "--", casePath)
		if bytes.Contains(content, []byte(marker)) && !bytes.Contains(parent, []byte(marker)) && len(snapshot) > 0 && bytes.Equal(tree, snapshot) && len(present) == 0 {
			commit = c
			break
		}
	}
	v := map[string]any{}
	for k, x := range m {
		v[k] = x
	}
	v["retirement_commit"] = commit
	v["commit_state"] = "pending"
	if commit != nil {
		v["commit_state"] = "committed"
	}
	return v, nil
}
func retirementConflicts(rs []record, ret []map[string]any) error {
	ids := map[string]bool{}
	for _, m := range ret {
		ids[m["id"].(string)] = true
		for _, x := range m["consolidated-from"].([]any) {
			ids[x.(string)] = true
		}
	}
	for _, r := range rs {
		for _, id := range append([]string{r.get("id")}, r.lineage()...) {
			if ids[id] {
				return errors.New("retired ID resolves to a live case")
			}
		}
	}
	return nil
}
func listCases(root string) (any, error) {
	rs, old, e := records(root)
	if e != nil {
		return nil, e
	}
	ret, e := retired(root)
	if e != nil {
		return nil, e
	}
	if e = retirementConflicts(rs, ret); e != nil {
		return nil, e
	}
	views := []map[string]any{}
	for _, m := range ret {
		v, e := retirementView(root, m)
		if e != nil {
			return nil, e
		}
		views = append(views, v)
	}
	cases := []map[string]any{}
	for _, r := range rs {
		cases = append(cases, map[string]any{"id": r.get("id"), "title": r.get("title"), "status": r.fields["status"], "visibility": r.visibility, "summary": section(r.text, requiredSections[0]), "updated-at": r.fields["updated-at"]})
	}
	return map[string]any{"status": "listed", "cases": cases, "retired": views, "legacy": old}, nil
}
func loadCase(root, id string) (any, error) {
	if !idPattern.MatchString(id) {
		return nil, errors.New("invalid investigation ID")
	}
	ret, e := retired(root)
	if e != nil {
		return nil, e
	}
	for _, m := range ret {
		match := m["id"] == id
		for _, x := range m["consolidated-from"].([]any) {
			match = match || x == id
		}
		if match {
			v, e := retirementView(root, m)
			if e != nil {
				return nil, e
			}
			v["status"] = "retired"
			return v, nil
		}
	}
	rs, old, e := records(root)
	if e != nil {
		return nil, e
	}
	if e = retirementConflicts(rs, ret); e != nil {
		return nil, e
	}
	for _, o := range old {
		if o["id"] == id {
			_, e := os.Stat(filepath.Join(root, id, "investigation.md"))
			if errors.Is(e, fs.ErrNotExist) {
				return map[string]any{"status": "legacy", "id": id, "path": o["path"], "visibility": "unpublished"}, nil
			}
		}
	}
	matches := []record{}
	for _, r := range rs {
		if r.get("id") == id || contains(r.lineage(), id) {
			matches = append(matches, r)
		}
	}
	if len(matches) != 1 {
		return nil, errors.New("exactly one live investigation must resolve")
	}
	r := matches[0]
	id = r.get("id")
	if e = publicSafe(filepath.Dir(r.path)); e != nil {
		return nil, e
	}
	privateRoot := filepath.Join(filepath.Dir(root), ".investigations-private")
	privateDir := filepath.Join(privateRoot, id)
	privatePath := filepath.Join(privateDir, "private.md")
	for _, p := range []string{privateRoot, privateDir, privatePath} {
		if e = noSymlink(p, true); e != nil {
			return nil, e
		}
	}
	private := map[string]any{"available": false, "path": nil, "sha256": "absent"}
	text, e := readText(privatePath)
	if e == nil {
		if e = privateSafe(text, id); e != nil {
			return nil, e
		}
		private = map[string]any{"available": true, "path": privatePath, "sha256": digest([]byte(text))}
	} else if !errors.Is(e, fs.ErrNotExist) {
		return nil, e
	}
	localPath := filepath.Join(privateDir, "local")
	local := map[string]any{"available": false, "path": nil}
	if e = noSymlink(localPath, true); e != nil {
		return nil, e
	}
	d, e := os.ReadDir(localPath)
	if e == nil && len(d) > 0 {
		local = map[string]any{"available": true, "path": localPath}
	} else if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return nil, e
	}
	tree, _, e := Snapshot(filepath.Dir(r.path))
	if e != nil {
		return nil, e
	}
	return map[string]any{"status": "loaded", "id": id, "visibility": r.visibility, "public": map[string]any{"path": r.path, "sha256": digest([]byte(r.text)), "tree_sha256": tree}, "private": private, "local": local}, nil
}

// StableRead makes no filesystem mutations. It refuses a live Python mutation
// gate and detects store changes across the read, including publication moves.
func stableRead(root string, fn func() (any, error)) (any, error) {
	fingerprint := func() (string, error) {
		if b, e := currentBytes(retirementJournalPath(root)); e != nil {
			return "", e
		} else if b != nil {
			return "", errors.New("pending retirement transaction; recovery required")
		}
		if b, e := currentBytes(treeJournalPath(root)); e != nil {
			return "", e
		} else if b != nil {
			return "", errors.New("pending investigation tree transaction; recovery required")
		}
		if b, e := currentBytes(journalPath(root)); e != nil {
			return "", e
		} else if b != nil {
			return "", errors.New("pending investigation transaction; mutation recovery required")
		}
		// Fingerprint case records and visibility, not local workbenches. Legacy
		// investigations can contain node_modules, symlinks and unrelated large data.
		for _, p := range []string{root, filepath.Join(filepath.Dir(root), ".investigations")} {
			if e := noSymlink(p, true); e != nil {
				return "", e
			}
			if _, e := os.Lstat(filepath.Join(p, ".open.lock")); e == nil {
				return "", errors.New("live investigation mutation lock exists")
			} else if !errors.Is(e, fs.ErrNotExist) {
				return "", e
			}
		}
		rs, legacy, e := records(root)
		if e != nil {
			return "", e
		}
		parts := []string{}
		for _, r := range rs {
			parts = append(parts, r.path+":"+digest([]byte(r.text)))
		}
		for _, l := range legacy {
			b, e := os.ReadFile(l["path"])
			if e != nil {
				return "", e
			}
			parts = append(parts, l["path"]+":"+digest(b))
		}
		ledger, e := currentBytes(filepath.Join(root, "retired.md"))
		if e != nil {
			return "", e
		}
		parts = append(parts, optionalDigest(ledger))
		return strings.Join(parts, ":"), nil
	}
	before, e := fingerprint()
	if e != nil {
		return nil, e
	}
	v, e := fn()
	if e != nil {
		return nil, e
	}
	after, e := fingerprint()
	if e != nil {
		return nil, e
	}
	if before != after {
		return nil, errors.New("investigation changed during read; retry")
	}
	return v, nil
}
