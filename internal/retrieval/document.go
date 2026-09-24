package retrieval

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"go.yaml.in/yaml/v3"
)

type passage struct {
	Section, Body string
	Line          int
}
type document struct {
	Edges                              []Edge
	Title, Aliases, Origin, Visibility string
	Passages                           []passage
}

func readBounded(r io.Reader, limit int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, errors.New("file grew beyond indexing size limit")
	}
	if !utf8.Valid(b) {
		return nil, errors.New("note is not UTF-8")
	}
	return b, nil
}

func parseDocument(path string, raw []byte) (document, error) {
	d := document{Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Origin: "vault-note", Visibility: "public"}
	if strings.HasPrefix(path, "90-Meta/") {
		d.Origin = "vault-guidance"
	}
	if strings.HasPrefix(path, "investigations/") {
		d.Origin = "published-investigation"
	}
	if strings.HasPrefix(path, ".investigations/") {
		d.Origin = "local-investigation"
		d.Visibility = "private"
	}
	if strings.HasPrefix(path, ".investigations-private/") {
		d.Origin = "private-investigation"
		d.Visibility = "private"
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	offset := 0
	meta := map[string]any{}
	if len(lines) > 0 && lines[0] == "---" {
		end := -1
		for n := 1; n < len(lines); n++ {
			if lines[n] == "---" || lines[n] == "..." {
				end = n
				break
			}
		}
		if end < 0 {
			return d, errors.New("unterminated frontmatter")
		}

		if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &meta); err != nil {
			return d, fmt.Errorf("invalid frontmatter: %w", err)
		}
		if title, ok := meta["title"].(string); ok && strings.TrimSpace(title) != "" {
			d.Title = title
		}
		switch aliases := meta["aliases"].(type) {
		case string:
			d.Aliases = aliases
		case []any:
			for _, a := range aliases {
				if s, ok := a.(string); ok {
					d.Aliases += " " + s
				}
			}
		}
		offset = end + 1
		lines = lines[offset:]
	}
	body := []byte(strings.Join(lines, "\n"))
	d.Edges = extractEdges(meta, string(body))
	tree := goldmark.DefaultParser().Parse(text.NewReader(body))
	type heading struct {
		level int
		name  string
	}
	headings := map[int]heading{}
	ast.Walk(tree, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering && h.Lines().Len() > 0 {
			line := bytes.Count(body[:h.Lines().At(0).Start], []byte("\n"))
			headings[line] = heading{h.Level, string(h.Text(body))}
		}
		return ast.WalkContinue, nil
	})
	section := ""
	var levels [6]string
	var buf strings.Builder
	start := offset + 1
	length := 0
	flush := func() {
		if strings.TrimSpace(buf.String()) != "" {
			d.Passages = append(d.Passages, passage{section, buf.String(), start})
		}
		buf.Reset()
		length = 0
	}
	for n, line := range lines {
		if h, ok := headings[n]; ok {
			flush()
			levels[h.level-1] = h.name
			for x := h.level; x < 6; x++ {
				levels[x] = ""
			}
			var names []string
			for _, s := range levels {
				if s != "" {
					names = append(names, s)
				}
			}
			section = strings.Join(names, " > ")
			if d.Title == strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) && h.level == 1 {
				d.Title = h.name
			}
		}
		runes := []rune(line)
		if len(runes) == 0 {
			if length > 800 {
				flush()
			} else if length > 0 {
				buf.WriteByte('\n')
				length++
			}
			continue
		}
		for len(runes) > 0 {
			if length == 0 {
				start = offset + n + 1
			}
			room := 1600 - length
			if room == 0 {
				flush()
				continue
			}
			take := len(runes)
			if take > room {
				take = room
			}
			buf.WriteString(string(runes[:take]))
			length += take
			runes = runes[take:]
			if length >= 1600 {
				flush()
			}
		}
		if length > 0 {
			buf.WriteByte('\n')
			length++
		}
	}
	flush()
	if len(d.Passages) == 0 {
		d.Passages = []passage{{Body: "", Line: offset + 1}}
	}
	return d, nil
}
