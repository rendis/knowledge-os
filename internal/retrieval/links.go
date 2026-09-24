package retrieval

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Edge struct {
	Field  string `json:"field"`
	Target string `json:"target"`
}
type Incoming struct {
	Field  string `json:"field"`
	Source string `json:"source"`
	Path   string `json:"path"`
}
type Neighbors struct {
	Node                    string                 `json:"node"`
	Path                    string                 `json:"path"`
	Outgoing                []Edge                 `json:"outgoing"`
	Incoming                []Incoming             `json:"incoming"`
	Refresh                 Refresh                `json:"refresh"`
	Investigations          []InvestigationPointer `json:"investigations"`
	InvestigationsTruncated bool                   `json:"investigations_truncated"`
}

// InvestigationPointer is a discovery hint, never an assertion of authority.
type InvestigationPointer struct {
	Title  string `json:"title"`
	Path   string `json:"path"`
	Origin string `json:"origin"`
}

const investigationPointerLimit = 20

var wiki = regexp.MustCompile(`\[\[([^\]|#]+)(?:#[^\]|]*)?(?:\|[^\]]*)?\]\]`)

func extractEdges(meta map[string]any, raw string) []Edge {
	out := []Edge{}
	seen := map[string]bool{}
	add := func(field, target string) {
		target = strings.TrimSpace(target)
		if target != "" && !seen[field+"\x00"+target] {
			out = append(out, Edge{field, target})
			seen[field+"\x00"+target] = true
		}
	}
	var values func(string, any)
	values = func(field string, v any) {
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				values(field, item)
			}
		case string:
			matches := wiki.FindAllStringSubmatch(x, -1)
			if len(matches) == 0 {
				add(field, x)
			} else {
				for _, m := range matches {
					add(field, m[1])
				}
			}
		}
	}
	for _, field := range []string{"publica-en", "gatillado-por", "consume-de", "lee-de", "escribe-en", "usa-infra", "participa-en", "compuesto-por", "implementado-por", "aplica-a", "sistema"} {
		values(field, meta[field])
	}
	targets := map[string]bool{}
	for _, e := range out {
		targets[e.Target] = true
	}
	for _, m := range wiki.FindAllStringSubmatch(raw, -1) {
		if !targets[m[1]] {
			add("body", m[1])
			targets[m[1]] = true
		}
	}
	return out
}
func (i *Index) Neighbors(ctx context.Context, node string) (Neighbors, error) {
	out := Neighbors{Node: node, Outgoing: []Edge{}, Incoming: []Incoming{}, Refresh: i.Stats, Investigations: []InvestigationPointer{}}
	if strings.TrimSpace(node) == "" {
		return out, fmt.Errorf("--node is required")
	}
	rows, err := i.db.QueryContext(ctx, `SELECT path FROM files ORDER BY path`)
	if err != nil {
		return out, err
	}
	paths := map[string]string{}
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			rows.Close()
			return out, err
		}
		if strings.HasPrefix(p, "investigations/") || strings.HasPrefix(p, ".") {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if previous, ok := paths[stem]; ok {
			rows.Close()
			return out, fmt.Errorf("ambiguous basename %q: %s and %s", stem, previous, p)
		}
		paths[stem] = p
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.Path = paths[node]
	rows, err = i.db.QueryContext(ctx, `SELECT path,field,target FROM edges ORDER BY path,field,target`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var p, field, target string
		if err = rows.Scan(&p, &field, &target); err != nil {
			return out, err
		}
		if strings.HasPrefix(p, "investigations/") || strings.HasPrefix(p, ".") {
			continue
		}
		if p == out.Path {
			out.Outgoing = append(out.Outgoing, Edge{field, target})
		}
		if target == node {
			out.Incoming = append(out.Incoming, Incoming{field, strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)), p})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.Investigations, out.InvestigationsTruncated, err = i.investigationPointers(ctx, node)
	return out, err
}

func (i *Index) investigationPointers(ctx context.Context, node string) ([]InvestigationPointer, bool, error) {
	result := []InvestigationPointer{}
	// Scan only published case records; private overlays and resource copies are
	// different evidence surfaces, not additional cases. Match literal text, not
	// FTS syntax, so identifiers and punctuation retain their meaning.
	rows, err := i.db.QueryContext(ctx, `SELECT path,title,body FROM passages WHERE path GLOB 'investigations/*/investigation.md' ORDER BY path,rowid`)
	if err != nil {
		return result, false, err
	}
	defer rows.Close()
	needle := strings.ToLower(strings.TrimSpace(node))
	last := ""
	for rows.Next() {
		var path, title, body string
		if err := rows.Scan(&path, &title, &body); err != nil {
			return result, false, err
		}
		if strings.Count(path, "/") != 2 || path == last || !strings.Contains(strings.ToLower(title+"\n"+body), needle) {
			continue
		}
		if len(result) == investigationPointerLimit {
			return result, true, nil
		}
		if runes := []rune(title); len(runes) > 240 {
			title = string(runes[:240]) + "…"
		}
		result = append(result, InvestigationPointer{Title: title, Path: path, Origin: "published-investigation"})
		last = path
	}
	return result, false, rows.Err()
}
