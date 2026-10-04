package kernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// The directory is both the backup store and an exclusive writer lock. A directory left by an
// interrupted update or failed rollback is never overwritten by another update.
const updateDir = ".kos-kernel-update"

type updateFile struct {
	Path    string      `json:"path"`
	Exists  bool        `json:"existed"`
	Mode    os.FileMode `json:"mode"`
	content []byte
	link    string
	mtime   time.Time
}

type updateTransaction struct {
	vault, dir string
	files      []updateFile
	newDirs    []string
}

// beginUpdate captures only the paths Apply can change. Backups live on the same filesystem, so
// rollback restores old files by rename, including their permissions, without rewriting their bytes.
func beginUpdate(vault string, paths []string) (_ *updateTransaction, err error) {
	tx := &updateTransaction{vault: vault, dir: filepath.Join(vault, updateDir)}
	if e := os.Mkdir(tx.dir, 0o700); e != nil {
		if os.IsExist(e) {
			return nil, fmt.Errorf("another kernel update is active or needs recovery at %s; existing recovery files were kept", tx.dir)
		}
		return nil, e
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.cleanup("kernel preparation failed; vault files were not changed"))
		}
	}()
	// The cell's ignore rules may be old or restored by rollback. Keep all recovery files local.
	if e := writeFileMode(filepath.Join(tx.dir, ".gitignore"), []byte("*\n"), 0o600); e != nil {
		return nil, e
	}
	seen, dirs := map[string]bool{}, map[string]bool{}
	for _, rel := range paths {
		if seen[rel] {
			continue
		}
		seen[rel] = true
		full := filepath.Join(vault, filepath.FromSlash(rel))
		for dir := filepath.Dir(full); dir != vault; dir = filepath.Dir(dir) {
			if _, e := os.Lstat(dir); os.IsNotExist(e) {
				dirs[dir] = true
			} else if e != nil {
				return nil, e
			}
		}
		st, e := os.Lstat(full)
		if os.IsNotExist(e) {
			tx.files = append(tx.files, updateFile{Path: rel})
			continue
		}
		if e != nil {
			return nil, e
		}
		if contains(bases, rel) {
			continue // existing Bases are cell-owned, including broken symlinks
		}
		f := updateFile{Path: rel, Exists: true, Mode: st.Mode(), mtime: st.ModTime()}
		backup := filepath.Join(tx.dir, "before", filepath.FromSlash(rel))
		if e := os.MkdirAll(filepath.Dir(backup), 0o700); e != nil {
			return nil, e
		}
		if st.Mode()&os.ModeSymlink != 0 {
			f.link, e = os.Readlink(full)
			if e == nil {
				e = os.Symlink(f.link, backup)
			}
		} else {
			f.content, e = os.ReadFile(full)
			if e == nil {
				e = writeFileMode(backup, f.content, st.Mode())
			}
			if e == nil {
				e = os.Chtimes(backup, st.ModTime(), st.ModTime())
			}
		}
		if e != nil {
			return nil, e
		}
		tx.files = append(tx.files, f)
	}
	for dir := range dirs {
		tx.newDirs = append(tx.newDirs, dir)
	}
	slices.SortFunc(tx.newDirs, func(a, b string) int { return len(b) - len(a) })
	newDirs := []string{}
	for _, dir := range tx.newDirs {
		rel, e := filepath.Rel(vault, dir)
		if e != nil {
			return nil, e
		}
		newDirs = append(newDirs, filepath.ToSlash(rel))
	}
	manifest, e := json.MarshalIndent(map[string]any{"files": tx.files, "new_directories": newDirs}, "", "  ")
	if e != nil {
		return nil, e
	}
	if e := writeFileMode(filepath.Join(tx.dir, "manifest.json"), append(manifest, '\n'), 0o600); e != nil {
		return nil, e
	}
	return tx, nil
}

// cleanup keeps the local ignore rule until every recovery file is removed. A failed cleanup still
// blocks another writer and must not expose retained backups to git add.
func (tx *updateTransaction) cleanup(state string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%s; cleanup failed at %s (remove this directory before retrying): %w", state, tx.dir, err)
		}
	}()
	st, e := os.Lstat(tx.dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !st.IsDir() {
		return fmt.Errorf("%s is not a kernel update directory", tx.dir)
	}
	entries, e := os.ReadDir(tx.dir)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if entry.Name() == ".gitignore" {
			continue
		}
		if e := os.RemoveAll(filepath.Join(tx.dir, entry.Name())); e != nil {
			return e
		}
	}
	if e := os.Remove(filepath.Join(tx.dir, ".gitignore")); e != nil && !os.IsNotExist(e) {
		return e
	}
	return os.Remove(tx.dir)
}

func (f updateFile) unchanged(full string) bool {
	st, e := os.Lstat(full)
	if os.IsNotExist(e) {
		return !f.Exists
	}
	if e != nil || !f.Exists || st.Mode() != f.Mode || !st.ModTime().Equal(f.mtime) {
		return false
	}
	if f.Mode&os.ModeSymlink != 0 {
		link, e := os.Readlink(full)
		return e == nil && link == f.link
	}
	b, e := os.ReadFile(full)
	return e == nil && bytes.Equal(b, f.content)
}

func (tx *updateTransaction) rollback() error {
	problems := []error{}
	for _, f := range tx.files {
		full := filepath.Join(tx.vault, filepath.FromSlash(f.Path))
		if f.unchanged(full) {
			continue // a failed atomic write may have left the original intact in an unwritable directory
		}
		if e := safePath(tx.vault, f.Path); e != nil {
			problems = append(problems, e)
			continue
		}
		var e error
		if f.Exists {
			e = os.Rename(filepath.Join(tx.dir, "before", filepath.FromSlash(f.Path)), full)
		} else {
			e = os.Remove(full)
		}
		if e != nil && (f.Exists || !os.IsNotExist(e)) {
			problems = append(problems, fmt.Errorf("restore %s: %w", f.Path, e))
		}
	}
	for _, dir := range tx.newDirs {
		if e := os.Remove(dir); e != nil && !os.IsNotExist(e) {
			problems = append(problems, fmt.Errorf("remove new directory %s: %w", dir, e))
		}
	}
	return errors.Join(problems...)
}
