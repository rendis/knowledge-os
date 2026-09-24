// Package audit validates the installed vault's structural document contracts.
package audit

import (
	"documentation-vault/internal/config"
	"errors"
	"flag"
	"fmt"
	"go.yaml.in/yaml/v3"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type note struct {
	path, body, raw string
	f               map[string]any
}
type auditor struct {
	root                string
	issues              []string
	systems, systemTags []string
	contracts           map[string][]string
	areas               map[string]string
}

func s(v any) string          { x, _ := v.(string); return x }
func arr(v any) ([]any, bool) { x, ok := v.([]any); return x, ok }
func has(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func contains(v any, x string) bool {
	a, _ := arr(v)
	for _, z := range a {
		if s(z) == x {
			return true
		}
	}
	return false
}
func rx(p, x string) bool { return regexp.MustCompile(p).MatchString(x) }
func date(v any) bool {
	x := s(v)
	t, e := time.Parse("2006-01-02", x)
	return e == nil && t.Format("2006-01-02") == x
}
func stem(p string) string { return strings.TrimSuffix(filepath.Base(p), ".md") }
func (a *auditor) issue(n note, format string, args ...any) {
	a.issues = append(a.issues, n.path+": "+fmt.Sprintf(format, args...))
}
func read(root, p string) (note, error) {
	b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	n := note{path: p, raw: string(b), body: string(b), f: map[string]any{}}
	if e != nil {
		return n, e
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" {
				var node yaml.Node
				if e = yaml.Unmarshal([]byte(strings.Join(lines[1:i], "\n")), &node); e != nil {
					return n, e
				}
				if len(node.Content) > 0 { // Preserve textual scalar semantics of the existing frontmatter contract.
					var normalize func(*yaml.Node)
					normalize = func(v *yaml.Node) {
						if v.Kind == yaml.ScalarNode && v.Tag == "!!null" && v.Value == "" {
							v.Kind = yaml.SequenceNode
							v.Tag = "!!seq"
						}
						if v.Kind == yaml.ScalarNode && v.Tag != "!!null" {
							v.Tag = "!!str"
						}
						for _, c := range v.Content {
							normalize(c)
						}
					}
					normalize(&node)
					if e = node.Decode(&n.f); e != nil {
						return n, e
					}
				}
				n.body = strings.Join(lines[i+1:], "\n")
				break
			}
		}
	}
	return n, nil
}
func headings(body string) []string {
	r := []string{}
	for _, m := range regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`).FindAllStringSubmatch(body, -1) {
		r = append(r, m[1])
	}
	return r
}
func section(body, title string) string {
	lines := strings.Split(body, "\n")
	active := false
	r := []string{}
	for _, l := range lines {
		if rx(`^##\s+`, l) {
			if active {
				break
			}
			active = strings.TrimSpace(strings.TrimPrefix(l, "##")) == title
			continue
		}
		if active {
			r = append(r, l)
		}
	}
	return strings.Join(r, "\n")
}
func (a *auditor) fields(n note, required, allowed []string) {
	for _, k := range required {
		if _, ok := n.f[k]; !ok {
			a.issue(n, "missing frontmatter field %s", k)
		}
	}
	ks := []string{}
	for k := range n.f {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		if !has(allowed, k) {
			a.issue(n, "unexpected frontmatter field %s", k)
		}
	}
}
func (a *auditor) sections(n note, req []string) {
	h := headings(n.body)
	for _, k := range req {
		if !has(h, k) {
			a.issue(n, "missing section %s", k)
		}
	}
}
func (a *auditor) lists(n note, keys []string, optional bool) {
	for _, k := range keys {
		v, exists := n.f[k]
		if !exists && optional {
			continue
		}
		if _, ok := arr(v); !ok {
			a.issue(n, "%s must be a list", k)
		}
	}
}
func (a *auditor) dates(n note, keys ...string) {
	for _, k := range keys {
		if !date(n.f[k]) {
			a.issue(n, "%s must be a valid YYYY-MM-DD date", k)
		}
	}
}
func (a *auditor) kind(n note, kind string) {
	if s(n.f["tipo"]) != kind {
		a.issue(n, "tipo must be %s", kind)
	}
}
func (a *auditor) tag(n note, t string) {
	if _, ok := arr(n.f["tags"]); ok && !contains(n.f["tags"], t) {
		a.issue(n, "tags must contain %s", t)
	}
}
func (a *auditor) system(n note) {
	if !has(a.systems, s(n.f["sistema"])) {
		a.issue(n, "sistema must be one of %v", a.systems)
	}
}
func (a *auditor) check(n note, group string) {
	f := n.f
	kind := s(f["tipo"])
	switch group {
	case "INDEX_NOTES":
		a.fields(n, INDEX_FIELDS, INDEX_FIELDS)
		a.kind(n, "indice")
		a.lists(n, []string{"tags"}, false)
		a.tag(n, "moc")
		if n.path == "70-Aprendizajes/Aprendizajes.md" {
			a.tag(n, "aprendizaje")
		}
	case "SYSTEM_NOTES":
		a.fields(n, SYSTEM_FIELDS, SYSTEM_FIELDS)
		a.kind(n, "sistema")
		a.lists(n, []string{"aliases", "tags"}, false)
		c, ok := a.contracts[stem(n.path)]
		if !ok {
			a.issue(n, "unsupported system note %s", stem(n.path))
			return
		}
		if c[0] != "" {
			if _, ok := arr(f["aliases"]); ok && !contains(f["aliases"], c[0]) {
				a.issue(n, "aliases must contain %s", c[0])
			}
		}
		a.tag(n, c[1])
		a.tag(n, "moc")
	case "REPO_NOTES":
		a.fields(n, REPO_REQUIRED, REPO_REQUIRED)
		a.sections(n, REPO_SECTIONS)
		a.lists(n, REPO_LIST_FIELDS, true)
		a.system(n)
		if !has(REPO_TYPES, kind) {
			a.issue(n, "invalid repo tipo %v", f["tipo"])
		}
		for _, k := range COVERAGE_FIELDS {
			if !has(COVERAGE_VALUES, s(f[k])) {
				a.issue(n, "invalid %s value %v", k, f[k])
			}
		}
		if !rx(`^[0-9a-f]{12}$`, s(f["commit-analizado"])) {
			a.issue(n, "commit-analizado is not a 12-char lowercase sha")
		}
		if !branch(s(f["rama-analizada"])) {
			a.issue(n, "rama-analizada must be a valid Git branch name")
		}
		a.dates(n, "fecha-analisis", "ultima-auditoria")
	case "ARCHITECTURE_NOTES":
		a.fields(n, ARCH_COMMON, ARCH_ALLOWED)
		req, ok := ARCH_SECTIONS[kind]
		if !ok {
			a.issue(n, "invalid architecture tipo %v", f["tipo"])
			return
		}
		a.sections(n, req)
		a.system(n)
		a.dates(n, "ultima-auditoria")
		a.lists(n, []string{"participa-en", "tags"}, false)
		forbidden := []string{}
		switch kind {
		case "servicio":
			v, ok := arr(f["compuesto-por"])
			if !ok || len(v) < 2 {
				a.issue(n, "servicio compuesto-por must contain at least two nodes")
			}
			forbidden = append(append(append([]string{}, ARCH_COMPONENT...), ARCH_RUNTIME_REQUIRED...), ARCH_RUNTIME_OPTIONAL...)
		case "componente":
			v, ok := arr(f["implementado-por"])
			if !ok || len(v) != 1 {
				a.issue(n, "componente implementado-por must contain exactly one repo")
			}
			forbidden = append(append(append([]string{}, ARCH_SERVICE...), ARCH_RUNTIME_REQUIRED...), ARCH_RUNTIME_OPTIONAL...)
		default:
			for _, k := range ARCH_RUNTIME_REQUIRED {
				if _, ok := f[k]; !ok {
					a.issue(n, "missing runtime field %s", k)
				}
			}
			if _, ok := f["ultima-verificacion"]; ok {
				a.dates(n, "ultima-verificacion")
			}
			a.lists(n, []string{"ejecuta", "gatilla-a", "usa-infra"}, true)
			forbidden = append(append([]string{}, ARCH_SERVICE...), ARCH_COMPONENT...)
		}
		for _, k := range forbidden {
			if _, ok := f[k]; ok {
				a.issue(n, "%s is not allowed for tipo %s", k, kind)
			}
		}
	case "TOPIC_NOTES":
		a.fields(n, TOPIC_FIELDS, TOPIC_FIELDS)
		a.sections(n, TOPIC_SECTIONS)
		a.kind(n, "topic")
		for _, h := range headings(n.body) {
			if has(TOPIC_FORBIDDEN_HEADINGS, h) {
				a.issue(n, "forbidden manual relationship section %s", h)
			}
		}
	case "INTEGRATION_NOTES":
		a.fields(n, []string{"tipo", "tags"}, INTEGRATION_FIELDS)
		a.sections(n, INTEGRATION_SECTIONS)
		a.kind(n, "integracion-externa")
	case "FLOW_NOTES":
		a.fields(n, FLOW_FIELDS, FLOW_FIELDS)
		a.sections(n, FLOW_SECTIONS)
		a.kind(n, "flujo")
		if len(regexp.MustCompile("(?m)^```mermaid\\s*$").FindAllString(n.body, -1)) != 2 {
			a.issue(n, "flow must contain exactly two Mermaid diagrams")
		}
		part := section(n.body, "Diagrama de componentes")
		if !strings.Contains(part, "```mermaid") || !rx(`(?m)^\s*subgraph\b`, part) {
			a.issue(n, "component diagram must be Mermaid and contain subgraph")
		}
	case "GLOSSARY_NOTES":
		a.fields(n, GLOSSARY_REQUIRED, GLOSSARY_ALLOWED)
		a.kind(n, "glosario")
		a.lists(n, []string{"tags"}, false)
		a.lists(n, []string{"aliases"}, true)
		a.tag(n, "glosario")
		if !rx(`\[\[[^\]]+\]\]`, n.body) {
			a.issue(n, "glossary must contain at least one wikilink")
		}
	case "OPERATIONAL_NOTES":
		a.operational(n)
	case "LEARNING_NOTES":
		a.learning(n)
	}
}
func branch(s string) bool {
	if s == "" || s == "@" || s == "HEAD" || strings.HasPrefix(s, "refs/") || strings.HasPrefix(s, "-") || strings.HasSuffix(s, ".") || strings.Contains(s, "..") || strings.Contains(s, "@{") || rx(`[\x00-\x20\x7f~^:?*\[\\]`, s) {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".lock") {
			return false
		}
	}
	return true
}
func (a *auditor) operational(n note) {
	f := n.f
	k := s(f["clase"])
	a.fields(n, OPERATIONAL_COMMON_REQUIRED, OPERATIONAL_ALLOWED)
	a.kind(n, "operacional")
	req, ok := OPERATIONAL_SECTIONS[k]
	if !ok {
		a.issue(n, "invalid operational clase %v", f["clase"])
		return
	}
	a.sections(n, req)
	if !has(OPERATIONAL_STATES, s(f["estado"])) {
		a.issue(n, "invalid operational estado %v", f["estado"])
	}
	if strings.TrimSpace(s(f["owner"])) == "" {
		a.issue(n, "owner must be a non-empty string")
	} else if s(f["estado"]) == "vigente" && s(f["owner"]) == "por-definir" {
		a.issue(n, "vigente operational note requires a real owner")
	}
	a.dates(n, "ultima-verificacion")
	a.lists(n, []string{"canales", "tags"}, false)
	parts := strings.Split(n.path, "/")
	area := ""
	if len(parts) >= 3 {
		area = parts[1]
	}
	slug := a.areas[area]
	allowed := append([]string{"operacion", "operacion/" + k}, a.systemTags...)
	a.tag(n, "operacion")
	a.tag(n, "operacion/"+k)
	if slug != "" {
		allowed = append(allowed, "operacion/area/"+slug)
		a.tag(n, "operacion/area/"+slug)
	}
	if tags, ok := arr(f["tags"]); ok {
		bad := []string{}
		for _, v := range tags {
			if !has(allowed, s(v)) {
				bad = append(bad, fmt.Sprint(v))
			}
		}
		if len(bad) > 0 {
			sort.Strings(bad)
			a.issue(n, "unsupported operational tags %v", bad)
		}
	}
	if len(parts) != 3 || parts[0] != "60-Operacion" {
		a.issue(n, "operational note must live at 60-Operacion/<Area>/<note>.md")
	} else {
		if _, ok := a.areas[area]; !ok {
			a.issue(n, "unknown operational area %s", area)
		}
		if s(f["area"]) != "[["+area+"]]" {
			a.issue(n, "area must be [[%s]]", area)
		}
	}
	if related, exists := f["relacionado-con"]; exists {
		v, ok := arr(related)
		if !ok {
			a.issue(n, "relacionado-con must be a list")
		} else {
			bad := []string{}
			for _, x := range v {
				valid := false
				for name := range a.areas {
					if s(x) == "[["+name+"]]" {
						valid = true
					}
				}
				if !valid {
					bad = append(bad, fmt.Sprint(x))
				}
			}
			if len(bad) > 0 {
				a.issue(n, "relacionado-con must target operational area MOCs: %v", bad)
			}
		}
	}
	if k == "reporte" {
		if !rx(`^[a-z0-9]+(?:-[a-z0-9]+)*$`, s(f["report-id"])) {
			a.issue(n, "reporte requires lowercase kebab-case report-id")
		}
	} else if _, ok := f["report-id"]; ok {
		a.issue(n, "report-id is only allowed for clase reporte")
	}
	if k == "procedimiento" && !strings.Contains(section(n.body, "Pasos"), OPERATIONAL_STEP_HEADER) {
		a.issue(n, "procedure steps must use the closed table header")
	}
}

