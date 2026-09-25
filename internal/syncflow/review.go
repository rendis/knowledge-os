// Package syncflow binds reviewed note images to their exact evidence and base.
// It does not perform semantic review or grant authority to publish.
package syncflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
var connectionRE = regexp.MustCompile(`<!--\s*connection:(connection\.[A-Za-z0-9][A-Za-z0-9._-]*)\s*-->`)
var idRE = regexp.MustCompile(`^connection\.[A-Za-z0-9][A-Za-z0-9._-]*$`)
var roots = map[string]bool{"10-Sistemas": true, "15-Arquitectura": true, "20-Repos": true, "25-Topics": true, "30-Flujos": true, "40-Integraciones": true, "50-Glosario": true, "60-Operacion": true, "70-Aprendizajes": true}

func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

var emptyHash = hash(nil)

func fail(s string) error { return errors.New(s) }

// canonical matches Python json.dumps(ensure_ascii=True,sort_keys=True,separators=(',',':')).
func canonical(v any) ([]byte, error) {
	var b bytes.Buffer
	var emit func(any) error
	emit = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			b.WriteByte('{')
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for i, k := range keys {
				if i > 0 {
					b.WriteByte(',')
				}
				if err := emit(k); err != nil {
					return err
				}
				b.WriteByte(':')
				if err := emit(x[k]); err != nil {
					return err
				}
			}
			b.WriteByte('}')
		case []any:
			b.WriteByte('[')
			for i, e := range x {
				if i > 0 {
					b.WriteByte(',')
				}
				if err := emit(e); err != nil {
					return err
				}
			}
			b.WriteByte(']')
		case string:
			var s bytes.Buffer
			enc := json.NewEncoder(&s)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(x); err != nil {
				return err
			}
			raw := strings.TrimSuffix(s.String(), "\n")
			for _, r := range raw {
				if r < 128 {
					b.WriteRune(r)
				} else if r <= 0xffff {
					fmt.Fprintf(&b, "\\u%04x", r)
				} else {
					a, c := utf16.EncodeRune(r)
					fmt.Fprintf(&b, "\\u%04x\\u%04x", a, c)
				}
			}
		case json.Number:
			if _, err := x.Int64(); err != nil {
				return fail("non-integral-json-number")
			}
			b.WriteString(x.String())
		case bool:
			fmt.Fprint(&b, x)
		case nil:
			b.WriteString("null")
		case int:
			fmt.Fprint(&b, x)
		default:
			return fail("unsupported-json-value")
		}
		return nil
	}
	if err := emit(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func digest(v any) (string, error) { b, e := canonical(v); return hash(b), e }
func decode(data []byte) (any, error) {
	if !utf8.Valid(data) {
		return nil, fail("invalid-json")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var read func() (any, error)
	read = func() (any, error) {
		t, e := d.Token()
		if e != nil {
			return nil, e
		}
		if z, ok := t.(json.Delim); ok {
			switch z {
			case '{':
				m := map[string]any{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return nil, e
					}
					s, ok := k.(string)
					if !ok {
						return nil, fail("invalid-json")
					}
					if _, ok := m[s]; ok {
						return nil, fail("duplicate-json-key")
					}
					v, e := read()
					if e != nil {
						return nil, e
					}
					m[s] = v
				}
				_, e = d.Token()
				return m, e
			case '[':
				a := []any{}
				for d.More() {
					v, e := read()
					if e != nil {
						return nil, e
					}
					a = append(a, v)
				}
				_, e = d.Token()
				return a, e
			default:
				return nil, fail("invalid-json")
			}
		}
		return t, nil
	}
	v, e := read()
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, fail("trailing-json")
	}
	return v, nil
}
func safe(p string, directory bool) (string, error) {
	if p == "" {
		return "", fail("missing-path")
	}
	for _, s := range strings.Split(filepath.ToSlash(p), "/") {
		if s == ".." {
			return "", fail("unsafe-path")
		}
	}
	a, e := filepath.Abs(p)
	if e != nil {
		return "", e
	}
	vol := filepath.VolumeName(a)
	cur := vol + string(os.PathSeparator)
	parts := strings.Split(strings.TrimPrefix(a, cur), string(os.PathSeparator))
	for i, s := range parts {
		cur = filepath.Join(cur, s)
		st, e := os.Lstat(cur)
		if e != nil {
			return "", e
		}
		if st.Mode()&os.ModeSymlink != 0 && !(i == 0 && (cur == "/var" || cur == "/tmp")) {
			return "", fail("unsafe-path")
		}
	}
	st, e := os.Stat(a)
	if e != nil {
		return "", e
	}
	if directory && !st.IsDir() {
		return "", fail("root-not-directory")
	}
	return a, nil
}
func relative(s string, note bool) error {
	p := strings.Split(s, "/")
	if s == "" || strings.ContainsAny(s, "\\:\x00") || filepath.IsAbs(s) {
		return fail("unsafe-relative-path")
	}
	for _, v := range p {
		if v == "" || v == "." || v == ".." {
			return fail("unsafe-relative-path")
		}
	}
	if note && (len(p) < 2 || !roots[p[0]] || filepath.Ext(s) != ".md") {
		return fail("candidate-path-not-authorized")
	}
	return nil
}
func content(root, name string, missing bool) ([]byte, bool, error) {
	if e := relative(name, false); e != nil {
		return nil, false, e
	}
	p := root
	parts := strings.Split(name, "/")
	for i, s := range parts {
		p = filepath.Join(p, s)
		st, e := os.Lstat(p)
		if os.IsNotExist(e) && missing {
			return nil, false, nil
		}
		if e != nil {
			return nil, false, e
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, false, fail("unsafe-path")
		}
		if i == len(parts)-1 && !st.Mode().IsRegular() {
			return nil, false, fail("missing-or-invalid-file")
		}
	}
	b, e := os.ReadFile(p)
	return b, e == nil, e
}
func connections(b []byte) ([]any, error) {
	if !utf8.Valid(b) {
		return nil, fail("candidate-not-utf8")
	}
	m := map[string]bool{}
	for _, v := range connectionRE.FindAllSubmatch(b, -1) {
		k := string(v[1])
		if m[k] {
			return nil, fail("duplicate-connection-anchor")
		}
		m[k] = true
	}
	return sortedKeys(m), nil
}
func sortedKeys[T any](m map[string]T) []any {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	a := make([]any, len(s))
	for i, k := range s {
		a[i] = k
	}
	return a
}
func readJSON(p string) (any, error) {
	p, e := safe(p, false)
	if e != nil {
		return nil, e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, e
	}
	return decode(b)
}
func freeze(vault, candidate, evidence string, ev, deleted []string, projection string) (map[string]any, error) {
	var e error
	vault, e = safe(vault, true)
	if e != nil {
		return nil, e
	}
	candidate, e = safe(candidate, true)
	if e != nil {
		return nil, e
	}
	evidence, e = safe(evidence, true)
	if e != nil {
		return nil, e
	}
	files := map[string]any{}
	bases := map[string]any{}
	images := map[string]any{}
	present := map[string]bool{}
	del := map[string]bool{}
	for _, n := range deleted {
		if e = relative(n, true); e != nil {
			return nil, e
		}
		del[n] = true
	}
	e = filepath.WalkDir(candidate, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fail("unsafe-path")
		}
		if d.IsDir() {
			return nil
		}
		n, e := filepath.Rel(candidate, p)
		if e != nil {
			return e
		}
		n = filepath.ToSlash(n)
		if e = relative(n, true); e != nil {
			return e
		}
		if del[n] {
			return fail("candidate-delete-overlap")
		}
		b, _, e := content(candidate, n, false)
		if e != nil {
			return e
		}
		files[n] = hash(b)
		return nil
	})
	if e != nil {
		return nil, e
	}
	for n := range del {
		files[n] = emptyHash
	}
	if len(files) == 0 {
		return nil, fail("empty-candidate")
	}
	for n := range files {
		b, exists, e := content(vault, n, true)
		if e != nil {
			return nil, e
		}
		if del[n] && !exists {
			return nil, fail("delete-base-missing")
		}
		bases[n] = hash(b)
		if exists {
			present[n] = true
		}
		before, e := connections(b)
		if e != nil {
			return nil, e
		}
		after := []any{}
		if !del[n] {
			b, _, e = content(candidate, n, false)
			if e != nil {
				return nil, e
			}
			after, e = connections(b)
			if e != nil {
				return nil, e
			}
		}
		images[n] = map[string]any{"base": before, "candidate": after}
	}
	evidenceFiles := map[string]any{}
	for _, n := range ev {
		if _, ok := evidenceFiles[n]; ok {
			return nil, fail("duplicate-evidence")
		}
		b, _, e := content(evidence, n, false)
		if e != nil {
			return nil, e
		}
		evidenceFiles[n] = hash(b)
	}
	if len(evidenceFiles) == 0 {
		return nil, fail("empty-evidence")
	}
	m := map[string]any{"version": json.Number("1"), "candidate_files": files, "base_files": bases, "base_present": sortedKeys(present), "deleted_files": sortedKeys(del), "evidence_files": evidenceFiles, "connections": images}
	if projection != "" {
		p, e := readJSON(projection)
		if e != nil {
			return nil, e
		}
		obj, ok := p.(map[string]any)
		if !ok {
			return nil, fail("projection-invalid")
		}
		base, ok := obj["base_files"].(map[string]any)
		if !ok {
			return nil, fail("projection-invalid")
		}
		result, ok := obj["result_files"].(map[string]any)
		if !ok {
			return nil, fail("projection-invalid")
		}
		for _, mapping := range []map[string]any{base, result} {
			for _, value := range mapping {
				d, ok := value.(string)
				if !ok || !digestRE.MatchString(d) {
					return nil, fail("projection-invalid")
				}
			}
		}
		if obj["unit_type"] == "write-group" {
			const ack = "90-Meta/.sync-acknowledgements.json"
			_, a := base[ack]
			_, b := result[ack]
			if a != b {
				return nil, fail("projection-invalid")
			}
			base = clone(base)
			result = clone(result)
			delete(base, ack)
			delete(result, ack)
		}
		if !reflect.DeepEqual(base, bases) || !reflect.DeepEqual(result, files) {
			return nil, fail("projection-file-binding-invalid")
		}
		dg, e := digest(p)
		if e != nil {
			return nil, e
		}
		m["projection_digest"] = dg
	}
	return m, nil
}
func clone(m map[string]any) map[string]any {
	n := map[string]any{}
	for k, v := range m {
		n[k] = v
	}
	return n
}
func stringsFrom(v any) ([]string, error) {
	a, ok := v.([]any)
	if !ok {
		return nil, fail("manifest-invalid")
	}
	r := []string{}
	for _, v := range a {
		s, ok := v.(string)
		if !ok {
			return nil, fail("manifest-invalid")
		}
		r = append(r, s)
	}
	return r, nil
}
func manifest(v any) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fail("manifest-invalid")
	}
	allowed := map[string]bool{"version": true, "candidate_files": true, "base_files": true, "base_present": true, "deleted_files": true, "evidence_files": true, "connections": true, "projection_digest": true, "source_bindings": true, "acknowledgements": true}
	version := m["version"]
	if version != json.Number("1") && version != json.Number("2") {
		return nil, fail("manifest-invalid")
	}
	want := 7
	if _, ok := m["projection_digest"]; ok {
		want++
	}
	if version == json.Number("2") {
		if _, exists := m["projection_digest"]; exists {
			return nil, fail("manifest-invalid")
		}
		want += 2
	}
	if len(m) != want {
		return nil, fail("manifest-invalid")
	}
	for k := range m {
		if !allowed[k] {
			return nil, fail("manifest-invalid")
		}
	}
	for _, f := range []string{"candidate_files", "base_files", "evidence_files"} {
		x, ok := m[f].(map[string]any)
		if !ok || len(x) == 0 && (version == json.Number("1") || f == "evidence_files") {
			return nil, fail("manifest-invalid")
		}
		for n, d := range x {
			s, ok := d.(string)
			if !ok || !digestRE.MatchString(s) || relative(n, f != "evidence_files") != nil {
				return nil, fail("manifest-invalid")
			}
		}
	}
	c := m["candidate_files"].(map[string]any)
	b := m["base_files"].(map[string]any)
	im, ok := m["connections"].(map[string]any)
	if !ok || !reflect.DeepEqual(sortedKeys(c), sortedKeys(b)) || !reflect.DeepEqual(sortedKeys(c), sortedKeys(im)) {
		return nil, fail("manifest-invalid")
	}
	sets := map[string]map[string]bool{}
	for _, f := range []string{"base_present", "deleted_files"} {
		a, e := stringsFrom(m[f])
		if e != nil {
			return nil, e
		}
		s := map[string]bool{}
		last := ""
		for _, n := range a {
			if _, ok := c[n]; !ok || n <= last {
				return nil, fail("manifest-invalid")
			}
			last = n
			s[n] = true
		}
		sets[f] = s
	}
	for n := range sets["deleted_files"] {
		if !sets["base_present"][n] || c[n] != emptyHash {
			return nil, fail("manifest-invalid")
		}
	}
	for _, v := range im {
		r, ok := v.(map[string]any)
		if !ok || len(r) != 2 {
			return nil, fail("manifest-invalid")
		}
		for _, f := range []string{"base", "candidate"} {
			a, e := stringsFrom(r[f])
			if e != nil {
				return nil, e
			}
			last := ""
			for _, id := range a {
				if id <= last || !idRE.MatchString(id) {
					return nil, fail("manifest-invalid")
				}
				last = id
			}
		}
	}
	if d, ok := m["projection_digest"]; ok {
		s, ok := d.(string)
		if !ok || !digestRE.MatchString(s) {
			return nil, fail("manifest-invalid")
		}
	}
	if version == json.Number("2") {
		if e := validateSourceBindings(m["source_bindings"]); e != nil {
			return nil, e
		}
		if e := validateAcknowledgementBindings(obj(m["acknowledgements"]), arr(m["source_bindings"])); e != nil {
			return nil, e
		}
	}
	return m, nil
}
func review(v any, m map[string]any) error {
	r, ok := v.(map[string]any)
	if !ok {
		return fail("review-invalid")
	}
	v2 := m["version"] == json.Number("2")
	fields := []string{"version", "manifest_digest", "verdict", "findings", "connection_decisions"}
	if v2 {
		fields = []string{"version", "manifest_digest", "verdict", "findings", "retired_connections"}
	}
	if len(r) != len(fields) {
		return fail("review-invalid")
	}
	for _, k := range fields {
		if _, ok := r[k]; !ok {
			return fail("review-invalid")
		}
	}
	if !v2 && r["version"] != json.Number("1") || v2 && r["version"] != json.Number("2") {
		return fail("review-invalid")
	}
	d, e := digest(m)
	if e != nil {
		return e
	}
	if r["manifest_digest"] != d {
		return fail("review-stale")
	}
	f, ok := r["findings"].([]any)
	if !ok {
		return fail("review-invalid")
	}
	if r["verdict"] != "accept" {
		return fail("review-requires-revision")
	}
	if len(f) != 0 {
		return fail("review-invalid")
	}
	if v2 {
		retired, ok := r["retired_connections"].(map[string]any)
		if !ok {
			return fail("connection-decisions-invalid")
		}
		expected := map[string]bool{}
		for p, v := range m["connections"].(map[string]any) {
			im := v.(map[string]any)
			before, _ := stringsFrom(im["base"])
			after, _ := stringsFrom(im["candidate"])
			keep := map[string]bool{}
			for _, id := range after {
				keep[id] = true
			}
			for _, id := range before {
				if !keep[id] {
					expected[p+"#"+id] = true
				}
			}
		}
		if !reflect.DeepEqual(sortedKeys(expected), sortedKeys(retired)) {
			return fail("connection-decisions-invalid")
		}
		for _, reason := range retired {
			if s, ok := reason.(string); !ok || strings.TrimSpace(s) == "" {
				return fail("connection-decisions-invalid")
			}
		}
		return nil
	}
	dec, ok := r["connection_decisions"].(map[string]any)
	if !ok {
		return fail("connection-decisions-invalid")
	}
	count := 0
	for p, v := range m["connections"].(map[string]any) {
		im := v.(map[string]any)
		before, _ := stringsFrom(im["base"])
		after, _ := stringsFrom(im["candidate"])
		s := map[string]int{}
		for _, id := range before {
			s[id] |= 1
		}
		for _, id := range after {
			s[id] |= 2
		}
		for id, mode := range s {
			count++
			x, ok := dec[p+"#"+id].(map[string]any)
			if !ok || len(x) != 2 {
				return fail("connection-decisions-invalid")
			}
			reason, ok := x["reason"].(string)
			a := x["action"]
			if !ok || strings.TrimSpace(reason) == "" || !(mode == 1 && a == "retire" || mode == 2 && a == "create" || mode == 3 && (a == "preserve" || a == "update")) {
				return fail("connection-decisions-invalid")
			}
		}
	}
	if count != len(dec) {
		return fail("connection-decisions-invalid")
	}
	return nil
}

