package discover

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"knowledge-os/internal/config"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// A note check depends only on the note, the kos build, the vault's discovery store and platform
// snapshots, its identity and workspace, the repository notes' names and the refs of every checkout
// (so fetching a commit or cloning a missing repository also invalidates a pending result). When none of them changed, the stored result is reused, so
// a sync verify that runs again after one fix checks only what changed. KOS_NO_CACHE=1 disables it; KOS_CACHE_DIR moves it.
func checkNote(vault, notePath, repoOverride string) (noteCheck, error) {
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
	if b, e := json.Marshal(r); e == nil && os.MkdirAll(dir, 0o755) == nil {
		_ = os.WriteFile(path, b, 0o644)
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
	// Every checkout under the workspace roots, by its refs on disk: a fetch or a new commit changes them.
	// Reading the files avoids hundreds of git processes per check.
	if w, e := config.Workspace(vault); e == nil {
		sc, _ := w["source_context"].(config.Object)
		for _, r := range asList(sc["roots"]) {
			root, _ := asMap(r)["path"].(string)
			entries, _ := os.ReadDir(root)
			for _, d := range entries {
				gitDir := filepath.Join(root, d.Name(), ".git")
				if st, e := os.Stat(gitDir); e != nil || !st.IsDir() {
					continue
				}
				add(d.Name())
				for _, f := range []string{"HEAD", "packed-refs"} {
					b, _ := os.ReadFile(filepath.Join(gitDir, f))
					add(f, string(b))
				}
				_ = filepath.WalkDir(filepath.Join(gitDir, "refs"), func(p string, d fs.DirEntry, err error) error {
					if err == nil && !d.IsDir() {
						b, _ := os.ReadFile(p)
						add(p, string(b))
					}
					return nil
				})
			}
		}
	}
	fp = hex.EncodeToString(h.Sum(nil))
	memoMu.Lock()
	fingerprintMemo[vault] = fp
	memoMu.Unlock()
	return fp
}
