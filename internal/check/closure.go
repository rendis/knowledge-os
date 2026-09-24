package check

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The two summary forms are persisted cell contracts. Keep compatibility until
// the summaries and their consumers have an explicitly migrated representation.
var closureCount = regexp.MustCompile(`(?i)(?:permanecen|remaining\s*:?)\s+(\d+)\s+(?:verificaciones|verification items)\s+(?:en|in)\s+(\d+)\s+(?:notas|notes)`)
var uncheckedItem = regexp.MustCompile(`(?m)^\s*- \[ \]`)

type ClosureCounts struct {
	Remaining int `json:"remaining_verification_items"`
	Notes     int `json:"notes_with_verifications"`
}
type ClosureResult struct {
	Status   string        `json:"status"`
	Observed ClosureCounts `json:"observed"`
	Errors   []string      `json:"errors"`
}

// MapClosure checks a caller-selected existing checkpoint and visible summaries.
// It never rewrites current or historical evidence.
func MapClosure(vault, checkpoint string) (ClosureResult, error) {
	r := ClosureResult{Status: "blocked", Errors: []string{}}
	fail := func(e error) (ClosureResult, error) { r.Errors = append(r.Errors, e.Error()); return r, e }
	root, e := filepath.Abs(vault)
	if e != nil {
		return fail(e)
	}
	root, e = filepath.EvalSymlinks(root)
	if e != nil {
		return fail(e)
	}
	st, e := os.Stat(root)
	if e != nil {
		return fail(e)
	}
	if !st.IsDir() {
		return fail(errors.New("vault must be a directory"))
	}
	b, e := os.ReadFile(checkpoint)
	if e != nil {
		return fail(e)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var data map[string]any
	if e = dec.Decode(&data); e != nil {
		return fail(e)
	}
	var trailing any
	if e = dec.Decode(&trailing); e != io.EOF {
		return fail(errors.New("checkpoint must contain one JSON object"))
	}
	if data == nil {
		return fail(errors.New("checkpoint must contain a JSON object"))
	}
	repoRoot := filepath.Join(root, "20-Repos")
	if _, e = os.Stat(repoRoot); e == nil {
		e = filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			resolved, e := filepath.EvalSymlinks(path)
			if e != nil {
				return e
			}
			rel, e := filepath.Rel(root, resolved)
			if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("repository note escapes vault: %s", path)
			}
			body, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			count := len(uncheckedItem.FindAll(body, -1))
			r.Observed.Remaining += count
			if count > 0 {
				r.Observed.Notes++
			}
			return nil
		})
		if e != nil {
			return fail(e)
		}
	} else if !os.IsNotExist(e) {
		return fail(e)
	}
	current, _ := data["visible_coverage"].(map[string]any)
	for _, field := range []struct {
		name     string
		expected int
	}{{"remaining_verification_items", r.Observed.Remaining}, {"notes_with_verifications", r.Observed.Notes}} {
		number, ok := current[field.name].(json.Number)
		actual, err := strconv.Atoi(string(number))
		if !ok || err != nil || actual != field.expected {
			r.Errors = append(r.Errors, fmt.Sprintf("visible_coverage.%s: expected %d", field.name, field.expected))
		}
	}
	paths, ok := current["paths"].([]any)
	home := false
	for _, v := range paths {
		if name, ok := v.(string); ok && name == "00-Home.md" {
			home = true
		}
	}
	if !ok || len(paths) < 2 || !home {
		r.Errors = append(r.Errors, "visible_coverage.paths: Home and linked coverage note required")
		paths = nil
	}
	seen := map[string]bool{}
	for _, v := range paths {
		name, ok := v.(string)
		if !ok {
			r.Errors = append(r.Errors, fmt.Sprintf("invalid summary path: %v", v))
			continue
		}
		p := name
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		p, e = filepath.EvalSymlinks(p)
		if e != nil {
			r.Errors = append(r.Errors, "invalid summary path: "+name)
			continue
		}
		relative, e := filepath.Rel(root, p)
		if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			r.Errors = append(r.Errors, "invalid summary path: "+name)
			continue
		}
		st, e := os.Stat(p)
		if e != nil || !st.Mode().IsRegular() {
			r.Errors = append(r.Errors, "invalid summary path: "+name)
			continue
		}
		if seen[p] {
			r.Errors = append(r.Errors, "duplicate summary path: "+name)
			continue
		}
		seen[p] = true
		body, e := os.ReadFile(p)
		if e != nil {
			return fail(e)
		}
		matches := closureCount.FindAllStringSubmatch(strings.ReplaceAll(string(body), "*", ""), -1)
		if len(matches) != 1 || matches[0][1] != strconv.Itoa(r.Observed.Remaining) || matches[0][2] != strconv.Itoa(r.Observed.Notes) {
			r.Errors = append(r.Errors, "summary count missing, ambiguous or stale: "+name)
		}
	}
	if len(r.Errors) > 0 {
		return r, ErrIssues
	}
	r.Status = "pass"
	return r, nil
}
func runClosure(args []string, out io.Writer) error {
	f := flag.NewFlagSet("map-closure", flag.ContinueOnError)
	f.SetOutput(out)
	vault := f.String("vault", "", "vault directory")
	checkpoint := f.String("checkpoint", "", "existing checkpoint JSON")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *vault == "" || *checkpoint == "" || f.NArg() != 0 {
		return errors.New("usage: check map-closure --vault PATH --checkpoint PATH")
	}
	r, e := MapClosure(*vault, *checkpoint)
	if err := json.NewEncoder(out).Encode(r); err != nil {
		return err
	}
	return e
}
