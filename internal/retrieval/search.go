package retrieval

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Card struct {
	SourceGroup string  `json:"source_group"`
	Vault       string  `json:"vault"`
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Section     string  `json:"section"`
	Line        int     `json:"line"`
	Excerpt     string  `json:"excerpt"`
	Origin      string  `json:"origin"`
	Visibility  string  `json:"visibility"`
	SHA256      string  `json:"sha256"`
	Rank        float64 `json:"bm25"`
}
type Result struct {
	Cards   []Card  `json:"cards"`
	Match   string  `json:"match"`
	Refresh Refresh `json:"refresh"`
	Ranking string  `json:"ranking"`
}

var words = regexp.MustCompile(`[\p{L}\p{N}_]+`)

func literalQuery(query string) ([]string, error) {
	if !utf8.ValidString(query) || len(query) > 4096 {
		return nil, errors.New("query must be UTF-8 and at most 4096 bytes")
	}
	terms := words.FindAllString(query, -1)
	if len(terms) > 64 {
		return nil, errors.New("query has more than 64 terms")
	}
	unique := map[string]bool{}
	var out []string
	for _, t := range terms {
		t = strings.ToLower(t)
		if !unique[t] {
			unique[t] = true
			out = append(out, `"`+t+`"`)
		}
	}
	return out, nil
}
func (i *Index) Search(ctx context.Context, query string, limit int, visibility string) (Result, error) {
	result := Result{Cards: []Card{}, Match: "none", Refresh: i.Stats, Ranking: "FTS5 BM25; lower is better; not a confidence probability"}
	if limit < 1 || limit > 10 {
		return result, errors.New("limit must be between 1 and 10")
	}
	if visibility != "all" && visibility != "public" {
		return result, errors.New("visibility must be all or public")
	}
	terms, err := literalQuery(query)
	if err != nil {
		return result, err
	}
	if len(terms) == 0 {
		return result, nil
	}
	for _, mode := range []string{"AND", "OR"} {
		// One best passage per file; ranking stays SQLite BM25, not a custom score.
		rows, err := i.db.QueryContext(ctx, `SELECT path,title,section,line,snippet(passages,4,'','', ' … ',32),origin,visibility,hash,bm25(passages,0,5,4,3,1,0,0,0,0)
  FROM passages WHERE passages MATCH ? AND (?='all' OR visibility='public') ORDER BY bm25(passages,0,5,4,3,1,0,0,0,0),path,rowid`, strings.Join(terms, " "+mode+" "), visibility)
		if err != nil {
			return result, err
		}
		seen := map[string]bool{}
		for rows.Next() {
			var c Card
			c.Vault = i.Root
			if err = rows.Scan(&c.Path, &c.Title, &c.Section, &c.Line, &c.Excerpt, &c.Origin, &c.Visibility, &c.SHA256, &c.Rank); err != nil {
				rows.Close()
				return result, err
			}
			c.SourceGroup = sourceGroup(c.Path)
			if seen[c.SourceGroup] {
				continue
			}
			seen[c.SourceGroup] = true
			if r := []rune(c.Excerpt); len(r) > 360 {
				c.Excerpt = string(r[:360]) + "…"
			}
			if r := []rune(c.Title); len(r) > 240 {
				c.Title = string(r[:240]) + "…"
			}
			if r := []rune(c.Section); len(r) > 240 {
				c.Section = string(r[:240]) + "…"
			}
			result.Cards = append(result.Cards, c)
			if len(result.Cards) == limit {
				break
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
		if len(result.Cards) > 0 {
			result.Match = strings.ToLower(mode)
			return result, nil
		}
	}
	return result, nil
}

func sourceGroup(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 2 && (parts[0] == "investigations" || parts[0] == ".investigations" || parts[0] == ".investigations-private") {
		return "investigation:" + parts[1]
	}
	return "note:" + path
}
