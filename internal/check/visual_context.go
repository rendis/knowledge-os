package check

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	gmtext "github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

type ContextResult struct {
	Pass     bool     `json:"context_pass"`
	Errors   []string `json:"errors"`
	Semantic string   `json:"semantic_review"`
}

var metadataMarker = regexp.MustCompile(`<!-- visual-context (.*?) -->`)
var comments = regexp.MustCompile(`(?s)<!--.*?-->`)

func contextLinks(body string) ([]string, bool, error) {
	source := []byte(comments.ReplaceAllString(body, ""))
	md := goldmark.New(goldmark.WithRendererOptions(gmhtml.WithUnsafe()))
	doc := md.Parser().Parse(gmtext.NewReader(source))
	embedded := false
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if f, ok := n.(*ast.FencedCodeBlock); ok && string(f.Language(source)) == "mermaid" {
			if strings.TrimSpace(string(f.Lines().Value(source))) != "" {
				embedded = true
			}
		}
		return ast.WalkContinue, nil
	})
	var rendered bytes.Buffer
	if e := md.Renderer().Render(&rendered, source, doc); e != nil {
		return nil, false, e
	}
	links := []string{}
	z := html.NewTokenizer(&rendered)
	svgDepth := 0
	for {
		typ := z.Next()
		if typ == html.ErrorToken {
			if z.Err() != io.EOF {
				return nil, false, z.Err()
			}
			break
		}
		tok := z.Token()
		if typ == html.EndTagToken && tok.Data == "svg" && svgDepth > 0 {
			svgDepth--
			embedded = true
		}
		if typ != html.StartTagToken && typ != html.SelfClosingTagToken {
			continue
		}
		if tok.Data == "svg" {
			svgDepth++
		}
		for _, a := range tok.Attr {
			if (tok.Data == "a" && a.Key == "href") || (tok.Data == "img" && a.Key == "src") {
				links = append(links, a.Val)
			}
		}
	}
	return links, embedded, nil
}
func resolved(path string) string {
	abs, e := filepath.Abs(path)
	if e != nil {
		return filepath.Clean(path)
	}
	real, e := filepath.EvalSymlinks(abs)
	if e == nil {
		return real
	}
	return abs
}
func inside(root, path string) bool {
	rel, e := filepath.Rel(root, path)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func portableAbs(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) || (len(path) > 1 && path[1] == ':')
}
func VisualContext(path string) ContextResult {
	r := ContextResult{Errors: []string{}, Semantic: "required"}
	add := func(s string) { r.Errors = append(r.Errors, s) }
	finish := func() ContextResult { r.Pass = len(r.Errors) == 0; return r }
	body, e := read(path)
	if e != nil {
		add(e.Error())
		return finish()
	}
	markers := metadataMarker.FindAllStringSubmatch(body, -1)
	if len(markers) != 1 {
		add("Expected exactly one visual-context metadata marker.")
		return finish()
	}
	var meta map[string]any
	if e = json.Unmarshal([]byte(markers[0][1]), &meta); e != nil || meta == nil {
		add("Context metadata must be an object.")
		return finish()
	}
	sources, ok := meta["sources"].([]any)
	validSources := ok && len(sources) > 0
	for _, s := range sources {
		validSources = validSources && nonempty(s)
	}
	if !validSources {
		add("Declare supporting sources or explicit synthetic assumptions.")
	}
	files, ok := meta["files"].([]any)
	if !ok {
		add("files must be a list.")
		return finish()
	}
	if len(files) == 0 && meta["embedded"] != true {
		add("A separate visual requires a file entry.")
	}
	links, embedded, e := contextLinks(body)
	if e != nil {
		add(e.Error())
		return finish()
	}
	if meta["embedded"] == true && !embedded {
		add("Embedded mode requires a diagram in this Markdown.")
	}
	dir := filepath.Dir(path)
	local := map[string]bool{}
	for _, link := range links {
		u, e := url.Parse(link)
		if e != nil {
			add("Invalid link: " + link)
			continue
		}
		if u.Scheme != "" || u.Host != "" {
			if u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "mailto" {
				add("Non-portable link: " + link)
			}
			continue
		}
		target := u.Path
		if target == "" {
			continue
		}
		if portableAbs(target) {
			add("Absolute local link: " + target)
			continue
		}
		local[target] = true
		if st, e := os.Stat(filepath.Join(dir, filepath.FromSlash(target))); e != nil || !st.Mode().IsRegular() {
			add("Missing linked file: " + target)
		}
	}
	seen := map[string]bool{}
	rootReal := resolved(dir)
	contextReal := resolved(path)
	for _, value := range files {
		entry, ok := value.(map[string]any)
		if !ok {
			add("Each file entry needs a relative path and SHA-256.")
			continue
		}
		name, ok := entry["path"].(string)
		if !ok {
			add("Each file entry needs a relative path and SHA-256.")
			continue
		}
		u, e := url.Parse(name)
		escape := name == "" || portableAbs(name) || e != nil || (u != nil && u.Scheme != "")
		for _, part := range strings.Split(strings.ReplaceAll(name, `\`, "/"), "/") {
			escape = escape || part == ".."
		}
		if escape {
			add("Artifact must be local to the context directory: " + name)
			continue
		}
		if seen[name] {
			add("Duplicate artifact: " + name)
		}
		seen[name] = true
		target := filepath.Join(dir, filepath.FromSlash(name))
		real := resolved(target)
		if real == contextReal {
			add("Context must not hash itself.")
			continue
		}
		st, e := os.Lstat(target)
		if e != nil || !st.Mode().IsRegular() || !inside(rootReal, real) {
			add("Missing or non-local artifact: " + name)
			continue
		}
		if !local[name] {
			add("Artifact needs a readable Markdown link: " + name)
		}
		data, e := os.ReadFile(target)
		if e != nil {
			add(e.Error())
			continue
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != entry["sha256"] {
			add("Unreviewed file drift or invalid SHA-256: " + name)
		}
	}
	for name := range local {
		real := resolved(filepath.Join(dir, filepath.FromSlash(name)))
		if inside(rootReal, real) && real != contextReal {
			found := false
			for item := range seen {
				if resolved(filepath.Join(dir, filepath.FromSlash(item))) == real {
					found = true
					break
				}
			}
			if !found {
				add("Local companion needs a file entry and SHA-256: " + name)
			}
		}
	}
	sameStem := false
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for name := range seen {
		if strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)) == stem {
			sameStem = true
		}
	}
	if len(files) > 0 && !sameStem {
		add("Context must share its stem with a represented file.")
	}
	return finish()
}
