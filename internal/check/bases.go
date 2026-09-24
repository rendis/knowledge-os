package check

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

type BaseResult struct {
	Bases  int      `json:"bases"`
	Issues []string `json:"issues"`
}

func stringSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, v := range strings.Fields(s) {
		m[v] = true
	}
	return m
}

var contractProperties = stringSet(`aliases ambiente area clase cobertura-datos cobertura-entradas cobertura-flujos cobertura-infra cobertura-salidas commit-analizado compuesto-por consume-de ejecuta escribe-en estado fecha-analisis gatilla-a gatillado-por implementado-por lee-de lenguaje nombre-raw owner participa-en plataforma proyecto publica-en report-id rama-analizada region sistema tags tipo ultima-auditoria ultima-verificacion usa-infra`)
var fileProperties = stringSet(`file.backlinks file.basename file.ctime file.embeds file.ext file.file file.folder file.links file.mtime file.name file.path file.properties file.size file.tags`)

func nonempty(v any) bool                  { s, ok := v.(string); return ok && strings.TrimSpace(s) != "" }
func mapping(v any) (map[string]any, bool) { m, ok := v.(map[string]any); return m, ok }
func keys(m map[string]any) []string {
	k := []string{}
	for key := range m {
		k = append(k, key)
	}
	sort.Strings(k)
	return k
}
func Bases(root string) (BaseResult, error) {
	r := BaseResult{Issues: []string{}}
	paths, e := filepath.Glob(filepath.Join(root, "*.base"))
	if e != nil {
		return r, e
	}
	r.Bases = len(paths)
	if len(paths) == 0 {
		r.Issues = append(r.Issues, "No .base files found at the vault root")
		return r, nil
	}
	props := map[string]bool{}
	for p := range contractProperties {
		props[p] = true
	}
	notes, e := visible(root, ".md", true)
	if e != nil {
		return r, e
	}
	for _, p := range notes {
		body, e := read(p)
		if e != nil {
			continue
		}
		fm, e := frontmatter(body)
		if e != nil {
			continue
		}
		for key := range fm {
			props[key] = true
		}
	}
	for _, p := range paths {
		r.Issues = append(r.Issues, validateBase(p, root, props)...)
	}
	return r, nil
}
func validateFilter(v any, loc string) []string {
	if nonempty(v) {
		return nil
	}
	m, ok := mapping(v)
	if !ok {
		return []string{loc + " must be a non-empty string or filter mapping"}
	}
	if len(m) != 1 {
		return []string{loc + " must contain exactly one of and, or, or not"}
	}
	for op, v := range m {
		if op != "and" && op != "or" && op != "not" {
			return []string{loc + " must contain exactly one of and, or, or not"}
		}
		list, ok := v.([]any)
		if !ok || len(list) == 0 {
			return []string{loc + "." + op + " must be a non-empty list"}
		}
		issues := []string{}
		for i, child := range list {
			issues = append(issues, validateFilter(child, fmt.Sprintf("%s.%s[%d]", loc, op, i))...)
		}
		return issues
	}
	return nil
}
func validateStrings(v any, loc string) []string {
	m, ok := mapping(v)
	if !ok {
		return []string{loc + " must be a mapping"}
	}
	issues := []string{}
	for _, k := range keys(m) {
		if !nonempty(k) {
			issues = append(issues, loc+" keys must be non-empty strings")
		}
		if !nonempty(m[k]) {
			issues = append(issues, loc+"."+k+" must be a non-empty string")
		}
	}
	return issues
}
func property(v any, loc string, props, formulas map[string]bool) []string {
	s, ok := v.(string)
	if !ok || s == "" {
		return []string{loc + " must be a non-empty property reference"}
	}
	if strings.HasPrefix(s, "formula.") {
		if !formulas[strings.TrimPrefix(s, "formula.")] {
			return []string{loc + " references undefined formula " + s}
		}
	} else if strings.HasPrefix(s, "file.") {
		if !fileProperties[s] {
			return []string{loc + " references unknown file property " + s}
		}
	} else if !props[strings.TrimPrefix(s, "note.")] {
		return []string{loc + " references unknown note property " + s}
	}
	return nil
}
func validateView(v any, i int, props, formulas map[string]bool) []string {
	loc := fmt.Sprintf("views[%d]", i)
	m, ok := mapping(v)
	if !ok {
		return []string{loc + " must be a mapping"}
	}
	issues := []string{}
	for _, field := range []string{"type", "name"} {
		if !nonempty(m[field]) {
			issues = append(issues, loc+"."+field+" must be a non-empty string")
		}
	}
	if v, ok := m["filters"]; ok {
		issues = append(issues, validateFilter(v, loc+".filters")...)
	}
	if v, ok := m["limit"]; ok {
		n, ok := v.(int)
		if !ok || n <= 0 {
			issues = append(issues, loc+".limit must be a positive integer")
		}
	}
	if v, ok := m["order"]; ok {
		list, ok := v.([]any)
		if !ok {
			issues = append(issues, loc+".order must be a list")
		} else {
			seen := map[string]bool{}
			for j, p := range list {
				at := fmt.Sprintf("%s.order[%d]", loc, j)
				issues = append(issues, property(p, at, props, formulas)...)
				if s, ok := p.(string); ok {
					if seen[s] {
						issues = append(issues, at+" duplicates property "+s)
					}
					seen[s] = true
				}
			}
		}
	}
	if v, ok := m["groupBy"]; ok {
		g, ok := mapping(v)
		if !ok {
			issues = append(issues, loc+".groupBy must be a mapping")
		} else {
			issues = append(issues, property(g["property"], loc+".groupBy.property", props, formulas)...)
			if g["direction"] != "ASC" && g["direction"] != "DESC" {
				issues = append(issues, loc+".groupBy.direction must be ASC or DESC")
			}
		}
	}
	if v, ok := m["summaries"]; ok {
		summaries, ok := mapping(v)
		if !ok {
			issues = append(issues, loc+".summaries must be a mapping")
		} else {
			for _, p := range keys(summaries) {
				at := loc + ".summaries." + p
				issues = append(issues, property(p, at, props, formulas)...)
				if !nonempty(summaries[p]) {
					issues = append(issues, at+" must name a summary")
				}
			}
		}
	}
	return issues
}

