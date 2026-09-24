package investigation

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

//go:embed template.md
var caseTemplate string

type options map[string][]string

func (o options) get(k string) string {
	a := o[k]
	if len(a) > 0 {
		return a[len(a)-1]
	}
	return ""
}
func (o options) required(keys ...string) error {
	for _, k := range keys {
		if o.get(k) == "" {
			return fmt.Errorf("--%s is required", k)
		}
	}
	return nil
}
func (o options) defaultValue(k, v string) {
	if o.get(k) == "" {
		o[k] = []string{v}
	}
}
func eventInput(values ...string) error {
	for _, v := range values {
		if strings.TrimSpace(v) == "" || strings.ContainsAny(v, "\r\n") || secretPattern.MatchString(v) || localPattern.MatchString(v) {
			return errors.New("event values must be nonempty portable lines without credentials")
		}
	}
	return nil
}
func identity(root string) (string, error) {
	values := []string{}
	for _, k := range []string{"user.name", "user.email"} {
		b, e := git(root, "config", "--get", k)
		if e != nil {
			return "", fmt.Errorf("effective Git %s is required", k)
		}
		s := strings.TrimSpace(string(b))
		if e = eventInput(s); e != nil {
			return "", e
		}
		values = append(values, s)
	}
	return values[0] + " <" + values[1] + ">", nil
}
func eventTime(o options) (string, error) {
	s := o.get("timestamp")
	if s == "" {
		return time.Now().Format(time.RFC3339), nil
	}
	if !timestamp(s) {
		return "", errors.New("timestamp must include UTC offset")
	}
	return s, nil
}
func isSpanish(text string) bool {
	return regexp.MustCompile(`(?m)^## Historial\s*$`).MatchString(text)
}
func noteSpanish(root string) bool {
	b, e := os.ReadFile(filepath.Join(filepath.Dir(root), "instance.yaml"))
	if e != nil {
		return false
	}
	m := regexp.MustCompile(`(?m)^locale:\s*$\n(?:^[ \t].*\n)*?^  notes:\s*([^#\s]+)`).FindSubmatch(b)
	return len(m) > 1 && strings.HasPrefix(strings.ToLower(string(m[1])), "es")
}
func attributed(ts, action, who, source string, es bool) string {
	rec, src := "recorded by", "source"
	if es {
		rec, src = "registrado por", "fuente"
	}
	return "- " + ts + " — " + action + "; " + rec + " " + who + "; " + src + ": " + source + "."
}
func appendHistory(text, event string) (string, error) {
	re := regexp.MustCompile(`(?m)^## (?:History|Historial)\s*$`)
	loc := re.FindStringIndex(text)
	if loc == nil {
		return "", errors.New("History section missing")
	}
	pos := strings.Index(text[loc[1]:], "\n## ")
	if pos < 0 {
		pos = len(text)
	} else {
		pos += loc[1]
	}
	suffix := strings.TrimLeft(text[pos:], "\n")
	out := strings.TrimRight(text[:pos], " \r\n") + "\n\n" + strings.TrimRight(event, "\r\n")
	if suffix != "" {
		out += "\n\n" + suffix
	} else {
		out += "\n"
	}
	return out, nil
}
func setField(text, key, value string) (string, error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return "", errors.New("frontmatter missing")
	}
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], key+":") {
			lines[i] = key + ": " + value
			return strings.Join(lines, "\n"), nil
		}
		if lines[i] == "---" {
			lines = append(lines[:i], append([]string{key + ": " + value}, lines[i:]...)...)
			return strings.Join(lines, "\n"), nil
		}
	}
	return "", errors.New("frontmatter not closed")
}
func removeFields(text string, keys ...string) string {
	lines := strings.Split(text, "\n")
	out := []string{}
	fm := true
	for i, l := range lines {
		if i > 0 && l == "---" {
			fm = false
		}
		skip := false
		if fm {
			for _, k := range keys {
				skip = skip || strings.HasPrefix(l, k+":")
			}
		}
		if !skip {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
func quoted(s string) string { b, _ := pythonJSON(s); return string(b) }
func locate(root, id string) (record, error) {
	rs, _, e := records(root)
	if e != nil {
		return record{}, e
	}
	ret, e := retired(root)
	if e != nil {
		return record{}, e
	}
	if e = retirementConflicts(rs, ret); e != nil {
		return record{}, e
	}
	matches := []record{}
	for _, r := range rs {
		if r.get("id") == id || contains(r.lineage(), id) {
			matches = append(matches, r)
		}
	}
	if len(matches) != 1 {
		return record{}, errors.New("exactly one live investigation must resolve")
	}
	return matches[0], nil
}

// mutationGate also acquires the legacy directory gate while Python remains
// installed. A stale legacy gate fails closed and is never automatically removed.
func mutationGate(root string, fn func() (any, error)) (any, error) {
	local := filepath.Join(filepath.Dir(root), ".investigations")
	if e := noSymlink(local, true); e != nil {
		return nil, e
	}
	if e := os.MkdirAll(local, 0700); e != nil {
		return nil, e
	}
	lockPath := filepath.Join(local, ".native.lock")
	if e := noSymlink(lockPath, true); e != nil {
		return nil, e
	}
	lock := flock.New(lockPath)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ok, e := lock.TryLockContext(ctx, 10*time.Millisecond)
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, errors.New("investigation is locked")
	}
	defer lock.Unlock()
	gate := filepath.Join(local, ".open.lock")
	deadline := time.Now().Add(2 * time.Second)
	for {
		e = os.Mkdir(gate, 0700)
		if e == nil {
			break
		}
		if !errors.Is(e, os.ErrExist) {
			return nil, e
		}
		// The OS lock proves no native writer owns this marker. Legacy gates
		// without it remain untouched; they may belong to a live Python writer.
		owner := filepath.Join(gate, "native-owner")
		if b, readErr := os.ReadFile(owner); readErr == nil && string(b) == "vaultctl-v1\n" {
			if err := noSymlink(gate, false); err != nil {
				return nil, err
			}
			if err := os.Remove(owner); err != nil {
				return nil, err
			}
			if err := os.Remove(gate); err != nil {
				return nil, err
			}
			continue
		}
		if time.Now().After(deadline) {
			return nil, errors.New("legacy investigation mutation gate is locked")
		}
		time.Sleep(10 * time.Millisecond)
	}
	owner := filepath.Join(gate, "native-owner")
	if err := atomicWrite(owner, []byte("vaultctl-v1\n")); err != nil {
		os.Remove(gate)
		return nil, err
	}
	defer func() { os.Remove(owner); os.Remove(gate) }()
	if err := recoverRetirement(root); err != nil {
		return nil, err
	}
	if err := recoverTree(root); err != nil {
		return nil, err
	}
	if err := recoverJournal(root); err != nil {
		return nil, err
	}
	return fn()
}
func atomicWrite(path string, b []byte) error {
	if e := noSymlink(path, true); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".native-write-*.tmp")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(temp, path)
}
func compareWrite(root string, r record, updated string, migration bool) ([]string, error) {
	fields, e := frontmatter(updated)
	if e != nil {
		return nil, e
	}
	candidate := r
	candidate.text = updated
	candidate.fields = fields
	before, e := validateVault(root)
	if e != nil {
		return nil, e
	}
	// Validate the candidate against the same files before exposing its bytes.
	candidateErrors := validateRecord(candidate)
	all := []string{}
	for _, s := range before {
		prefix := filepath.Base(filepath.Dir(r.path)) + "/investigation.md: "
		if !strings.HasPrefix(s, prefix) {
			all = append(all, s)
		}
	}
	all = append(all, candidateErrors...)
	if len(all) > 0 {
		newErrors := []string{}
		for _, s := range all {
			if !contains(before, s) {
				newErrors = append(newErrors, s)
			}
		}
		if !migration || len(newErrors) > 0 {
			return nil, fmt.Errorf("candidate validation failed: %s", strings.Join(all, "; "))
		}
	}
	current, e := os.ReadFile(r.path)
	if e != nil {
		return nil, e
	}
	if string(current) != r.text {
		return nil, errors.New("investigation changed before commit")
	}
	if e = atomicWrite(r.path, []byte(updated)); e != nil {
		return nil, e
	}
	warnings := []string{}
	if len(all) > 0 {
		warnings = append(warnings, "Other pre-existing case errors remain; complete the requested public lifecycle migration before normal mutations")
	}
	return warnings, nil
}
func lifecycle(root, verb string, o options) (any, error) {
	if e := o.required("id", "expected-public-sha256", "reason", "source"); e != nil {
		return nil, e
	}
	if e := eventInput(o.get("reason"), o.get("source")); e != nil {
		return nil, e
	}
	who, e := identity(root)
	if e != nil {
		return nil, e
	}
	ts, e := eventTime(o)
	if e != nil {
		return nil, e
	}
	return mutationGate(root, func() (any, error) {
		r, e := locate(root, o.get("id"))
		if e != nil {
			return nil, e
		}
		if digest([]byte(r.text)) != o.get("expected-public-sha256") {
			return nil, errors.New("stale public snapshot")
		}
		current := r.get("status")
		es := isSpanish(r.text)
		updated := r.text
		action := ""
		migration := false
		result := map[string]any{"id": o.get("id")}
		if verb == "transition" {
			if e = o.required("to"); e != nil {
				return nil, e
			}
			to := o.get("to")
			migration = contains([]string{"intake", "scoped", "validating", "ready-to-export", "exported"}, current) || r.fields["resume-to"] != nil || (current == "closed" && !contains([]string{"completed", "abandoned"}, r.get("closure-outcome")))
			allowed := (current == "investigating" && to == "blocked") || ((current == "blocked" || current == "closed") && to == "investigating") || (migration && to == "investigating")
			if !allowed || (migration && to != "investigating") {
				return nil, errors.New("unsupported lifecycle transition")
			}
			blocked := o.get("blocked-on")
			if to == "blocked" {
				if e = eventInput(blocked); e != nil {
					return nil, e
				}
			} else if blocked != "" {
				return nil, errors.New("blocked-on allowed only when blocking")
			}
			updated, e = setField(updated, "status", to)
			if e != nil {
				return nil, e
			}
			if to == "blocked" {
				updated, e = setField(updated, "blocked-on", quoted(blocked))
				if e != nil {
					return nil, e
				}
			} else {
				updated = removeFields(updated, "blocked-on", "closure-outcome", "resume-to")
			}
			switch {
			case migration:
				action = "Migrated prior state `" + current + "` to `investigating`; reason: " + o.get("reason")
				if es {
					action = "Estado anterior `" + current + "` migrado a `investigating`; razón: " + o.get("reason")
				}
			case current == "investigating":
				action = "Investigation blocked by " + blocked + "; reason: " + o.get("reason")
				if es {
					action = "Investigación bloqueada por " + blocked + "; razón: " + o.get("reason")
				}
			case current == "blocked":
				action = "Investigation unblocked; reason: " + o.get("reason")
				if es {
					action = "Investigación desbloqueada; razón: " + o.get("reason")
				}
			default:
				action = "Investigation reopened; reason: " + o.get("reason")
				if es {
					action = "Investigación reabierta; razón: " + o.get("reason")
				}
			}
			result["status"] = "transitioned"
			result["from"] = current
			result["to"] = to
			result["migration"] = migration
		} else {
			if e = o.required("decision", "limitations"); e != nil {
				return nil, e
			}
			if e = eventInput(o.get("limitations")); e != nil {
				return nil, e
			}
			decision := o.get("decision")
			if decision != "complete" && decision != "abandoned" {
				return nil, errors.New("invalid closure decision")
			}
			if !contains([]string{"investigating", "blocked"}, current) || r.fields["resume-to"] != nil || r.get("closure-outcome") != "" || (current == "blocked" && r.get("blocked-on") == "") || (current == "investigating" && r.get("blocked-on") != "") {
				return nil, errors.New("invalid lifecycle for closure")
			}
			evidence := o["evidence"]
			if decision == "complete" && (r.get("purpose") == "undecided" || r.get("vault-outcome") == "not-evaluated" || len(evidence) == 0) {
				return nil, errors.New("completion requires resolved purpose, evaluated vault outcome and evidence")
			}
			exports, e := exportIDs(filepath.Dir(r.path), o.get("id"))
			if e != nil {
				return nil, e
			}
			for _, v := range evidence {
				if !registerID.MatchString(v) || (!declares(r.text, v) && !contains(exports, v)) {
					return nil, errors.New("closure evidence must reference existing register IDs")
				}
			}
			outcome := "completed"
			if decision == "abandoned" {
				outcome = "abandoned"
			}
			updated, _ = setField(updated, "status", "closed")
			updated, _ = setField(updated, "closure-outcome", outcome)
			updated = removeFields(updated, "blocked-on", "resume-to")
			if decision == "abandoned" && r.get("vault-outcome") == "not-evaluated" {
				updated, _ = setField(updated, "vault-outcome", "none")
			}
			refs := []string{}
			for _, v := range evidence {
				refs = append(refs, "`"+v+"`")
			}
			ev := strings.Join(refs, ", ")
			if ev == "" {
				ev = "none"
			}
			action = "Investigation closed as `" + outcome + "`; reason: " + o.get("reason") + "; evidence: " + ev + "; outstanding limitations: " + o.get("limitations")
			if es {
				action = "Investigación cerrada como `" + outcome + "`; razón: " + o.get("reason") + "; evidencia: " + ev + "; limitaciones pendientes: " + o.get("limitations")
			}
			result["status"] = "closed"
			result["outcome"] = outcome
		}
		updated, e = setField(updated, "updated-at", ts)
		if e != nil {
			return nil, e
		}
		updated, e = appendHistory(updated, attributed(ts, action, who, o.get("source"), es))
		if e != nil {
			return nil, e
		}
		warnings, e := compareWrite(root, r, updated, migration)
		if e != nil {
			return nil, e
		}
		result["warnings"] = warnings
		return result, nil
	})
}

var registerID = regexp.MustCompile(`^(?:E|A|Q|D|AC|S|DH)-[0-9]{3,}$`)

func declares(text, id string) bool {
	prefix := strings.Split(id, "-")[0]
	index := map[string]int{"E": 3, "A": 2, "Q": 6, "D": 7, "AC": 8, "DH": 5}
	i, ok := index[prefix]
	if !ok {
		return false
	}
	s := section(text, requiredSections[i])
	return regexp.MustCompile("(?m)^(?:-\\s+|###\\s+)`?" + regexp.QuoteMeta(id) + "`?(?:$|[\\s:—(])").MatchString(s)
}
func exportIDs(dir, id string) ([]string, error) {
	paths, e := filepath.Glob(filepath.Join(dir, "exports", "*.md"))
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for _, p := range paths {
		s, e := readText(p)
		if e != nil {
			continue
		}
		f, e := frontmatter(s)
		if e != nil {
			continue
		}
		story, _ := f["story-id"].(string)
		if regexp.MustCompile(`^S-[0-9]{3,}$`).MatchString(story) && f["source-investigation"] == id {
			ids = append(ids, story)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func renderCase(root string, o options, ts, who string) string {
	source := "direct user request"
	if len(o["source-ref"]) > 0 {
		source = o["source-ref"][0]
	}
	text := caseTemplate
	replacements := map[string]string{"id: <investigation-id>": "id: " + o.get("id"), "title: <title>": "title: " + quoted(o.get("title")), "dedupe-key: <dedupe-key>": "dedupe-key: " + o.get("dedupe-key"), "created-at: <ISO-8601 timestamp>": "created-at: " + ts, "updated-at: <ISO-8601 timestamp>": "updated-at: " + ts, "source-type: <message|bug|ticket|issue|attachment|other>": "source-type: " + o.get("source-type"), "source-ref: <source reference>": "source-ref: " + quoted(source), "requester-role: <role|unknown>": "requester-role: " + quoted(o.get("requester-role")), "export-intent: <technical-stories|user-stories|mixed|undecided>": "export-intent: " + o.get("export-intent"), "purpose: <knowledge|development|mixed|undecided>": "purpose: " + o.get("purpose"), "vault-outcome: <not-evaluated|none|deferred-until-production|candidate-for-audit|documented>": "vault-outcome: " + o.get("vault-outcome"), "learning-outcome: <not-evaluated|no-learning|already-covered|insufficient-evidence|candidate|documented>": "learning-outcome: " + o.get("learning-outcome"), "# <Title>": "# " + o.get("title"), "<Formalize the relevant request without preserving the conversation transcript.>": o.get("request-summary")}
	action := "Case file created"
	es := noteSpanish(root)
	if es {
		action = "Expediente creado"
	}
	replacements["- <timestamp> — Case file created."] = attributed(ts, action, who, source, es)
	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}
	text = strings.Replace(text, "### Objective\n\n", "### Objective\n\n"+o.get("objective")+"\n\n", 1)
	if len(o["source-ref"]) > 0 {
		refs := []string{}
		for _, s := range o["source-ref"] {
			refs = append(refs, "- `"+s+"`")
		}
		text = strings.Replace(text, "## References and attachments\n", "## References and attachments\n\n"+strings.Join(refs, "\n")+"\n", 1)
	}
	if es {
		pairs := [][2]string{{"## Request summary", "## Resumen de la solicitud"}, {"## Current state", "## Estado actual"}, {"### Objective", "### Objetivo"}, {"### Scope", "### Alcance"}, {"### Out of scope", "### Fuera de alcance"}, {"### Current productive state", "### Estado productivo actual"}, {"### Future/proposed state", "### Estado futuro o propuesto"}, {"## References and attachments", "## Referencias y adjuntos"}, {"## Evidence", "## Evidencia"}, {"### Facts", "### Hechos"}, {"### Inferences", "### Inferencias"}, {"### Contradictions", "### Contradicciones"}, {"## Affected surfaces", "## Superficies afectadas"}, {"## Development handoffs", "## Handoffs de desarrollo"}, {"## Open questions", "## Preguntas abiertas"}, {"## Decisions", "## Decisiones"}, {"## Acceptance criteria", "## Criterios de aceptación"}, {"## Readiness", "## Preparación"}, {"## History", "## Historial"}}
		for _, p := range pairs {
			text = strings.ReplaceAll(text, p[0], p[1])
		}
	}
	return text
}
func openCase(root string, o options) (any, error) {
	if e := o.required("id", "title", "objective", "dedupe-key", "purpose", "vault-outcome", "learning-outcome", "request-summary"); e != nil {
		return nil, e
	}
	if !idPattern.MatchString(o.get("id")) || !keyPattern.MatchString(o.get("dedupe-key")) {
		return nil, errors.New("noncanonical ID or dedupe key")
	}
	o.defaultValue("source-type", "message")
	o.defaultValue("requester-role", "unknown")
	o.defaultValue("export-intent", "undecided")
	o.defaultValue("visibility", "unpublished")
	for _, k := range []string{"purpose", "vault-outcome", "learning-outcome"} {
		if !contains(allowedValues[k], o.get(k)) {
			return nil, fmt.Errorf("invalid %s", k)
		}
	}
	if !contains([]string{"message", "bug", "ticket", "issue", "attachment", "other"}, o.get("source-type")) || !contains([]string{"technical-stories", "user-stories", "mixed", "undecided"}, o.get("export-intent")) || !contains([]string{"published", "unpublished"}, o.get("visibility")) {
		return nil, errors.New("invalid creation option")
	}
	for _, k := range []string{"title", "objective", "request-summary", "source-ref"} {
		for _, v := range o[k] {
			if secretPattern.MatchString(v) {
				return nil, errors.New("secret-bearing creation input")
			}
		}
	}
	who, e := identity(root)
	if e != nil {
		return nil, e
	}
	ts, e := eventTime(o)
	if e != nil {
		return nil, e
	}
	return mutationGate(root, func() (any, error) {
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
		for _, m := range ret {
			if m["id"] == o.get("id") {
				return nil, errors.New("retired ID cannot be reused")
			}
			for _, v := range m["consolidated-from"].([]any) {
				if v == o.get("id") {
					return nil, errors.New("retired lineage cannot be reused")
				}
			}
		}
		for _, m := range old {
			if m["id"] == o.get("id") {
				return nil, errors.New("legacy unpublished case requires migration")
			}
		}
		refs := map[string]bool{}
		for _, v := range o["source-ref"] {
			v = strings.Join(strings.Fields(strings.ToLower(v)), " ")
			if !contains([]string{"", "unknown", "message", "customer conversation", "conversation"}, v) {
				refs[v] = true
			}
		}
		matches := []map[string]any{}
		for _, r := range rs {
			title := normalizeTitle(r.get("title"))
			ref := strings.Join(strings.Fields(strings.ToLower(r.get("source-ref"))), " ")
			if r.get("id") == o.get("id") || contains(r.lineage(), o.get("id")) || r.get("dedupe-key") == o.get("dedupe-key") || (title != "" && title == normalizeTitle(o.get("title"))) || refs[ref] {
				matches = append(matches, map[string]any{"id": r.get("id"), "path": filepath.Dir(r.path), "visibility": r.visibility})
			}
		}
		if len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool { return matches[i]["id"].(string) < matches[j]["id"].(string) })
			return map[string]any{"status": "definite_match", "cases": matches}, nil
		}
		store := root
		if o.get("visibility") == "unpublished" {
			store = filepath.Join(filepath.Dir(root), ".investigations")
		}
		target := filepath.Join(store, o.get("id"))
		if _, e := os.Lstat(target); !errors.Is(e, os.ErrNotExist) {
			return nil, errors.New("investigation target already exists")
		}
		staging, e := os.MkdirTemp(store, ".open-*.tmp")
		if e != nil {
			return nil, e
		}
		defer os.RemoveAll(staging)
		for _, d := range []string{"artifacts", "exports", "handoffs"} {
			if e = os.Mkdir(filepath.Join(staging, d), 0700); e != nil {
				return nil, e
			}
		}
		text := renderCase(root, o, ts, who)
		if e = atomicWrite(filepath.Join(staging, "investigation.md"), []byte(text)); e != nil {
			return nil, e
		}
		if e = os.Rename(staging, target); e != nil {
			return nil, e
		}
		return map[string]any{"status": "created", "id": o.get("id"), "path": target, "visibility": o.get("visibility")}, nil
	})
}
