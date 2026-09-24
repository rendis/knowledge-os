package syncflow

import (
	"bytes"
	"encoding/json"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type patchSection struct {
	OldMissing, NewMissing bool
	Old, New               []byte
}

var fullHunkRE = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(?: .*)?\r?\n?$`)
var shortOIDRE = regexp.MustCompile(`^[0-9a-f]{12}$`)
var repositoryPrefixRE = regexp.MustCompile(`^APP[0-9]{5}-`)
var processRE = regexp.MustCompile(`(?i)\b(?:la sync|la review|claims? (?:aceptad|rechazad)|por el gate|conocimiento durable adicional publicado)\b`)

func patchSections(raw []byte) (map[string]patchSection, error) {
	bad := func() (map[string]patchSection, error) { return nil, fail("patch-structure-invalid") }
	if !utf8.Valid(raw) {
		return bad()
	}
	lines := strings.SplitAfter(string(raw), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	normalized := []string{}
	for _, l := range lines {
		if strings.HasPrefix(l, "\\ No newline at end of file") {
			if len(normalized) == 0 {
				return bad()
			}
			normalized[len(normalized)-1] = strings.TrimRight(normalized[len(normalized)-1], "\r\n")
		} else {
			normalized = append(normalized, l)
		}
	}
	lines = normalized
	out := map[string]patchSection{}
	header := func(l string) string {
		v := strings.SplitN(strings.TrimRight(l[4:], "\r\n"), "\t", 2)[0]
		if v == "/dev/null" {
			return ""
		}
		if strings.HasPrefix(v, "a/") || strings.HasPrefix(v, "b/") {
			v = v[2:]
		}
		return v
	}
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		if !strings.HasPrefix(lines[i], "--- ") || i+2 >= len(lines) || !strings.HasPrefix(lines[i+1], "+++ ") {
			return bad()
		}
		old, newp := header(lines[i]), header(lines[i+1])
		p := newp
		if p == "" {
			p = old
		}
		if p == "" || relative(p, false) != nil || old != "" && newp != "" && old != newp {
			return bad()
		}
		if _, ok := out[p]; ok {
			return bad()
		}
		i += 2
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i >= len(lines) {
			return bad()
		}
		m := fullHunkRE.FindStringSubmatch(lines[i])
		if m == nil {
			return bad()
		}
		i++
		n := make([]int, 4)
		for j := range n {
			s := m[j+1]
			if s == "" {
				s = "1"
			}
			v, e := strconv.Atoi(s)
			if e != nil {
				return bad()
			}
			n[j] = v
		}
		start := func(count int) int {
			if count == 0 {
				return 0
			}
			return 1
		}
		if n[0] != start(n[1]) || n[2] != start(n[3]) {
			return bad()
		}
		section := patchSection{OldMissing: old == "", NewMissing: newp == ""}
		oc, nc := n[1], n[3]
		for oc > 0 || nc > 0 {
			if i >= len(lines) || len(lines[i]) == 0 {
				return bad()
			}
			l := lines[i]
			i++
			switch l[0] {
			case ' ':
				oc--
				nc--
				section.Old = append(section.Old, l[1:]...)
				section.New = append(section.New, l[1:]...)
			case '-':
				oc--
				section.Old = append(section.Old, l[1:]...)
			case '+':
				nc--
				if processRE.MatchString(l[1:]) {
					return nil, fail("sync-process-state-added")
				}
				section.New = append(section.New, l[1:]...)
			default:
				return bad()
			}
			if oc < 0 || nc < 0 {
				return bad()
			}
		}
		if section.OldMissing && len(section.Old) > 0 || section.NewMissing && len(section.New) > 0 {
			return bad()
		}
		out[p] = section
	}
	if len(out) == 0 {
		return bad()
	}
	return out, nil
}
func patchImages(raw []byte) (map[string][]byte, map[string][]byte, error) {
	s, e := patchSections(raw)
	if e != nil {
		return nil, nil, e
	}
	before, after := map[string][]byte{}, map[string][]byte{}
	for p, x := range s {
		before[p] = x.Old
		after[p] = x.New
	}
	return before, after, nil
}
func projectedImages(vault string, raw []byte) (map[string][]byte, map[string]bool, error) {
	s, e := patchSections(raw)
	if e != nil {
		return nil, nil, e
	}
	out := map[string][]byte{}
	deleted := map[string]bool{}
	for p, x := range s {
		b, exists, e := content(vault, p, true)
		if e != nil {
			return nil, nil, e
		}
		if exists == x.OldMissing || !bytes.Equal(b, x.Old) {
			return nil, nil, fail("projection-base-stale")
		}
		out[p] = x.New
		deleted[p] = x.NewMissing
	}
	return out, deleted, nil
}

const ackPath = "90-Meta/.sync-acknowledgements.json"

func acknowledgementDocument(raw []byte, strict bool) (map[string]any, error) {
	out := map[string]any{}
	if len(raw) == 0 {
		return out, nil
	}
	v, e := decode(raw)
	m := obj(v)
	if e != nil || !exact(m, "version repositories") || m["version"] != json.Number("1") {
		return nil, fail("acknowledgement-content-invalid")
	}
	records, ok := m["repositories"].([]any)
	if !ok {
		return nil, fail("acknowledgement-content-invalid")
	}
	prev := ""
	for _, v := range records {
		r := obj(v)
		n := str(r["repository"])
		date := str(r["analysis_date"])
		dt, e := time.Parse("2006-01-02", date)
		if !exact(r, "repository branch analyzed_sha decision analysis_date") || !basename(n) || !basename(repositoryPrefixRE.ReplaceAllString(n, "")) || !branch(str(r["branch"])) || !shortOIDRE.MatchString(str(r["analyzed_sha"])) || !nonempty(str(r["decision"])) || e != nil || dt.Format("2006-01-02") != date || out[n] != nil || strict && n <= prev {
			return nil, fail("acknowledgement-content-invalid")
		}
		out[n] = r
		prev = n
	}
	if strict {
		b, e := canonical(m)
		if e != nil || !bytes.Equal(raw, append(b, '\n')) {
			return nil, fail("acknowledgement-content-invalid")
		}
	}
	return out, nil
}
func validateProjection(gate, projection map[string]any, patch []byte) error {
	if e := validateGate(gate); e != nil {
		return e
	}
	if !exact(projection, "version run_id gate_digest unit_id unit_type patch_digest base_files result_files grants") || projection["version"] != json.Number("1") || !basename(str(projection["run_id"])) {
		return fail("projection-contract-invalid")
	}
	d, _ := digest(gate)
	if projection["gate_digest"] != d {
		return fail("projection-gate-digest-mismatch")
	}
	if projection["patch_digest"] != hash(patch) {
		return fail("projection-patch-digest-mismatch")
	}
	nodes, repos := map[string]bool{}, map[string]bool{}
	ack := projection["unit_type"] == "acknowledgements"
	if ack {
		if projection["unit_id"] != "acknowledgements" || len(arr(gate["acknowledgements"])) == 0 || !reflect.DeepEqual(projection["grants"], []any{}) {
			return fail("projection-unit-invalid")
		}
	} else if projection["unit_type"] == "write-group" {
		found := false
		for _, v := range arr(gate["write_groups"]) {
			g := obj(v)
			if g["group_id"] == projection["unit_id"] {
				found = true
				if !reflect.DeepEqual(g["grants"], projection["grants"]) {
					return fail("projection-grants-invalid")
				}
				for _, n := range arr(g["nodes"]) {
					nodes[str(n)] = true
				}
				for _, r := range arr(g["repositories"]) {
					repos[str(r)] = true
				}
			}
		}
		if !found {
			return fail("projection-unit-invalid")
		}
	} else {
		return fail("projection-unit-invalid")
	}
	sections, e := patchSections(patch)
	if e != nil {
		return e
	}
	base, result := obj(projection["base_files"]), obj(projection["result_files"])
	if len(base) != len(sections) || len(result) != len(sections) {
		return fail("projection-file-binding-invalid")
	}
	covered := map[string]bool{}
	for p, s := range sections {
		if base[p] != hash(s.Old) || result[p] != hash(s.New) {
			return fail("projection-file-binding-invalid")
		}
		if p == ackPath {
			continue
		}
		stem := strings.TrimSuffix(path.Base(p), path.Ext(p))
		if ack || relative(p, true) != nil || !nodes[stem] {
			return fail("projection-path-not-authorized")
		}
		covered[stem] = true
		lines := strings.Split(string(s.New), "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
			for _, l := range lines[1:] {
				if strings.TrimSpace(l) == "---" {
					break
				}
				kv := strings.SplitN(l, ":", 2)
				if len(kv) != 2 {
					continue
				}
				switch kv[0] {
				case "cobertura-entradas", "cobertura-salidas", "cobertura-datos", "cobertura-infra", "cobertura-flujos":
					v := strings.Trim(strings.TrimSpace(strings.SplitN(kv[1], " #", 2)[0]), "\"'")
					if !one(v, "completo", "parcial", "no-aplica", "por-confirmar") {
						return fail("projection-frontmatter-invalid")
					}
				}
			}
		}
	}
	if !ack && !reflect.DeepEqual(nodes, covered) {
		return fail("projection-node-coverage-invalid")
	}
	if s, ok := sections[ackPath]; ok {
		prior, e := acknowledgementDocument(s.Old, false)
		if e != nil {
			return e
		}
		observed, e := acknowledgementDocument(s.New, true)
		if e != nil {
			return e
		}
		if ack {
			for _, v := range arr(gate["acknowledgements"]) {
				r := obj(v)
				n := str(r["repository"])
				o := obj(observed[n])
				oid := str(r["new_oid"])
				if o == nil || o["branch"] != r["branch"] || o["analyzed_sha"] != oid[:12] || o["decision"] != r["decision"] || o["analysis_date"] != r["analysis_date"] {
					return fail("acknowledgement-content-invalid")
				}
				prior[n] = o
			}
		} else {
			for _, v := range arr(gate["repositories"]) {
				r := obj(v)
				n := str(r["repository"])
				if repos[n] && obj(prior[n])["analyzed_sha"] == str(r["new_oid"])[:12] {
					delete(prior, n)
				}
			}
		}
		if !reflect.DeepEqual(prior, observed) {
			return fail("acknowledgement-content-invalid")
		}
	} else if ack {
		return fail("acknowledgement-content-invalid")
	}
	return nil
}
