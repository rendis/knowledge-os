package check

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const spatial = `<svg aria-label="demo"><g data-node="a" role="button" tabindex="0" aria-pressed="false"><rect x="0" y="0" width="10" height="10"/></g><g data-node="b" role="button" tabindex="0" aria-pressed="false"><rect x="20" y="0" width="10" height="10"/></g><path class="edge" id="e" data-from="a" data-to="b" d="M10 5L20 5"/><text data-edge="e">to b</text></svg>`

func TestVisualGeometryAndAccessibility(t *testing.T) {
	r := Visual(spatial, "spatial", false)
	if !r.Pass || r.Browser {
		t.Fatalf("%+v", r)
	}
	for _, pair := range [][2]string{{`data-to="b"`, `data-to="missing"`}, {`data-edge="e"`, `data-edge="missing"`}, {`aria-label="demo"`, `aria-labelledby="missing"`}, {`L20 5`, `L25 5`}, {`<svg `, `<svg transform="translate(2 2)" `}, {`<g data-node="a"`, `<g transform="translate(2 2)" data-node="a"`}, {`<rect x="0"`, `<rect transform="translate(2 2)" x="0"`}} {
		bad := strings.ReplaceAll(spatial, pair[0], pair[1])
		if Visual(bad, "spatial", false).Pass {
			t.Errorf("accepted %s", bad)
		}
	}
	icon := `<defs><g transform="translate(2 2)"><circle r="1"/></g></defs>`
	if !Visual(strings.ReplaceAll(spatial, "</svg>", icon+"</svg>"), "spatial", false).Pass {
		t.Fatal("unrelated transform rejected")
	}
}
func TestVisualTemporal(t *testing.T) {
	for _, v := range []struct {
		date  string
		valid bool
	}{{"", false}, {"2026-02-30", false}, {"2026-09-14", true}, {"2026-09-14T14:00:00Z", true}} {
		body := spatial
		if v.date != "" {
			body = strings.Replace(body, "<svg", `<svg data-as-of="`+v.date+`"`, 1)
		}
		if Visual(body, "spatial", true).Pass != v.valid {
			t.Errorf("date %s", v.date)
		}
	}
}
func TestVisualActualTemplates(t *testing.T) {
	for _, name := range []string{"path-explorer.html", "explainer.html", "explorer.html"} {
		path := filepath.Join("..", "..", "kernel", ".agents", "skills", "explain-visually", "assets", name)
		body, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		kind := "diagram"
		if name == "path-explorer.html" {
			kind = "spatial"
		}
		r := Visual(string(body), kind, false)
		if !r.Pass {
			t.Fatalf("%s: %+v", name, r)
		}
	}
}
func marker(files, extra string) string {
	return `<!-- visual-context {"files": ` + files + `, "sources": ["E-001"]` + extra + `} -->` + "\n"
}
func TestVisualContextDriftAndCompanions(t *testing.T) {
	root := t.TempDir()
	put(t, root, "flow.html", "<h1>Flow</h1>")
	files := fmt.Sprintf(`[{"path":"flow.html","sha256":"%x"}]`, sha256.Sum256([]byte("<h1>Flow</h1>")))
	path := filepath.Join(root, "flow.md")
	body := marker(files, "") + "[Open][view]\n\n[view]: flow.html\n"
	put(t, root, "flow.md", body)
	if r := VisualContext(path); !r.Pass {
		t.Fatalf("%+v", r)
	}
	put(t, root, "flow.html", "Changed")
	if VisualContext(path).Pass {
		t.Fatal("drift accepted")
	}
	put(t, root, "flow.html", "<h1>Flow</h1>")
	put(t, root, "data.csv", "units\n10\n")
	put(t, root, "flow.md", body+"\n[Data](data.csv)")
	if VisualContext(path).Pass {
		t.Fatal("unlisted companion accepted")
	}
}
func TestVisualContextEmbeddedExamples(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "flow.md")
	header := marker("[]", `, "embedded":true`)
	for _, example := range []string{"<!-- <svg></svg> -->", "```html\n<svg></svg>\n```", "```mermaid\n\n```", "The literal <svg token is not a diagram.", "```html\n<svg></svg>\n````", "`<svg></svg>`", "    <svg></svg>"} {
		put(t, root, "flow.md", header+example)
		if VisualContext(path).Pass {
			t.Errorf("accepted %s", example)
		}
	}
	for _, valid := range []string{"```mermaid\nflowchart LR\n A --> B\n```", `<svg aria-label="Flow"><text>Example</text></svg>`} {
		put(t, root, "flow.md", header+valid)
		if r := VisualContext(path); !r.Pass {
			t.Errorf("%+v", r)
		}
	}
}
func TestVisualContextEscapeAndSelf(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"flow.md", "../outside.html", "/tmp/outside.html", "C:/outside.html"} {
		files := fmt.Sprintf(`[{"path":%q,"sha256":"0000"}]`, name)
		put(t, root, "flow.md", marker(files, "")+"[Open]("+name+")")
		if VisualContext(filepath.Join(root, "flow.md")).Pass {
			t.Errorf("accepted %s", name)
		}
	}
}