// Bound tree depth, including alias references, before decoding recursive filters.
func safeNode(n *yaml.Node, depth int) error {
	active := map[*yaml.Node]bool{}
	budget := 100000
	var visit func(*yaml.Node, int) error
	visit = func(n *yaml.Node, d int) error {
		budget--
		if budget < 0 {
			return fmt.Errorf("YAML structure exceeds limits")
		}
		if n == nil {
			return fmt.Errorf("invalid YAML alias")
		}
		if d > 64 || active[n] {
			return fmt.Errorf("YAML recursion/depth exceeds limits")
		}
		active[n] = true
		defer delete(active, n)
		if n.Kind == yaml.AliasNode {
			return visit(n.Alias, d+1)
		}
		for _, c := range n.Content {
			if e := visit(c, d+1); e != nil {
				return e
			}
		}
		return nil
	}
	return visit(n, depth)
}

func validateBase(path, root string, props map[string]bool) []string {
	rel, _ := filepath.Rel(root, path)
	prefix := filepath.ToSlash(rel) + ": "
	fail := func(s string) []string { return []string{prefix + s} }
	st, e := os.Lstat(path)
	if e != nil {
		return fail("cannot read file: " + e.Error())
	}
	if !st.Mode().IsRegular() {
		return fail("must be a regular file")
	}
	if st.Size() > 1000000 {
		return fail("file exceeds 1000000 bytes")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return fail("cannot read file: " + e.Error())
	}
	if len(b) > 1000000 {
		return fail("file exceeds 1000000 bytes")
	}
	if !utf8.Valid(b) {
		return fail("must be UTF-8")
	}
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	if e = decoder.Decode(&node); e != nil {
		return fail("invalid YAML: " + e.Error())
	}
	var extra yaml.Node
	if e = decoder.Decode(&extra); e != io.EOF {
		return fail("invalid YAML: expected one document")
	}
	if e = safeNode(&node, 0); e != nil {
		return fail("invalid YAML: " + e.Error())
	}
	var raw any
	if e = node.Decode(&raw); e != nil {
		return fail("invalid YAML: " + e.Error())
	}
	m, ok := mapping(raw)
	if !ok {
		return fail("root must be a mapping")
	}
	issues := []string{}
	allowed := stringSet("filters formulas properties summaries views")
	unknown := []string{}
	for _, k := range keys(m) {
		if !allowed[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		issues = append(issues, "unknown top-level keys: "+strings.Join(unknown, ", "))
	}
	if v, ok := m["filters"]; ok {
		issues = append(issues, validateFilter(v, "filters")...)
	}
	formulas := map[string]bool{}
	if v, ok := m["formulas"]; ok {
		issues = append(issues, validateStrings(v, "formulas")...)
		if f, ok := mapping(v); ok {
			for k := range f {
				formulas[k] = true
			}
		}
	}
	if v, ok := m["summaries"]; ok {
		issues = append(issues, validateStrings(v, "summaries")...)
	}
	if v, ok := m["properties"]; ok {
		pm, ok := mapping(v)
		if !ok {
			issues = append(issues, "properties must be a mapping")
		} else {
			for _, p := range keys(pm) {
				at := "properties." + p
				issues = append(issues, property(p, at, props, formulas)...)
				config, ok := mapping(pm[p])
				if !ok {
					issues = append(issues, at+" must be a mapping")
				} else if name, exists := config["displayName"]; exists {
					if _, ok := name.(string); !ok {
						issues = append(issues, at+".displayName must be a string")
					}
				}
			}
		}
	}
	views, ok := m["views"].([]any)
	if !ok || len(views) == 0 {
		issues = append(issues, "views must be a non-empty list")
	} else {
		names := map[string]bool{}
		for i, v := range views {
			issues = append(issues, validateView(v, i, props, formulas)...)
			if view, ok := mapping(v); ok && nonempty(view["name"]) {
				name := view["name"].(string)
				if names[name] {
					issues = append(issues, fmt.Sprintf("views[%d].name duplicates view %s", i, name))
				}
				names[name] = true
			}
		}
	}
	for i := range issues {
		issues[i] = prefix + issues[i]
	}
	return issues
}
