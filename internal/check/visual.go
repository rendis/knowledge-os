package check

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type VisualResult struct {
	Pass    bool     `json:"structural_pass"`
	Errors  []string `json:"errors"`
	Manual  []string `json:"requires_manual_review"`
	Browser bool     `json:"browser_verified"`
}
type element struct {
	tag         string
	attrs       map[string]string
	transformed bool
}
type point struct{ x, y float64 }
type box struct{ x, y, w, h float64 }

var pathTokens = regexp.MustCompile(`[A-Za-z]|[-+]?(?:\d*\.\d+|\d+\.?\d*)(?:[eE][-+]?\d+)?`)

func endpoints(path string) (point, point, bool) {
	tokens := pathTokens.FindAllString(path, -1)
	if len(tokens) < 3 || tokens[0] != "M" {
		return point{}, point{}, false
	}
	count := 0
	cmd := ""
	for _, token := range append(append([]string{}, tokens...), "M") {
		if len(token) == 1 && ((token[0] >= 'a' && token[0] <= 'z') || (token[0] >= 'A' && token[0] <= 'Z')) {
			if token != "M" && token != "L" && token != "C" {
				return point{}, point{}, false
			}
			n := 2
			if cmd == "C" {
				n = 6
			}
			if cmd != "" && (count == 0 || count%n != 0) {
				return point{}, point{}, false
			}
			cmd = token
			count = 0
		} else {
			if _, e := strconv.ParseFloat(token, 64); e != nil {
				return point{}, point{}, false
			}
			count++
		}
	}
	nums := []float64{}
	for _, i := range []int{1, 2, len(tokens) - 2, len(tokens) - 1} {
		v, e := strconv.ParseFloat(tokens[i], 64)
		if e != nil {
			return point{}, point{}, false
		}
		nums = append(nums, v)
	}
	return point{nums[0], nums[1]}, point{nums[2], nums[3]}, true
}
func boundary(p point, b box) bool {
	return ((math.Abs(p.x-b.x) <= 1 || math.Abs(p.x-b.x-b.w) <= 1) && p.y >= b.y && p.y <= b.y+b.h) || ((math.Abs(p.y-b.y) <= 1 || math.Abs(p.y-b.y-b.h) <= 1) && p.x >= b.x && p.x <= b.x+b.w)
}
func isoDate(v string) bool {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02", "2006-01-02T15:04:05.999999999", "2006-01-02T15:04", "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05Z07:00"} {
		if _, e := time.Parse(layout, v); e == nil {
			return true
		}
	}
	return false
}
func Visual(text, kind string, temporal bool) VisualResult {
	r := VisualResult{Errors: []string{}, Manual: []string{"source fidelity and date/period meaning", "actual arrow endpoints and route causality", "rendered labels, contrast and overlap", "keyboard and control behavior"}}
	add := func(s string) { r.Errors = append(r.Errors, s) }
	elements := []element{}
	groups := []string{}
	geometry := []element{}
	boxes := map[string]box{}
	transformedBoxes := map[string]bool{}
	z := html.NewTokenizer(strings.NewReader(text))
	for {
		typ := z.Next()
		if typ == html.ErrorToken {
			if z.Err() != io.EOF {
				add(z.Err().Error())
			}
			break
		}
		if typ != html.StartTagToken && typ != html.SelfClosingTagToken && typ != html.EndTagToken {
			continue
		}
		tok := z.Token()
		if typ == html.EndTagToken {
			if len(geometry) > 0 && geometry[len(geometry)-1].tag == tok.Data {
				geometry = geometry[:len(geometry)-1]
			}
			if tok.Data == "g" && len(groups) > 0 {
				groups = groups[:len(groups)-1]
			}
			continue
		}
		attrs := map[string]string{}
		for _, a := range tok.Attr {
			attrs[a.Key] = a.Val
		}
		trans := attrs["transform"] != ""
		for _, g := range geometry {
			trans = trans || g.transformed
		}
		el := element{tok.Data, attrs, trans}
		elements = append(elements, el)
		if tok.Data == "svg" || tok.Data == "g" || tok.Data == "defs" || tok.Data == "symbol" {
			geometry = append(geometry, el)
		}
		if tok.Data == "g" {
			groups = append(groups, attrs["data-node"])
		}
		if tok.Data == "rect" && len(groups) > 0 && groups[len(groups)-1] != "" {
			name := groups[len(groups)-1]
			if trans {
				transformedBoxes[name] = true
			}
			vals := []float64{}
			valid := true
			for _, k := range []string{"x", "y", "width", "height"} {
				v := 0.0
				if attrs[k] != "" {
					var e error
					v, e = strconv.ParseFloat(attrs[k], 64)
					if e != nil {
						valid = false
					}
				}
				vals = append(vals, v)
			}
			if valid {
				boxes[name] = box{vals[0], vals[1], vals[2], vals[3]}
			}
		}
		if typ == html.SelfClosingTagToken {
			if len(geometry) > 0 && geometry[len(geometry)-1].tag == tok.Data {
				geometry = geometry[:len(geometry)-1]
			}
			if tok.Data == "g" && len(groups) > 0 {
				groups = groups[:len(groups)-1]
			}
		}
	}
	ids := map[string]int{}
	for _, e := range elements {
		if id := e.attrs["id"]; id != "" {
			ids[id]++
		}
	}
	for id, count := range ids {
		if count > 1 {
			add("Duplicate id: " + id)
		}
	}
	svgCount := 0
	for _, el := range elements {
		a := el.attrs
		for _, ref := range strings.Fields(a["aria-labelledby"] + " " + a["aria-describedby"]) {
			if ids[ref] == 0 {
				add("Missing accessibility reference: " + ref)
			}
		}
		if el.tag == "use" {
			ref, exists := a["href"]
			if !exists {
				ref = a["xlink:href"]
			}
			if !strings.HasPrefix(ref, "#") || ids[strings.TrimPrefix(ref, "#")] == 0 {
				add("Missing or non-local SVG symbol: " + ref)
			}
			if a["data-icon"] != "" && (a["aria-hidden"] != "true" || a["focusable"] != "false") {
				add("Supplementary component icons need aria-hidden=true and focusable=false; keep the visible component label.")
			}
		}
		if el.tag == "svg" {
			svgCount++
			if a["aria-hidden"] != "true" && a["aria-label"] == "" && a["aria-labelledby"] == "" {
				add("SVG needs an accessible name.")
			}
		}
	}
	if svgCount == 0 {
		add("Expected an SVG diagram in this checker; other renderers need their own checks.")
	}
	if kind == "spatial" {
		names := map[string]bool{}
		for _, el := range elements {
			a := el.attrs
			if a["data-node"] == "" {
				continue
			}
			name := a["data-node"]
			if names[name] {
				add("Duplicate data-node identifier.")
			}
			names[name] = true
			if a["role"] != "button" || a["tabindex"] != "0" {
				add("Node " + name + " needs role=button and tabindex=0.")
			}
			if a["aria-pressed"] != "true" && a["aria-pressed"] != "false" {
				add("Node " + name + " needs aria-pressed selection state.")
			}
		}
		if len(names) == 0 {
			add("Spatial explorer needs data-node identifiers.")
		}
		edges, labels := map[string]bool{}, map[string]bool{}
		edgeCount := 0
		for _, el := range elements {
			a := el.attrs
			if a["data-edge"] != "" {
				labels[a["data-edge"]] = true
			}
			isEdge := false
			for _, c := range strings.Fields(a["class"]) {
				isEdge = isEdge || c == "edge"
			}
			if el.tag != "path" || !isEdge {
				continue
			}
			edgeCount++
			id := a["id"]
			edges[id] = true
			for _, key := range []string{"data-from", "data-to"} {
				if !names[a[key]] {
					add("Edge " + id + " has missing/unknown " + key + ".")
				}
			}
			if id == "" || a["d"] == "" {
				add("Every edge needs an id and path geometry.")
			}
			start, end, ok := endpoints(a["d"])
			if !ok || el.transformed || transformedBoxes[a["data-from"]] || transformedBoxes[a["data-to"]] {
				add("Edge " + id + " needs manual geometry verification: checker supports untransformed absolute M/L/C paths only.")
			} else {
				for j, key := range []string{"data-from", "data-to"} {
					p := start
					if j == 1 {
						p = end
					}
					b, exists := boxes[a[key]]
					if !exists || !boundary(p, b) {
						add(fmt.Sprintf("Edge %s %s point (%g, %g) is not on its node rectangle boundary.", id, key, p.x, p.y))
					}
				}
			}
		}
		if edgeCount == 0 {
			add("Spatial explorer needs explicit edge paths.")
		}
		for id := range edges {
			if !labels[id] {
				add("Edge has no linked label: " + id)
			}
		}
		for id := range labels {
			if !edges[id] {
				add("Label refers to unknown edge: " + id)
			}
		}
	}
	if temporal {
		dates := []string{}
		for _, el := range elements {
			if el.tag == "time" && el.attrs["datetime"] != "" {
				dates = append(dates, el.attrs["datetime"])
			}
			if el.attrs["data-as-of"] != "" {
				dates = append(dates, el.attrs["data-as-of"])
			}
		}
		if len(dates) == 0 {
			add("Temporal visual needs source-backed time[datetime] or data-as-of (ISO date/time).")
		}
		for _, d := range dates {
			if !isoDate(d) {
				add("Invalid ISO date/time: " + d)
			}
		}
	}
	r.Pass = len(r.Errors) == 0
	return r
}