const investigationID = `^\d{8}-\d{6}-[a-z0-9]+(?:-[a-z0-9]+)*(?:-\d{2})?$`
const wiki = `^\[\[[^\]|#]+\]\]$`

func (a *auditor) learning(n note) {
	f := n.f
	a.fields(n, LEARNING_FIELDS, LEARNING_FIELDS)
	a.sections(n, LEARNING_SECTIONS)
	a.kind(n, "aprendizaje")
	if !has(LEARNING_STATES, s(f["estado"])) {
		a.issue(n, "invalid learning estado %v", f["estado"])
	}
	if !has(LEARNING_RESULTS, s(f["resultado"])) {
		a.issue(n, "invalid learning resultado %v", f["resultado"])
	}
	if !strings.HasPrefix(stem(n.path), "Aprendizaje - ") {
		a.issue(n, "learning filename must start with Aprendizaje - ")
	}
	a.lists(n, LEARNING_LIST_FIELDS, false)
	for _, k := range []string{"aplica-a", "dimensiones", "investigaciones-origen"} {
		if v, ok := arr(f[k]); ok && len(v) == 0 {
			a.issue(n, "%s must not be empty", k)
		}
	}
	for _, k := range []string{"aplica-a", "supersede-a"} {
		v, _ := arr(f[k])
		for _, x := range v {
			if !rx(wiki, s(x)) {
				a.issue(n, "%s values must be canonical wikilinks", k)
			}
		}
	}
	dimensions, _ := arr(f["dimensiones"])
	for _, x := range dimensions {
		if !rx(`^[a-z0-9]+(?:-[a-z0-9]+)*$`, s(x)) {
			a.issue(n, "dimensiones values must be lowercase kebab-case")
		}
	}
	origins, _ := arr(f["investigaciones-origen"])
	for _, x := range origins {
		if !rx(investigationID, s(x)) {
			a.issue(n, "investigaciones-origen values must be investigation IDs")
		}
	}
	a.dates(n, "fecha-conclusion", "ultima-validacion")
	if date(f["fecha-conclusion"]) && date(f["ultima-validacion"]) && s(f["ultima-validacion"]) < s(f["fecha-conclusion"]) {
		a.issue(n, "ultima-validacion must not precede fecha-conclusion")
	}
	a.tag(n, "aprendizaje")
	for _, x := range dimensions {
		if v, ok := x.(string); ok {
			a.tag(n, "aprendizaje/"+v)
		}
	}
	evidence := section(n.body, "Evidencia acumulada")
	re := regexp.MustCompile(`(?m)^###\s+EV-(\d{3})\b.*$`)
	matches := re.FindAllStringSubmatchIndex(evidence, -1)
	seen := map[string]bool{}
	if len(matches) == 0 {
		a.issue(n, "Evidencia acumulada must contain at least one EV-### entry")
	}
	badOrder := false
	for i, m := range matches {
		end := len(evidence)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		entry := evidence[m[1]:end]
		id := evidence[m[2]:m[3]]
		if id != fmt.Sprintf("%03d", i+1) {
			badOrder = true
		}
		for _, field := range LEARNING_EVIDENCE_FIELDS {
			if !rx(`(?m)^-\s+`+regexp.QuoteMeta(field)+`:\s+\S`, entry) {
				a.issue(n, "EV-%s missing evidence field %s", id, field)
			}
		}
		originMatch := regexp.MustCompile(`(?m)^-\s+Investigación:\s+(.+?)\s*$`).FindStringSubmatch(entry)
		if len(originMatch) > 0 {
			origin := strings.TrimSpace(originMatch[1])
			if strings.HasPrefix(origin, "`") && strings.HasSuffix(origin, "`") {
				origin = strings.Trim(origin, "`")
			}
			if !rx(investigationID, origin) {
				a.issue(n, "EV-%s Investigación must be a valid investigation ID", id)
			} else if _, ok := arr(f["investigaciones-origen"]); ok && !contains(f["investigaciones-origen"], origin) {
				a.issue(n, "EV-%s Investigación must appear in investigaciones-origen", id)
			} else {
				seen[origin] = true
			}
		}
		if rx(`(?:\.investigations|\.investigations-private|\.operations|\.knowledge-os-handoffs)[/\\]|(?:^|[^A-Za-z0-9_.-])investigations[/\\]|(?:^|[^A-Za-z0-9_-])\.(?:plan|scratch)[/\\]`, entry) {
			a.issue(n, "EV-%s Fuentes durables must not reference local ignored workspaces", id)
		}
	}
	if badOrder {
		a.issue(n, "evidence IDs must be unique and sequential from EV-001")
	}
	for _, x := range origins {
		if rx(investigationID, s(x)) && !seen[s(x)] {
			a.issue(n, "investigaciones-origen value %s must be referenced by at least one EV entry", s(x))
		}
	}
	if strings.Join(headings(n.body), "\x00") != strings.Join(LEARNING_SECTION_ORDER, "\x00") {
		a.issue(n, "learning sections must exactly match the closed order")
	}
}
func (a *auditor) learningRelations(notes []note) {
	by := map[string]note{}
	graph := map[string][]string{}
	inbound := map[string]bool{}
	for _, n := range notes {
		by[stem(n.path)] = n
	}
	for _, n := range notes {
		name := stem(n.path)
		targets, _ := arr(n.f["supersede-a"])
		for _, raw := range targets {
			v := s(raw)
			if !rx(wiki, v) {
				continue
			}
			target := v[2 : len(v)-2]
			if target == name {
				a.issue(n, "supersede-a must not reference the same learning")
				continue
			}
			other, ok := by[target]
			if !ok || s(other.f["tipo"]) != "aprendizaje" {
				a.issue(n, "supersede-a target must be a learning note")
				continue
			}
			graph[name] = append(graph[name], target)
			inbound[target] = true
			if s(other.f["estado"]) != "superado" {
				a.issue(n, "supersede-a target must have estado superado")
			}
		}
	}
	for _, n := range notes {
		if s(n.f["estado"]) == "superado" && !inbound[stem(n.path)] {
			a.issue(n, "superado learning must be referenced by a replacement")
		}
	}
	state := map[string]int{}
	var visit func(string) bool
	visit = func(n string) bool {
		if state[n] == 1 {
			return true
		}
		if state[n] == 2 {
			return false
		}
		state[n] = 1
		for _, t := range graph[n] {
			if visit(t) {
				return true
			}
		}
		state[n] = 2
		return false
	}
	for n := range by {
		if visit(n) {
			a.issues = append(a.issues, "70-Aprendizajes: learning supersession graph must not contain cycles")
			break
		}
	}
}

