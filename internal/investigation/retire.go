package investigation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type retirementJournal struct {
	Version                   int
	Source, Backup, Tree      string
	LedgerBefore, LedgerAfter []byte
}

func retirementJournalPath(root string) string {
	return filepath.Join(filepath.Dir(root), ".investigations-private", ".native-retirement-transaction.json")
}
func replayRetirement(root string, j retirementJournal) error {
	if j.Version != 1 || j.Source == j.Backup || !idPattern.MatchString(filepath.Base(j.Source)) || !strings.HasPrefix(filepath.Base(j.Backup), ".native-tree-") {
		return errors.New("invalid retirement transaction")
	}
	source, e := treeTarget(root, j.Source)
	if e != nil {
		return e
	}
	backup, e := treeTarget(root, j.Backup)
	if e != nil {
		return e
	}
	ledger := filepath.Join(root, "retired.md")
	current, e := currentBytes(ledger)
	if e != nil {
		return e
	}
	if j.LedgerAfter != nil && !sameBytes(current, j.LedgerBefore) && !sameBytes(current, j.LedgerAfter) {
		return errors.New("retirement ledger changed during transaction")
	}
	sh, e := treeHash(source)
	if e != nil {
		return e
	}
	bh, e := treeHash(backup)
	if e != nil {
		return e
	}
	if sh == "absent" && bh == "absent" {
		if j.LedgerAfter == nil || sameBytes(current, j.LedgerAfter) {
			return os.Remove(retirementJournalPath(root))
		}
		return errors.New("retirement source missing before ledger commit")
	}
	if bh == "absent" {
		if sh != j.Tree {
			return errors.New("retirement source changed")
		}
		if e = os.Rename(source, backup); e != nil {
			return e
		}
		bh = j.Tree
	} else if sh != "absent" {
		return errors.New("retirement source reappeared")
	}
	if bh != j.Tree {
		return errors.New("retirement backup changed")
	}
	if j.LedgerAfter != nil {
		if e = atomicWrite(ledger, j.LedgerAfter); e != nil {
			return e
		}
	}
	if e = os.RemoveAll(backup); e != nil {
		return e
	}
	return os.Remove(retirementJournalPath(root))
}
func recoverRetirement(root string) error {
	b, e := currentBytes(retirementJournalPath(root))
	if e != nil || b == nil {
		return e
	}
	var j retirementJournal
	if e = json.Unmarshal(b, &j); e != nil {
		return e
	}
	return replayRetirement(root, j)
}
func commitRetirement(root, source, tree string, before, after []byte) error {
	if b, e := currentBytes(retirementJournalPath(root)); e != nil {
		return e
	} else if b != nil {
		return errors.New("pending retirement transaction")
	}
	tmp, e := os.MkdirTemp(filepath.Dir(source), ".native-tree-retire-")
	if e != nil {
		return e
	}
	if e = os.Remove(tmp); e != nil {
		return e
	}
	rel := func(p string) string { s, _ := filepath.Rel(filepath.Dir(root), p); return filepath.ToSlash(s) }
	j := retirementJournal{1, rel(source), rel(tmp), tree, before, after}
	b, e := json.Marshal(j)
	if e != nil {
		return e
	}
	if e = noSymlink(filepath.Dir(retirementJournalPath(root)), true); e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(retirementJournalPath(root)), 0700); e != nil {
		return e
	}
	if e = atomicWrite(retirementJournalPath(root), b); e != nil {
		return e
	}
	return replayRetirement(root, j)
}
func retireCase(root string, o options) (any, error) {
	if o.get("authorized") != "true" {
		return nil, errors.New("retirement requires explicit --authorized")
	}
	if e := o.required("id", "expected-public-sha256", "reason", "source", "dependency-review", "absorption-review", "summary"); e != nil {
		return nil, e
	}
	for _, k := range []string{"reason", "source", "dependency-review", "absorption-review", "summary", "destination"} {
		for _, v := range o[k] {
			if e := eventInput(v); e != nil {
				return nil, e
			}
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
		if r.get("status") != "closed" {
			return nil, errors.New("only closed investigations can retire")
		}
		if digest([]byte(r.text)) != o.get("expected-public-sha256") {
			return nil, errors.New("stale public snapshot")
		}
		if e = eventInput(r.get("title")); e != nil {
			return nil, e
		}
		errs, e := validateVault(root)
		if e != nil {
			return nil, e
		}
		if len(errs) > 0 {
			return nil, errors.New(strings.Join(errs, "; "))
		}
		source := filepath.Dir(r.path)
		tree, entries, e := Snapshot(source)
		if e != nil {
			return nil, e
		}
		if r.visibility == "unpublished" {
			if e = commitRetirement(root, source, tree, nil, nil); e != nil {
				return nil, e
			}
			return map[string]any{"status": "retired", "id": o.get("id"), "visibility": "unpublished"}, nil
		}
		commit := o.get("snapshot-commit")
		if !regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`).MatchString(commit) {
			return nil, errors.New("retirement snapshot requires full Git commit")
		}
		resolved, e := git(root, "rev-parse", "--verify", commit+"^{commit}")
		if e != nil || strings.TrimSpace(string(resolved)) != commit {
			return nil, errors.New("retirement snapshot is not exact commit")
		}
		if _, e = git(root, "merge-base", "--is-ancestor", commit, "HEAD"); e != nil {
			return nil, errors.New("snapshot commit is not reachable from HEAD")
		}
		top, e := git(root, "rev-parse", "--show-toplevel")
		if e != nil {
			return nil, e
		}
		relative, e := filepath.Rel(strings.TrimSpace(string(top)), source)
		if e != nil || strings.HasPrefix(relative, "..") {
			return nil, errors.New("case outside Git repository")
		}
		relative = filepath.ToSlash(relative)
		status, e := git(root, "status", "--porcelain=v1", "--untracked-files=all", "--", ":(top,literal)"+relative)
		if e != nil || len(status) > 0 {
			return nil, errors.New("retirement case has uncommitted changes")
		}
		listing, e := git(root, "ls-tree", "--full-tree", "-rz", commit, "--", relative)
		if e != nil {
			return nil, e
		}
		seen := map[string]bool{}
		for _, item := range bytes.Split(listing, []byte{0}) {
			if len(item) == 0 {
				continue
			}
			meta, path, ok := bytes.Cut(item, []byte{'\t'})
			if !ok {
				return nil, errors.New("invalid Git tree")
			}
			parts := strings.Fields(string(meta))
			if len(parts) != 3 || parts[1] != "blob" || !contains([]string{"100644", "100755"}, parts[0]) {
				return nil, errors.New("retirement snapshot has non-regular file")
			}
			suffix := strings.TrimPrefix(string(path), relative+"/")
			if suffix == string(path) || !relativeSafe(suffix) {
				return nil, errors.New("snapshot file outside case")
			}
			p := filepath.Join(source, filepath.FromSlash(suffix))
			b, e := os.ReadFile(p)
			if e != nil {
				return nil, e
			}
			blob, e := git(root, "cat-file", "blob", parts[2])
			if e != nil {
				return nil, e
			}
			st, e := os.Stat(p)
			if e != nil {
				return nil, e
			}
			if !bytes.Equal(b, blob) || (st.Mode().Perm()&0111 != 0) != (parts[0] == "100755") {
				return nil, errors.New("snapshot differs from case bytes or executable bit")
			}
			seen[suffix] = true
		}
		files := 0
		for _, v := range entries {
			if v.Type == "file" {
				files++
				if !seen[v.Path] {
					return nil, errors.New("snapshot omits published file")
				}
			}
		}
		if files == 0 || files != len(seen) {
			return nil, errors.New("retirement snapshot file set differs")
		}
		destinations := o["destination"]
		if destinations == nil {
			destinations = []string{}
		}
		lineage := r.lineage()
		if lineage == nil {
			lineage = []string{}
		}
		entry := map[string]any{"id": o.get("id"), "title": r.get("title"), "snapshot-commit": commit, "public-sha256": o.get("expected-public-sha256"), "retired-at": ts, "recorded-by": who, "reason": o.get("reason"), "source": o.get("source"), "dependency-review": o.get("dependency-review"), "absorption-review": o.get("absorption-review"), "summary": o.get("summary"), "destinations": destinations, "consolidated-from": lineage}
		before, e := currentBytes(filepath.Join(root, "retired.md"))
		if e != nil {
			return nil, e
		}
		content := append([]byte{}, before...)
		if before == nil {
			content = []byte("# Retired investigations\n\n")
		}
		line, e := pythonJSON(entry)
		if e != nil {
			return nil, e
		}
		content = append(content, []byte("- ")...)
		content = append(content, line...)
		content = append(content, '\n')
		if e = commitRetirement(root, source, tree, before, content); e != nil {
			return nil, e
		}
		view, e := retirementView(root, entry)
		if e != nil {
			return nil, fmt.Errorf("retired but could not derive Git state: %w", e)
		}
		view["status"] = "retired"
		return view, nil
	})
}