type repeated []string

func (r *repeated) String() string     { return strings.Join(*r, ",") }
func (r *repeated) Set(v string) error { *r = append(*r, v); return nil }

// Run executes native synchronization and content-bound review commands.
func Run(args []string, out io.Writer) error {
	if len(args) == 0 || one(args[0], "--help", "-h", "help") {
		_, err := fmt.Fprintln(out, `Usage: vaultctl sync <command> [options]

Discovery: scan
Packages: build, build-new, init-analysis, close-package, gate-batch
Analysis: analysis finalize-analysis|check
Review: review freeze|check|publish|verify-published, review-finalize
Correction: correction prepare|check
Lifecycle: begin, checkpoint-package, seal-gate, status, validate-unit,
           review-unit, apply-unit, resume, close, tool-digest

Follow synchronize-ecosystem and its command contracts for required arguments,
review and mutation prerequisites. This help does not read or modify vault state.`)
		return err
	}
	if len(args) > 0 {
		switch args[0] {
		case "scan":
			return runScan(args, out)
		case "build", "build-new", "init-analysis":
			return runSource(args, out)
		case "analysis":
			return runAnalysis(args[1:], out)
		case "close-package", "gate-batch":
			return runGate(args, out)
		case "review-finalize":
			return runCorrection(args, out)
		case "correction":
			return runCorrection(args[1:], out)
		}
	}
	if len(args) > 0 && one(args[0], "begin", "status", "checkpoint-package", "seal-gate", "tool-digest", "validate-unit", "review-unit", "apply-unit", "resume", "close") {
		return runState(args, out)
	}
	if len(args) > 0 && args[0] == "review" {
		args = args[1:]
	}
	if len(args) == 0 {
		return fail("usage: sync review freeze|check|verify-published")
	}
	cmd := args[0]
	if cmd == "verify-published" {
		return runPublished(args, out)
	}
	if cmd != "freeze" && cmd != "check" && cmd != "publish" && cmd != "verify-published" {
		return fmt.Errorf("unknown sync operation %q; use vaultctl sync --help; no state changed", cmd)
	}
	var sources []directSourceArg
	var checkouts map[string]string
	var e error
	args, sources, checkouts, e = directArguments(args)
	if e != nil {
		return e
	}
	if cmd == "freeze" && len(checkouts) > 0 || cmd != "freeze" && len(sources) > 0 {
		return fail("direct-arguments-invalid")
	}
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	vault := f.String("vault", "", "vault")
	candidate := f.String("candidate", "", "candidate")
	evidence := f.String("evidence-root", "", "evidence")
	projection := f.String("projection", "", "projection")
	output := f.String("output", "", "output")
	mp := f.String("manifest", "", "manifest")
	rp := f.String("review", "", "review")
	stateRoot := f.String("state-root", defaultStateRoot, "state root")
	analysisDate := f.String("analysis-date", "", "analysis date")
	var ev, del repeated
	f.Var(&ev, "evidence", "evidence path")
	f.Var(&del, "delete", "deleted note")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return fail("unexpected-arguments")
	}
	var result any
	switch cmd {
	case "freeze":
		var m map[string]any
		var issues []any
		var e error
		if len(sources) > 0 {
			m, issues, e = freezeBound(*vault, *candidate, *evidence, ev, del, *projection, *analysisDate, sources)
		} else {
			m, e = freeze(*vault, *candidate, *evidence, ev, del, *projection)
		}
		if e != nil {
			return e
		}
		if len(issues) > 0 {
			result = map[string]any{"status": "blocked", "code": "output-scan-blocked", "issues": issues}
			break
		}
		if *output == "" {
			return fail("output-required")
		}
		parent, e := safe(filepath.Dir(*output), true)
		if e != nil {
			return e
		}
		target := filepath.Join(parent, filepath.Base(*output))
		if filepath.Base(target) == "." {
			return fail("invalid-output")
		}
		b, e := json.MarshalIndent(m, "", "  ")
		if e != nil {
			return e
		}
		tmp, e := os.CreateTemp(parent, ".note-review-*")
		if e != nil {
			return e
		}
		name := tmp.Name()
		defer os.Remove(name)
		if _, e = tmp.Write(append(b, '\n')); e == nil {
			e = tmp.Sync()
		}
		ce := tmp.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		if e = os.Link(name, target); e != nil {
			return fmt.Errorf("output must be new: %w", e)
		}
		d, _ := digest(m)
		result = map[string]any{"status": "pass", "code": "note-candidate-frozen", "manifest": target, "manifest_digest": d}
	case "check", "publish":
		v, e := readJSON(*mp)
		if e != nil {
			return e
		}
		m, e := manifest(v)
		if e != nil {
			return e
		}
		r, e := readJSON(*rp)
		if e != nil {
			return e
		}
		if m["version"] == json.Number("2") {
			if *projection != "" {
				return fail("direct-projection-not-supported")
			}
			if cmd == "publish" {
				result, e = directPublish(*stateRoot, *vault, *candidate, *evidence, *projection, m, r, checkouts, 0, 0)
				if e != nil {
					return e
				}
				break
			}
			issues, e := checkBound(m, r, *vault, *candidate, *evidence, *projection, checkouts)
			if e != nil {
				return e
			}
			if len(issues) > 0 {
				result = map[string]any{"status": "blocked", "code": "output-scan-blocked", "issues": issues}
				break
			}
			d, _ := digest(m)
			result = map[string]any{"status": "pass", "code": "note-candidate-reviewed", "manifest_digest": d}
			break
		}
		if cmd == "publish" {
			return fail("direct-manifest-required")
		}
		if e = review(r, m); e != nil {
			return e
		}
		evidencePaths := []string{}
		for n := range m["evidence_files"].(map[string]any) {
			evidencePaths = append(evidencePaths, n)
		}
		deletedPaths, _ := stringsFrom(m["deleted_files"])
		current, e := freeze(*vault, *candidate, *evidence, evidencePaths, deletedPaths, *projection)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(current, m) {
			return fail("candidate-base-or-evidence-stale")
		}
		d, _ := digest(m)
		result = map[string]any{"status": "pass", "code": "note-candidate-reviewed", "manifest_digest": d}
	}
	return json.NewEncoder(out).Encode(result)
}
