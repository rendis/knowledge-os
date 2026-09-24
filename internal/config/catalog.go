package config

import (
	"errors"
	"fmt"
	"go.yaml.in/yaml/v3"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func frontmatter(path string) (Object, error) {
	st, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("operational note must be a regular file")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return Object{}, nil
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil, errors.New("unterminated frontmatter")
	}
	var m Object
	e = yaml.Unmarshal([]byte(text[4:4+end]), &m)
	return m, e
}
func safeNote(root, path string) error {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	root = canonicalRoot
	r, e := filepath.EvalSymlinks(path)
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(root, r)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("note escapes vault")
	}
	if st, err := os.Lstat(path); err != nil || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("symlinked operational notes are not permitted")
	}
	return nil
}
func ResolveCapability(root string, m Object, capability string) (Object, error) {
	values, exists := obj(m["capabilities"])[capability]
	if !exists {
		return Object{"status": "unconfigured", "capability": capability, "procedures": []string{}}, nil
	}
	names, e := stringsList(values, true)
	if e != nil {
		return nil, e
	}
	paths := []string{}
	base := filepath.Join(root, "60-Operacion")
	for _, name := range names {
		matches := []string{}
		e = filepath.WalkDir(base, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && d.Name() == name+".md" {
				matches = append(matches, p)
			}
			return nil
		})
		if e != nil {
			return nil, e
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf("capability %s: missing or ambiguous procedure %s", capability, name)
		}
		p := matches[0]
		if e = safeNote(root, p); e != nil {
			return nil, e
		}
		fm, e := frontmatter(p)
		if e != nil {
			return nil, e
		}
		if fm["tipo"] != "operacional" {
			return nil, errors.New("capability procedure must be operational")
		}
		rel, _ := filepath.Rel(root, p)
		paths = append(paths, filepath.ToSlash(rel))
	}
	return Object{"status": "configured", "capability": capability, "procedures": paths}, nil
}
func OperationalNotes(root string) ([]Object, error) {
	paths, e := filepath.Glob(filepath.Join(root, "60-Operacion", "*", "*.md"))
	if e != nil {
		return nil, e
	}
	r := []Object{}
	for _, p := range paths {
		stem := strings.TrimSuffix(filepath.Base(p), ".md")
		if stem == filepath.Base(filepath.Dir(p)) {
			continue
		}
		if e = safeNote(root, p); e != nil {
			return nil, e
		}
		m, e := frontmatter(p)
		if e != nil {
			return nil, e
		}
		if m["tipo"] != "operacional" {
			return nil, fmt.Errorf("%s: tipo must be operacional", p)
		}
		rel, _ := filepath.Rel(root, p)
		r = append(r, Object{"basename": stem, "path": filepath.ToSlash(rel), "area": m["area"], "clase": m["clase"], "estado": m["estado"], "report-id": m["report-id"]})
	}
	return r, nil
}
func OperationalAreas(root string) ([]Object, error) {
	base := filepath.Join(root, "60-Operacion")
	dirs, e := os.ReadDir(base)
	if e != nil {
		return nil, e
	}
	r := []Object{}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		p := filepath.Join(base, d.Name(), d.Name()+".md")
		notes, _ := filepath.Glob(filepath.Join(base, d.Name(), "*.md"))
		if len(notes) == 0 {
			continue
		}
		if e = safeNote(root, p); e != nil {
			return nil, e
		}
		m, e := frontmatter(p)
		if e != nil {
			return nil, e
		}
		tags := map[string]bool{}
		for _, t := range list(m["tags"]) {
			tags[str(t)] = true
		}
		if m["tipo"] != "indice" || !tags["moc"] || !tags["operacion"] {
			return nil, fmt.Errorf("invalid area MOC %s", p)
		}
		rel, _ := filepath.Rel(root, p)
		r = append(r, Object{"area": d.Name(), "path": filepath.ToSlash(rel)})
	}
	return r, nil
}
func OperationalReports(root string) ([]Object, error) {
	notes, e := OperationalNotes(root)
	if e != nil {
		return nil, e
	}
	r := []Object{}
	seen := map[string]bool{}
	for _, n := range notes {
		if n["clase"] != "reporte" {
			continue
		}
		id := str(n["report-id"])
		if id == "" || seen[id] {
			return nil, errors.New("report-id must be present and unique")
		}
		seen[id] = true
		r = append(r, n)
	}
	sort.Slice(r, func(i, j int) bool { return str(r[i]["report-id"]) < str(r[j]["report-id"]) })
	return r, nil
}
func Catalog(root string) (Object, error) {
	_, m, _, e := readYAML(filepath.Join(root, "90-Meta", "vault-catalog.yaml"))
	if os.IsNotExist(e) {
		return Object{"version": 1, "vaults": []any{}}, nil
	}
	if e != nil {
		return nil, e
	}
	if e = allowed(m, "version vaults"); e != nil {
		return nil, e
	}
	if m["version"] != 1 {
		return nil, errors.New("catalog version must be 1")
	}
	vaults, ok := m["vaults"].([]any)
	if !ok {
		return nil, errors.New("catalog vaults must be list")
	}
	ids, repos := map[string]bool{}, map[string]bool{}
	for _, x := range vaults {
		entry := obj(x)
		if len(entry) != 8 {
			return nil, errors.New("invalid catalog fields")
		}
		if e = allowed(entry, "id repository domain scope summary tags relationship consult_when"); e != nil {
			return nil, e
		}
		for _, f := range []string{"id", "repository", "domain", "scope", "summary", "relationship"} {
			if !nonempty(entry[f]) {
				return nil, fmt.Errorf("catalog requires %s", f)
			}
		}
		id := str(entry["id"])
		if !kebab.MatchString(id) || ids[id] {
			return nil, errors.New("invalid or duplicate catalog id")
		}
		ids[id] = true
		repo, e := RemoteIdentity(str(entry["repository"]))
		if e != nil {
			return nil, e
		}
		if repos[repo] {
			return nil, errors.New("duplicate catalog repository")
		}
		repos[repo] = true
		for _, f := range []string{"tags", "consult_when"} {
			values, e := stringsList(entry[f], true)
			if e != nil {
				return nil, e
			}
			seen := map[string]bool{}
			for _, s := range values {
				s = strings.ToLower(strings.TrimSpace(s))
				if seen[s] {
					return nil, errors.New("duplicate catalog text entry")
				}
				seen[s] = true
			}
		}
	}
	return m, nil
}
