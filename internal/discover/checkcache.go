package discover

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"knowledge-os/internal/config"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// A note check depends only on the note, the kos build, the vault's discovery store and platform
// snapshots, its identity and workspace, the repository notes' names and the refs of every checkout
// (so fetching a commit or cloning a missing repository also invalidates a pending result). When none of them changed, the stored result is reused, so
// a sync verify that runs again after one fix checks only what changed. KOS_NO_CACHE=1 disables it; KOS_CACHE_DIR moves it.
//
// Duplicated paragraphs depend on every other note, so they are computed on each call, outside the stored result.
func checkNote(vault, notePath, repoOverride string) (noteCheck, error) {
	return checkNoteMode(vault, notePath, repoOverride, true)
}

// Read-only reviews recheck source evidence without reading or writing the result cache.
func checkNoteMode(vault, notePath, repoOverride string, useCache bool) (noteCheck, error) {
	r, e := checkNoteCached(vault, notePath, repoOverride, useCache)
	if e != nil {
		return r, e
	}
	full := notePath
	if !filepath.IsAbs(full) {
		full = filepath.Join(vault, notePath)
	}
	if text, e := os.ReadFile(full); e == nil {
		if dups := duplicateParagraphs(vault, full, text); len(dups) > 0 {
			r.Issues = append(r.Issues, dups...)
			r.OK, r.Verified = false, false
		}
	}
	return r, nil
}

func checkNoteCached(vault, notePath, repoOverride string, useCache bool) (noteCheck, error) {
	if !useCache {
		return checkNoteFresh(vault, notePath, repoOverride)
	}
	full := notePath
	if !filepath.IsAbs(full) {
		full = filepath.Join(vault, notePath)
	}
	text, e := os.ReadFile(full)
	dir := checkCacheDir()
	if e != nil || dir == "" || os.Getenv("KOS_NO_CACHE") != "" {
		return checkNoteFresh(vault, notePath, repoOverride)
	}
	sum := sha256.New()
	for _, part := range []string{checkFingerprint(vault), notePath, repoOverride, string(text)} {
		sum.Write([]byte(part))
		sum.Write([]byte{0})
	}
	path := filepath.Join(dir, hex.EncodeToString(sum.Sum(nil))+".json")
	if b, e := os.ReadFile(path); e == nil {
		var r noteCheck
		if json.Unmarshal(b, &r) == nil {
			return r, nil
		}
	}
	r, e := checkNoteFresh(vault, notePath, repoOverride)
	if e != nil {
		return r, e
	}
	if b, e := json.Marshal(r); e == nil && os.MkdirAll(dir, 0o700) == nil {
		_ = os.WriteFile(path, b, 0o600)
	}
	return r, nil
}

func checkCacheDir() string {
	if d := os.Getenv("KOS_CACHE_DIR"); d != "" {
		return filepath.Join(d, "note-checks")
	}
	d, e := os.UserCacheDir()
	if e != nil {
		return ""
	}
	return filepath.Join(d, "knowledge-os", "note-checks")
}

// checkFingerprint summarizes every input of a note check other than the note itself, once per
// process and vault.
func checkFingerprint(vault string) string {
	memoMu.Lock()
	fp, ok := fingerprintMemo[vault]
	memoMu.Unlock()
	if ok {
		return fp
	}
	h := sha256.New()
	add := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	if exe, e := os.Executable(); e == nil {
		if st, e := os.Stat(exe); e == nil {
			add(exe, st.ModTime().String(), strconv.FormatInt(st.Size(), 10))
		}
	}
	files := []string{"instance.yaml", ".knowledge-os-config.yaml"}
	_ = filepath.WalkDir(filepath.Join(vault, "90-Meta", "discovery"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(vault, p)
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	for _, f := range files {
		b, _ := os.ReadFile(filepath.Join(vault, f))
		add(f, string(b))
	}
	if notes, e := repoNotes(vault); e == nil {
		keys := make([]string, 0, len(notes))
		for k := range notes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(k, notes[k])
		}
	}
	// Topic, event and integration notes, by name and frontmatter: G4-relation matches resources against them.
	for _, dir := range []string{"25-Topics", "40-Integraciones"} {
		_ = filepath.WalkDir(filepath.Join(vault, dir), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				b, _ := os.ReadFile(p)
				add(p, fmt.Sprint(frontmatterOf(b)))
			}
			return nil
		})
	}
	// Git worktrees keep their refs in the common directory. The .git indirection,
	// common-dir marker, remote config and shallow boundary also affect source binding.
	visited := map[string]bool{}
	if paths, e := config.Checkouts(vault); e == nil {
		for _, checkout := range paths {
			add(checkout)
			gitPath := filepath.Join(checkout, ".git")
			gitDir := gitPath
			if st, e := os.Stat(gitPath); e == nil && !st.IsDir() {
				b, _ := os.ReadFile(gitPath)
				add(gitPath, string(b))
				if target, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir: "); ok {
					gitDir = target
					if !filepath.IsAbs(gitDir) {
						gitDir = filepath.Join(checkout, gitDir)
					}
				}
			}
			if canonical, e := filepath.EvalSymlinks(gitDir); e == nil {
				gitDir = canonical
			}
			common := gitDir
			if b, e := os.ReadFile(filepath.Join(gitDir, "commondir")); e == nil {
				add("commondir", string(b))
				common = strings.TrimSpace(string(b))
				if !filepath.IsAbs(common) {
					common = filepath.Join(gitDir, common)
				}
			}
			dirs := []string{gitDir}
			if filepath.Clean(common) != gitDir {
				dirs = append(dirs, filepath.Clean(common))
			}
			for _, dir := range dirs {
				if canonical, e := filepath.EvalSymlinks(dir); e == nil {
					dir = canonical
				}
				if visited[dir] {
					continue
				}
				visited[dir] = true
				for _, f := range []string{"HEAD", "packed-refs", "config", "config.worktree", "shallow", "FETCH_HEAD"} {
					b, e := os.ReadFile(filepath.Join(dir, f))
					add(dir, f, string(b), fmt.Sprint(e))
				}
				_ = filepath.WalkDir(filepath.Join(dir, "refs"), func(p string, d fs.DirEntry, err error) error {
					if err == nil && !d.IsDir() {
						b, e := os.ReadFile(p)
						add(p, string(b), fmt.Sprint(e))
					}
					return nil
				})
			}
		}
	} else {
		add("checkout resolution", e.Error())
	}
	fp = hex.EncodeToString(h.Sum(nil))
	memoMu.Lock()
	fingerprintMemo[vault] = fp
	memoMu.Unlock()
	return fp
}