var groupOrder = []string{"INDEX_NOTES", "SYSTEM_NOTES", "REPO_NOTES", "ARCHITECTURE_NOTES", "TOPIC_NOTES", "FLOW_NOTES", "INTEGRATION_NOTES", "GLOSSARY_NOTES", "OPERATIONAL_NOTES", "LEARNING_NOTES"}

// Audit returns deterministic diagnostics and per-family counts without mutating the vault.
func Audit(root string) (map[string]int, []string, error) {
	instance, e := config.LoadInstance(root)
	if e != nil {
		return nil, nil, e
	}
	a := auditor{root: root, contracts: map[string][]string{}, areas: map[string]string{}}
	systems, _ := arr(instance["systems"])
	for _, v := range systems {
		m, _ := v.(map[string]any)
		name := s(m["name"])
		alias := ""
		aliases, _ := arr(m["aliases"])
		if len(aliases) > 0 {
			alias = s(aliases[0])
		}
		tag := "sistema/" + s(m["id"])
		a.contracts[name] = []string{alias, tag}
		a.systems = append(a.systems, "[["+name+"]]")
		a.systemTags = append(a.systemTags, tag)
	}
	sort.Strings(a.systems)
	all := map[string]note{}
	e = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || rel == "investigations" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if has(FORBIDDEN_RELATION_FILES, d.Name()) {
			a.issues = append(a.issues, rel+": external relation ledger is not allowed")
		}
		if strings.HasSuffix(d.Name(), ".md") && auditedPath(rel) {
			n, err := read(root, rel)
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			all[rel] = n
		}
		return nil
	})
	if e != nil {
		return nil, nil, e
	}
	areaTags := map[string]map[string]bool{}
	for p, n := range all {
		parts := strings.Split(p, "/")
		if len(parts) == 3 && parts[0] == "60-Operacion" {
			if areaTags[parts[1]] == nil {
				areaTags[parts[1]] = map[string]bool{}
			}
			tags, _ := arr(n.f["tags"])
			for _, t := range tags {
				if strings.HasPrefix(s(t), "operacion/area/") {
					areaTags[parts[1]][strings.TrimPrefix(s(t), "operacion/area/")] = true
				}
			}
		}
	}
	for area, tags := range areaTags {
		if len(tags) == 1 {
			for t := range tags {
				a.areas[area] = t
			}
		}
	}
	groups := map[string][]note{}
	keys := []string{}
	for p := range all {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	rootNotes := []string{}
	reportIDs := map[string][]string{}
	for _, p := range keys {
		n := all[p]
		parts := strings.Split(p, "/")
		g := ""
		if has([]string{"00-Home.md", "30-Flujos/Flujos.md", "60-Operacion/Operacion.md", "70-Aprendizajes/Aprendizajes.md"}, p) {
			g = "INDEX_NOTES"
		} else if len(parts) == 3 && parts[0] == "60-Operacion" && a.areas[parts[1]] != "" && stem(p) == parts[1] {
			g = "INDEX_NOTES"
		} else {
			switch parts[0] {
			case "10-Sistemas":
				if len(parts) == 2 {
					g = "SYSTEM_NOTES"
				}
			case "20-Repos":
				if len(parts) == 3 {
					g = "REPO_NOTES"
				}
			case "15-Arquitectura":
				if len(parts) == 2 {
					g = "ARCHITECTURE_NOTES"
				}
			case "25-Topics":
				if len(parts) == 2 {
					g = "TOPIC_NOTES"
				}
			case "30-Flujos":
				if len(parts) == 2 {
					g = "FLOW_NOTES"
				}
			case "40-Integraciones":
				if len(parts) == 2 {
					g = "INTEGRATION_NOTES"
				}
			case "50-Glosario":
				if len(parts) == 2 {
					g = "GLOSSARY_NOTES"
				}
			case "60-Operacion":
				if filepath.Base(p) != "Operacion.md" {
					g = "OPERATIONAL_NOTES"
				}
			case "70-Aprendizajes":
				if len(parts) == 2 {
					g = "LEARNING_NOTES"
				}
			}
		}
		if parts[0] == "60-Operacion" && len(parts) == 2 {
			rootNotes = append(rootNotes, filepath.Base(p))
		}
		if g != "" {
			groups[g] = append(groups[g], n)
		}
		if g == "OPERATIONAL_NOTES" {
			if id, ok := n.f["report-id"].(string); ok {
				reportIDs[id] = append(reportIDs[id], p)
			}
		}
	}
	counts := map[string]int{}
	leak := regexp.MustCompile(`(?i)\b(?:la sync|la review|claims? (?:aceptad|rechazad)|por el gate|conocimiento durable adicional publicado)\b`)
	for _, g := range groupOrder {
		counts[g] = len(groups[g])
		for _, n := range groups[g] {
			a.check(n, g)
			if !strings.HasPrefix(n.path, "60-Operacion/") {
				for i, l := range strings.Split(n.raw, "\n") {
					if leak.MatchString(l) {
						a.issues = append(a.issues, fmt.Sprintf("%s:%d: synchronization process state must not appear in a durable graph note", n.path, i+1))
					}
				}
			}
		}
	}
	a.learningRelations(groups["LEARNING_NOTES"])
	if len(rootNotes) != 1 || rootNotes[0] != "Operacion.md" {
		a.issues = append(a.issues, fmt.Sprintf("60-Operacion: root must contain only Operacion.md; found %v", rootNotes))
	}
	for area := range a.areas {
		p := "60-Operacion/" + area + "/" + area + ".md"
		if _, ok := all[p]; !ok {
			a.issues = append(a.issues, fmt.Sprintf("60-Operacion/%s: missing area MOC %s.md", area, area))
		}
	}
	for id, owners := range reportIDs {
		if len(owners) > 1 {
			a.issues = append(a.issues, fmt.Sprintf("60-Operacion: duplicate report-id %s: %v", id, owners))
		}
	}
	sort.Strings(a.issues)
	return counts, a.issues, nil
}
func Run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	fs.SetOutput(out)
	root := fs.String("vault", ".", "vault directory")
	fs.StringVar(root, "root", ".", "vault directory (compatibility)")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return errors.New("audit accepts no positional arguments")
	}
	counts, issues, e := Audit(*root)
	if e != nil {
		return e
	}
	for _, g := range groupOrder {
		fmt.Fprintf(out, "%s: %d\n", g, counts[g])
	}
	fmt.Fprintf(out, "ISSUES: %d\n", len(issues))
	for _, issue := range issues {
		fmt.Fprintln(out, issue)
	}
	if len(issues) > 0 {
		return fmt.Errorf("structural audit found %d issues", len(issues))
	}
	return nil
}

func auditedPath(p string) bool {
	parts := strings.Split(p, "/")
	if p == "00-Home.md" {
		return true
	}
	switch parts[0] {
	case "10-Sistemas", "15-Arquitectura", "25-Topics", "30-Flujos", "40-Integraciones", "50-Glosario", "70-Aprendizajes":
		return len(parts) == 2
	case "20-Repos":
		return len(parts) == 3
	case "60-Operacion":
		return len(parts) >= 2
	}
	return false
}
