package investigation

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func relativeSafe(s string) bool {
	if s == "" || filepath.IsAbs(s) || strings.Contains(s, "\\") {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." {
			return false
		}
	}
	return true
}
func retainedLinks(dir string, removed map[string]bool) error {
	markdown := regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	html := regexp.MustCompile(`(?i)\b(?:href|src)\s*=\s*['"]([^'"]+)['"]`)
	return filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if !contains([]string{".md", ".html", ".htm"}, strings.ToLower(filepath.Ext(p))) {
			return nil
		}
		s, e := readText(p)
		if e != nil {
			return e
		}
		matches := append(markdown.FindAllStringSubmatch(s, -1), html.FindAllStringSubmatch(s, -1)...)
		for _, m := range matches {
			raw := strings.Fields(m[1])
			if len(raw) == 0 {
				continue
			}
			u, e := url.Parse(strings.Trim(raw[0], "<> "))
			if e != nil {
				return e
			}
			if u.Scheme != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
				continue
			}
			target, e := url.PathUnescape(u.Path)
			if e != nil {
				return e
			}
			rel, e := filepath.Rel(dir, filepath.Clean(filepath.Join(filepath.Dir(p), filepath.FromSlash(target))))
			if e != nil {
				return e
			}
			if removed[filepath.ToSlash(rel)] {
				return fmt.Errorf("published document links to retained resource: %s", rel)
			}
		}
		return nil
	})
}
func publishCase(root string, o options) (any, error) {
	if e := o.required("id", "expected-public-sha256", "expected-tree-sha256", "source"); e != nil {
		return nil, e
	}
	if e := eventInput(o.get("source")); e != nil {
		return nil, e
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
		if r.visibility != "unpublished" {
			return nil, errors.New("investigation already published")
		}
		if digest([]byte(r.text)) != o.get("expected-public-sha256") {
			return nil, errors.New("stale public snapshot")
		}
		source := filepath.Dir(r.path)
		before, entries, e := Snapshot(source)
		if e != nil {
			return nil, e
		}
		if before != o.get("expected-tree-sha256") {
			return nil, errors.New("stale case tree")
		}
		dest := filepath.Join(root, o.get("id"))
		if _, e = os.Lstat(dest); !errors.Is(e, os.ErrNotExist) {
			return nil, errors.New("published destination already exists")
		}
		retained := []string{}
		removed := map[string]bool{}
		for _, p := range o["retain-local"] {
			if !relativeSafe(p) || p == "investigation.md" {
				return nil, errors.New("invalid retained path")
			}
			if _, e = os.Lstat(filepath.Join(source, filepath.FromSlash(p))); e != nil {
				return nil, e
			}
			for _, prev := range retained {
				if p == prev || strings.HasPrefix(p, prev+"/") || strings.HasPrefix(prev, p+"/") {
					return nil, errors.New("overlapping retained paths")
				}
			}
			retained = append(retained, p)
			for _, item := range entries {
				if item.Type == "file" && (item.Path == p || strings.HasPrefix(item.Path, p+"/")) {
					unit := unitID(item.Path)
					if (unit != "" && declares(r.text, unit)) || strings.Contains(r.text, item.Path) {
						return nil, errors.New("registered resource must stay published")
					}
					removed[item.Path] = true
				}
			}
		}
		retainedTarget := ""
		if len(retained) > 0 {
			private := filepath.Join(filepath.Dir(root), ".investigations-private")
			local := filepath.Join(private, o.get("id"), "local")
			for _, p := range []string{private, filepath.Dir(local), local, filepath.Join(local, "publish-retained")} {
				if e = noSymlink(p, true); e != nil {
					return nil, e
				}
			}
			retainedTarget = filepath.Join(local, "publish-retained", before)
			if _, e = os.Lstat(retainedTarget); !errors.Is(e, os.ErrNotExist) {
				return nil, errors.New("retained snapshot already exists")
			}
		}
		staging, e := os.MkdirTemp(root, ".native-tree-")
		if e != nil {
			return nil, e
		}
		pending, committed := false, false
		defer func() {
			if !pending {
				os.RemoveAll(staging)
			}
			if !pending && !committed && retainedTarget != "" {
				os.RemoveAll(retainedTarget)
			}
		}()
		if e = copyTree(source, staging); e != nil {
			return nil, e
		}
		h, _, e := Snapshot(staging)
		if e != nil {
			return nil, e
		}
		if h != before {
			return nil, errors.New("case changed during publication staging")
		}
		for _, p := range retained {
			sp := filepath.Join(staging, filepath.FromSlash(p))
			rp := filepath.Join(retainedTarget, filepath.FromSlash(p))
			if e = os.MkdirAll(filepath.Dir(rp), 0700); e != nil {
				return nil, e
			}
			st, e := os.Stat(sp)
			if e != nil {
				return nil, e
			}
			if st.IsDir() {
				if e = copyTree(sp, rp); e != nil {
					return nil, e
				}
				a, _, e := Snapshot(sp)
				if e != nil {
					return nil, e
				}
				b, _, e := Snapshot(rp)
				if e != nil || a != b {
					return nil, errors.New("retained copy differs")
				}
				if e = os.RemoveAll(sp); e != nil {
					return nil, e
				}
			} else {
				b, e := os.ReadFile(sp)
				if e != nil {
					return nil, e
				}
				if e = os.WriteFile(rp, b, st.Mode().Perm()); e != nil {
					return nil, e
				}
				if e = os.Chmod(rp, st.Mode()); e != nil {
					return nil, e
				}
				if e = os.Remove(sp); e != nil {
					return nil, e
				}
			}
		}
		if e = retainedLinks(staging, removed); e != nil {
			return nil, e
		}
		_, stagedEntries, e := Snapshot(staging)
		if e != nil {
			return nil, e
		}
		if errs := artifactErrors(stagedEntries, r.text, true, nil); len(errs) > 0 {
			return nil, errors.New(strings.Join(errs, "; "))
		}
		if errs := largeErrors(staging, stagedEntries, r.text, nil); len(errs) > 0 {
			return nil, errors.New(strings.Join(errs, "; "))
		}
		if e = publicSafe(staging); e != nil {
			return nil, e
		}
		updated, e := setField(r.text, "updated-at", ts)
		if e != nil {
			return nil, e
		}
		updated, e = setField(updated, "artifact-integrity", "sha256-v1")
		if e != nil {
			return nil, e
		}
		action := "Published investigation"
		if isSpanish(updated) {
			action = "Investigación publicada"
		}
		updated, e = appendHistory(updated, attributed(ts, action, who, o.get("source"), isSpanish(updated)))
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
		if e = commitTree(root, source, dest, staging, before, after); e != nil {
			return nil, e
		}
		pending = false
		committed = true
		return map[string]any{"status": "published", "id": o.get("id"), "path": filepath.Join(dest, "investigation.md"), "visibility": "published", "public_sha256": digest([]byte(updated)), "tree_sha256": after, "retained_local": retained, "warnings": []string{}}, nil
	})
}
