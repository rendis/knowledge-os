package investigation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

func withoutArtifacts(text string, targets map[string]bool) (string, error) {
	lines := strings.SplitAfter(text, "\n")
	start, end, count := -1, len(lines), 0
	for i, l := range lines {
		if strings.HasPrefix(l, "## ") && contains(requiredSections[2], strings.TrimSpace(l[3:])) {
			start = i + 1
			count++
		}
	}
	if count != 1 {
		return "", errors.New("one artifact register is required")
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	kept := []string{}
	skip := false
	for _, l := range lines[start:end] {
		m := artifactHeader.FindStringSubmatch(strings.TrimSuffix(l, "\n"))
		if m != nil {
			skip = targets[m[1]]
		}
		if !skip {
			kept = append(kept, l)
		}
	}
	return strings.Join(lines[:start], "") + strings.Trim(strings.Join(kept, ""), "\n") + "\n" + strings.Join(lines[end:], ""), nil
}
func validateStaged(root string, old record, staging, text string) error {
	fields, e := frontmatter(text)
	if e != nil {
		return e
	}
	r := record{filepath.Join(staging, "investigation.md"), text, old.visibility, fields}
	errs := []string{}
	for _, s := range validateRecord(r) {
		if strings.HasSuffix(s, ": directory does not match id") {
			continue
		}
		errs = append(errs, s)
	}
	previous, e := validateVault(root)
	if e != nil {
		return e
	}
	prefix := filepath.Base(filepath.Dir(old.path)) + "/investigation.md: "
	for _, s := range previous {
		if !strings.HasPrefix(s, prefix) {
			errs = append(errs, s)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("staged case validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}
func saveResources(root string, o options) (any, error) {
	if e := o.required("id", "candidate-dir", "expected-tree-sha256", "expected-candidate-tree-sha256", "source"); e != nil {
		return nil, e
	}
	if e := eventInput(o.get("source")); e != nil {
		return nil, e
	}
	targets := map[string]bool{}
	for _, t := range o["target"] {
		if !regexp.MustCompile(`^A-[0-9]{3,}$`).MatchString(t) {
			return nil, errors.New("resource targets must be A-NNN")
		}
		targets[t] = true
	}
	if len(targets) == 0 {
		return nil, errors.New("resource target is required")
	}
	candidate, e := filepath.Abs(o.get("candidate-dir"))
	if e != nil {
		return nil, e
	}
	hash, entries, e := Snapshot(candidate)
	if e != nil {
		return nil, e
	}
	if hash != o.get("expected-candidate-tree-sha256") {
		return nil, errors.New("stale candidate tree")
	}
	text, e := readText(filepath.Join(candidate, "investigation.md"))
	if e != nil {
		return nil, e
	}
	if secretPattern.MatchString(text) {
		return nil, errors.New("secret-bearing candidate")
	}
	fields, e := frontmatter(text)
	if e != nil {
		return nil, e
	}
	if fields["id"] != o.get("id") {
		return nil, errors.New("candidate identity mismatch")
	}
	for t := range targets {
		if !declares(text, t) {
			return nil, errors.New("resource target missing from register")
		}
	}
	who, e := identity(root)
	if e != nil {
		return nil, e
	}
	ts, e := eventTime(o)
	if e != nil {
		return nil, e
	}
	return mutationGate(root, func() (any, error) {
		r, e := locate(root, o.get("id"))
		if e != nil {
			return nil, e
		}
		source := filepath.Dir(r.path)
		resolved, e := filepath.EvalSymlinks(candidate)
		if e != nil {
			return nil, e
		}
		if resolved == source || strings.HasPrefix(resolved, source+string(filepath.Separator)) {
			return nil, errors.New("resource candidate must be outside live case")
		}
		before, oldEntries, e := Snapshot(source)
		if e != nil {
			return nil, e
		}
		if before != o.get("expected-tree-sha256") {
			return nil, errors.New("stale case tree")
		}
		if hash == before {
			return map[string]any{"status": "unchanged", "id": o.get("id"), "tree_sha256": before}, nil
		}
		for _, k := range []string{"artifact-integrity", "status", "blocked-on", "closure-outcome", "resume-to"} {
			if !reflect.DeepEqual(fields[k], r.fields[k]) {
				return nil, fmt.Errorf("%s cannot change through resource save", k)
			}
		}
		a, e := withoutArtifacts(text, targets)
		if e != nil {
			return nil, e
		}
		b, e := withoutArtifacts(r.text, targets)
		if e != nil {
			return nil, e
		}
		if a != b {
			return nil, errors.New("resource note changes exceed targeted entries")
		}
		old, new := map[string]Entry{}, map[string]Entry{}
		paths := map[string]bool{}
		for _, v := range oldEntries {
			old[v.Path] = v
			paths[v.Path] = true
		}
		for _, v := range entries {
			new[v.Path] = v
			paths[v.Path] = true
		}
		changed := []string{}
		artifactChange := false
		for p := range paths {
			if old[p] == new[p] {
				continue
			}
			changed = append(changed, p)
			if p == "investigation.md" {
				continue
			}
			if p == "artifacts" && (new[p].Type == "directory" || old[p].Type == "directory") {
				continue
			}
			parts := strings.Split(p, "/")
			allowed := false
			if len(parts) >= 2 && parts[0] == "artifacts" {
				for t := range targets {
					allowed = allowed || parts[1] == t || strings.HasPrefix(parts[1], t+"-")
				}
			}
			if !allowed {
				return nil, fmt.Errorf("non-target resource changed: %s", p)
			}
			artifactChange = true
		}
		if !artifactChange {
			return nil, errors.New("resource save requires artifact change")
		}
		sort.Strings(changed)
		if errs := artifactErrors(entries, text, true, targets); len(errs) > 0 {
			return nil, errors.New(strings.Join(errs, "; "))
		}
		if errs := largeErrors(candidate, entries, text, targets); len(errs) > 0 {
			return nil, errors.New(strings.Join(errs, "; "))
		}
		staging, e := os.MkdirTemp(filepath.Dir(source), ".native-tree-")
		if e != nil {
			return nil, e
		}
		pending := false
		defer func() {
			if !pending {
				os.RemoveAll(staging)
			}
		}()
		if e = copyTree(candidate, staging); e != nil {
			return nil, e
		}
		staged, _, e := Snapshot(staging)
		if e != nil {
			return nil, e
		}
		if staged != hash {
			return nil, errors.New("candidate changed during staging")
		}
		updated, e := setField(text, "updated-at", ts)
		if e != nil {
			return nil, e
		}
		sorted := []string{}
		for t := range targets {
			sorted = append(sorted, t)
		}
		sort.Strings(sorted)
		refs := []string{}
		for _, t := range sorted {
			refs = append(refs, "`"+t+"`")
		}
		action := "Reviewed resources "
		if isSpanish(text) {
			action = "Artefactos revisados "
		}
		updated, e = appendHistory(updated, attributed(ts, action+strings.Join(refs, ", "), who, o.get("source"), isSpanish(text)))
		if e != nil {
			return nil, e
		}
		if e = atomicWrite(filepath.Join(staging, "investigation.md"), []byte(updated)); e != nil {
			return nil, e
		}
		if e = validateStaged(root, r, staging, updated); e != nil {
			return nil, e
		}
		after, _, e := Snapshot(staging)
		if e != nil {
			return nil, e
		}
		pending = true
		if e = commitTree(root, source, source, staging, before, after); e != nil {
			return nil, e
		}
		pending = false
		return map[string]any{"status": "resources_saved", "id": o.get("id"), "tree_sha256": after, "changed": changed, "warnings": []string{}}, nil
	})
}
