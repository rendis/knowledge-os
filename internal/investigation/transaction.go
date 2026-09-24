package investigation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type fileChange struct {
	Path   string `json:"path"`
	Before []byte `json:"before"`
	After  []byte `json:"after"`
}
type writeJournal struct {
	Version int          `json:"version"`
	Changes []fileChange `json:"changes"`
}

func journalPath(root string) string {
	return filepath.Join(filepath.Dir(root), ".investigations-private", ".native-transaction.json")
}
func currentBytes(p string) ([]byte, error) {
	if e := noSymlink(p, true); e != nil {
		return nil, e
	}
	b, e := os.ReadFile(p)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	return b, e
}
func journalTarget(root, rel string) (string, error) {
	p := filepath.FromSlash(rel)
	parts := strings.Split(rel, "/")
	if filepath.IsAbs(p) || len(parts) != 3 || !contains([]string{"investigations", ".investigations", ".investigations-private"}, parts[0]) || !idPattern.MatchString(parts[1]) {
		return "", errors.New("transaction path outside investigation files")
	}
	expected := "investigation.md"
	if parts[0] == ".investigations-private" {
		expected = "private.md"
	}
	if parts[2] != expected {
		return "", errors.New("invalid transaction target")
	}
	base := filepath.Dir(root)
	for _, part := range parts {
		base = filepath.Join(base, part)
		if e := noSymlink(base, true); e != nil {
			return "", e
		}
	}
	return base, nil
}
func replayJournal(root string, j writeJournal) error {
	if j.Version != 1 || len(j.Changes) == 0 || len(j.Changes) > 2 {
		return errors.New("invalid investigation transaction journal")
	}
	seen := map[string]bool{}
	// Verify every target before making progress; never overwrite a concurrent edit.
	for _, c := range j.Changes {
		if seen[c.Path] {
			return errors.New("duplicate transaction target")
		}
		seen[c.Path] = true
		p, e := journalTarget(root, c.Path)
		if e != nil {
			return e
		}
		b, e := currentBytes(p)
		if e != nil {
			return e
		}
		if !sameBytes(b, c.Before) && !sameBytes(b, c.After) {
			return fmt.Errorf("transaction recovery conflict at %s", c.Path)
		}
	}
	for _, c := range j.Changes {
		p, e := journalTarget(root, c.Path)
		if e != nil {
			return e
		}
		b, e := currentBytes(p)
		if e != nil {
			return e
		}
		if sameBytes(b, c.After) {
			continue
		}
		if c.After == nil {
			if e = os.Remove(p); e != nil && !errors.Is(e, os.ErrNotExist) {
				return e
			}
		} else {
			if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
				return e
			}
			if e = atomicWrite(p, c.After); e != nil {
				return e
			}
		}
	}
	return os.Remove(journalPath(root))
}
func recoverJournal(root string) error {
	p := journalPath(root)
	b, e := currentBytes(p)
	if e != nil {
		return e
	}
	if b == nil {
		return nil
	}
	var j writeJournal
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(&j); e != nil {
		return e
	}
	return replayJournal(root, j)
}
func commitChanges(root string, changes []fileChange) error {
	dir := filepath.Dir(journalPath(root))
	if e := noSymlink(dir, true); e != nil {
		return e
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	if b, e := currentBytes(journalPath(root)); e != nil {
		return e
	} else if b != nil {
		return errors.New("pending transaction must be recovered first")
	}
	j := writeJournal{1, changes}
	b, e := json.Marshal(j)
	if e != nil {
		return e
	}
	if e = atomicWrite(journalPath(root), b); e != nil {
		return e
	}
	return replayJournal(root, j)
}

func sameBytes(a, b []byte) bool { return (a == nil) == (b == nil) && bytes.Equal(a, b) }
