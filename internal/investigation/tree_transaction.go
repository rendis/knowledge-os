package investigation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type treeJournal struct {
	Version                                             int `json:"version"`
	Source, Destination, Staging, Backup, Before, After string
	Remove, RemoveHash                                  string
}

func treeJournalPath(root string) string {
	return filepath.Join(filepath.Dir(root), ".investigations-private", ".native-tree-transaction.json")
}
func treeTarget(root, relative string) (string, error) {
	parts := strings.Split(relative, "/")
	if len(parts) != 2 || !contains([]string{"investigations", ".investigations"}, parts[0]) || (!idPattern.MatchString(parts[1]) && !strings.HasPrefix(parts[1], ".native-tree-")) || strings.Contains(parts[1], "..") || strings.ContainsAny(parts[1], "\\/") {
		return "", errors.New("invalid transaction tree path")
	}
	p := filepath.Join(filepath.Dir(root), filepath.FromSlash(relative))
	if e := noSymlink(filepath.Dir(p), false); e != nil {
		return "", e
	}
	if e := noSymlink(p, true); e != nil {
		return "", e
	}
	return p, nil
}
func treeHash(p string) (string, error) {
	h, _, e := Snapshot(p)
	if errors.Is(e, os.ErrNotExist) {
		return "absent", nil
	}
	return h, e
}
func replayTree(root string, j treeJournal) error {
	if j.Version != 1 || j.Source == j.Staging || j.Source == j.Backup || j.Destination == j.Staging || j.Destination == j.Backup || j.Staging == j.Backup {
		return errors.New("invalid tree transaction")
	}
	if j.Remove != "" {
		if j.Remove == j.Source || j.Remove == j.Destination || !idPattern.MatchString(filepath.Base(j.Remove)) {
			return errors.New("invalid consolidation removal")
		}
		p, e := treeTarget(root, j.Remove)
		if e != nil {
			return e
		}
		h, e := treeHash(p)
		if e != nil {
			return e
		}
		if h != "absent" && h != j.RemoveHash {
			return errors.New("consolidation retiring case changed")
		}
	}
	source, e := treeTarget(root, j.Source)
	if e != nil {
		return e
	}
	dest, e := treeTarget(root, j.Destination)
	if e != nil {
		return e
	}
	staging, e := treeTarget(root, j.Staging)
	if e != nil {
		return e
	}
	backup, e := treeTarget(root, j.Backup)
	if e != nil {
		return e
	}
	bh, e := treeHash(backup)
	if e != nil {
		return e
	}
	dh, e := treeHash(dest)
	if e != nil {
		return e
	}
	sh, e := treeHash(staging)
	if e != nil {
		return e
	}
	if bh == "absent" && dh == j.After && sh == "absent" {
		return finishTreeRemoval(root, j)
	}
	if bh == "absent" {
		h, e := treeHash(source)
		if e != nil {
			return e
		}
		if h != j.Before || sh != j.After || (dest != source && dh != "absent") {
			return errors.New("tree transaction source or staged bytes changed")
		}
		if e = os.Rename(source, backup); e != nil {
			return e
		}
		bh = j.Before
		dh = "absent"
	}
	if bh != j.Before {
		return errors.New("tree transaction backup differs from reviewed bytes")
	}
	if dh == "absent" {
		if sh != j.After {
			return errors.New("tree transaction staged bytes changed")
		}
		if e = os.Rename(staging, dest); e != nil {
			return e
		}
	} else if dh != j.After {
		return errors.New("tree transaction destination conflict")
	}
	if e = os.RemoveAll(backup); e != nil {
		return e
	}
	return finishTreeRemoval(root, j)
}
func recoverTree(root string) error {
	b, e := currentBytes(treeJournalPath(root))
	if e != nil || b == nil {
		return e
	}
	var j treeJournal
	if e = json.Unmarshal(b, &j); e != nil {
		return e
	}
	return replayTree(root, j)
}
func commitTree(root, source, dest, staging, before, after string) error {
	if b, e := currentBytes(treeJournalPath(root)); e != nil {
		return e
	} else if b != nil {
		return errors.New("tree transaction already pending")
	}
	backup := staging + ".backup"
	relative := func(p string) string { r, _ := filepath.Rel(filepath.Dir(root), p); return filepath.ToSlash(r) }
	j := treeJournal{1, relative(source), relative(dest), relative(staging), relative(backup), before, after, "", ""}
	b, e := json.Marshal(j)
	if e != nil {
		return e
	}
	if e = noSymlink(filepath.Dir(treeJournalPath(root)), true); e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(treeJournalPath(root)), 0700); e != nil {
		return e
	}
	if e = atomicWrite(treeJournalPath(root), b); e != nil {
		return e
	}
	return replayTree(root, j)
}
func copyTree(source, dest string) error {
	if _, _, e := Snapshot(source); e != nil {
		return e
	}
	return filepath.WalkDir(source, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		st, e := d.Info()
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(source, p)
		if e != nil {
			return e
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			if e = os.MkdirAll(target, 0700); e != nil {
				return e
			}
			return os.Chmod(target, st.Mode())
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("cannot copy non-regular entry %s", p)
		}
		in, e := os.Open(p)
		if e != nil {
			return e
		}
		defer in.Close()
		opened, e := in.Stat()
		if e != nil {
			return e
		}
		if !os.SameFile(st, opened) || !opened.Mode().IsRegular() {
			return errors.New("source file changed during copy")
		}
		out, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, e = io.Copy(out, in)
		if e == nil {
			e = out.Chmod(st.Mode())
		}
		if e == nil {
			e = out.Sync()
		}
		ce := out.Close()
		if e != nil {
			return e
		}
		return ce
	})
}

func finishTreeRemoval(root string, j treeJournal) error {
	if j.Remove != "" {
		if j.Remove == j.Destination || j.Remove == j.Source || !idPattern.MatchString(filepath.Base(j.Remove)) {
			return errors.New("invalid consolidation removal")
		}
		p, e := treeTarget(root, j.Remove)
		if e != nil {
			return e
		}
		h, e := treeHash(p)
		if e != nil {
			return e
		}
		if h != "absent" {
			if h != j.RemoveHash {
				return errors.New("consolidation retiring case changed")
			}
			if e = os.RemoveAll(p); e != nil {
				return e
			}
		}
	}
	return os.Remove(treeJournalPath(root))
}
