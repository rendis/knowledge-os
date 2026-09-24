package investigation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func addLineage(text, id string) (string, error) {
	f, e := frontmatter(text)
	if e != nil {
		return "", e
	}
	if a, ok := f["consolidated-from"].([]string); ok && contains(a, id) {
		return text, nil
	}
	lines := strings.Split(text, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] == "consolidated-from:" {
			j := i + 1
			for j < len(lines) && strings.HasPrefix(lines[j], "  - ") {
				j++
			}
			lines = append(lines[:j], append([]string{"  - " + id}, lines[j:]...)...)
			return strings.Join(lines, "\n"), nil
		}
		if lines[i] == "---" {
			lines = append(lines[:i], append([]string{"consolidated-from:", "  - " + id}, lines[i:]...)...)
			return strings.Join(lines, "\n"), nil
		}
	}
	return "", errors.New("frontmatter missing")
}
func archiveCase(source, dest, ts string) error {
	_, entries, e := Snapshot(source)
	if e != nil {
		return e
	}
	if e = copyTree(source, dest); e != nil {
		return e
	}
	files := []map[string]string{}
	for _, v := range entries {
		p := filepath.Join(dest, filepath.FromSlash(v.Path))
		if v.Type == "directory" {
			if e = os.Chmod(p, 0700); e != nil {
				return e
			}
		} else {
			if e = os.Chmod(p, 0600); e != nil {
				return e
			}
			files = append(files, map[string]string{"path": v.Path, "sha256": v.SHA256})
		}
	}
	manifest := map[string]any{"retired-id": filepath.Base(source), "captured-at": ts, "files": files}
	b, e := json.MarshalIndent(manifest, "", "  ")
	if e != nil {
		return e
	}
	if e = atomicWrite(filepath.Join(dest, "manifest.json"), append(b, '\n')); e != nil {
		return e
	}
	if errs := validateArchive(dest); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
func consolidateCase(root string, o options) (any, error) {
	if e := o.required("canonical", "retire", "expected-canonical-sha256", "expected-retire-sha256"); e != nil {
		return nil, e
	}
	if o.get("canonical") == o.get("retire") {
		return nil, errors.New("canonical and retiring IDs must differ")
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
		canonical, e := locate(root, o.get("canonical"))
		if e != nil {
			return nil, e
		}
		retiring, e := locate(root, o.get("retire"))
		if e != nil {
			return nil, e
		}
		if canonical.visibility != retiring.visibility {
			return nil, errors.New("consolidation requires same visibility")
		}
		if digest([]byte(canonical.text)) != o.get("expected-canonical-sha256") || digest([]byte(retiring.text)) != o.get("expected-retire-sha256") {
			return nil, errors.New("stale consolidation snapshot")
		}
		source, retiringDir := filepath.Dir(canonical.path), filepath.Dir(retiring.path)
		mappingName := "consolidation-" + o.get("retire") + "-mapping.md"
		mapping, e := readText(filepath.Join(source, "artifacts", mappingName))
		if e != nil {
			return nil, e
		}
		learning := regexp.MustCompile(`(?m)^learning-assessment: (preserved|reset)$`).FindStringSubmatch(mapping)
		if !strings.Contains(mapping, "retired-id: "+o.get("retire")) || !regexp.MustCompile(`(?m)^drafts: (?:none|reconciled|stale)$`).MatchString(mapping) || learning == nil {
			return nil, errors.New("invalid semantic consolidation mapping")
		}
		archiveRel := filepath.Join("artifacts", "consolidated", o.get("retire"))
		if _, e = os.Lstat(filepath.Join(source, archiveRel)); !errors.Is(e, os.ErrNotExist) {
			return nil, errors.New("consolidation archive exists")
		}
		before, _, e := Snapshot(source)
		if e != nil {
			return nil, e
		}
		retiringHash, _, e := Snapshot(retiringDir)
		if e != nil {
			return nil, e
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
		if e = copyTree(source, staging); e != nil {
			return nil, e
		}
		if e = os.MkdirAll(filepath.Dir(filepath.Join(staging, archiveRel)), 0700); e != nil {
			return nil, e
		}
		if e = archiveCase(retiringDir, filepath.Join(staging, archiveRel), ts); e != nil {
			return nil, e
		}
		updated, e := addLineage(canonical.text, o.get("retire"))
		if e != nil {
			return nil, e
		}
		updated, e = setField(updated, "updated-at", ts)
		if e != nil {
			return nil, e
		}
		prior := canonical.get("learning-outcome")
		if prior == "" {
			prior = "missing"
		}
		assessment := "learning assessment preserved as `" + prior + "`"
		if learning[1] == "reset" {
			updated, e = setField(updated, "learning-outcome", "not-evaluated")
			if e != nil {
				return nil, e
			}
			assessment = "learning assessment reset from `" + prior + "` to `not-evaluated`"
		}
		action := "Consolidated `" + o.get("retire") + "`; archive "
		if isSpanish(updated) {
			action = "Consolidado `" + o.get("retire") + "`; archivo "
		}
		action += "`artifacts/consolidated/" + o.get("retire") + "/`; mapping `artifacts/" + mappingName + "`; " + assessment
		updated, e = appendHistory(updated, attributed(ts, action, who, "semantic mapping `artifacts/"+mappingName+"`", isSpanish(updated)))
		if e != nil {
			return nil, e
		}
		if e = atomicWrite(filepath.Join(staging, "investigation.md"), []byte(updated)); e != nil {
			return nil, e
		}
		if e = validateStaged(root, canonical, staging, updated); e != nil {
			return nil, e
		}
		after, _, e := Snapshot(staging)
		if e != nil {
			return nil, e
		}
		backup := staging + ".backup"
		rel := func(p string) string { s, _ := filepath.Rel(filepath.Dir(root), p); return filepath.ToSlash(s) }
		j := treeJournal{1, rel(source), rel(source), rel(staging), rel(backup), before, after, rel(retiringDir), retiringHash}
		if h, _, e := Snapshot(retiringDir); e != nil || h != retiringHash {
			return nil, errors.New("retiring case changed during staging")
		}
		b, e := json.Marshal(j)
		if e != nil {
			return nil, e
		}
		if e = noSymlink(filepath.Dir(treeJournalPath(root)), true); e != nil {
			return nil, e
		}
		if e = os.MkdirAll(filepath.Dir(treeJournalPath(root)), 0700); e != nil {
			return nil, e
		}
		pending = true
		if e = atomicWrite(treeJournalPath(root), b); e != nil {
			return nil, e
		}
		if e = replayTree(root, j); e != nil {
			return nil, e
		}
		pending = false
		return map[string]any{"status": "consolidated", "canonical": o.get("canonical"), "retired": o.get("retire"), "warnings": []string{}}, nil
	})
}
