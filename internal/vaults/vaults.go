// Package vaults keeps the machine's list of knowledge-os vaults: every vault a kos command works on is
// remembered, each entry is checked at its recorded path whenever it is listed, and `kernel update --all`
// brings the outdated ones to the kernel this kos carries.
package vaults

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"knowledge-os/internal/config"
	"knowledge-os/internal/kernel"
	"knowledge-os/internal/release"
)

// Entry is one remembered vault: where it was last seen and the cell it holds.
type Entry struct {
	Path  string `json:"path"`
	Cell  string `json:"cell"`
	Added string `json:"added"`
}

type registry struct {
	Vaults []Entry `json:"vaults"`
}

// skipTemporary keeps vaults under the system's temporary directories (test fixtures, disposable
// clones) out of the list; tests of this package turn it off.
var skipTemporary = true

// File is the list's location: KOS_VAULTS_FILE, else the user's configuration directory.
func File() string {
	if p := os.Getenv("KOS_VAULTS_FILE"); p != "" {
		return p
	}
	dir, e := os.UserConfigDir()
	if e != nil {
		return ""
	}
	return filepath.Join(dir, "knowledge-os", "vaults.json")
}

func load() (registry, error) {
	var r registry
	b, e := os.ReadFile(File())
	if os.IsNotExist(e) {
		return r, nil
	}
	if e != nil {
		return r, e
	}
	if e := json.Unmarshal(b, &r); e != nil {
		return r, fmt.Errorf("%s: %w", File(), e)
	}
	return r, nil
}

func save(r registry) error {
	p := File()
	if p == "" {
		return errors.New("no configuration directory for the vault list")
	}
	sort.Slice(r.Vaults, func(i, j int) bool { return r.Vaults[i].Path < r.Vaults[j].Path })
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		return e
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	tmp := p + ".tmp"
	if e := os.WriteFile(tmp, append(b, '\n'), 0o644); e != nil {
		return e
	}
	return os.Rename(tmp, p)
}

func canonical(p string) string {
	abs, e := filepath.Abs(p)
	if e != nil {
		return filepath.Clean(p)
	}
	if real, e := filepath.EvalSymlinks(abs); e == nil {
		return real
	}
	return abs
}

func temporary(p string) bool {
	for _, t := range []string{os.TempDir(), "/tmp"} {
		for _, root := range []string{filepath.Clean(t), canonical(t)} {
			if root != "" && root != "/" && (p == root || strings.HasPrefix(p, root+string(filepath.Separator))) {
				return true
			}
		}
	}
	return false
}

// cellOf returns the cell name when dir holds a knowledge-os vault (a kernel lock and a valid instance).
func cellOf(dir string) (string, bool) {
	if _, e := os.Stat(filepath.Join(dir, kernel.LockName)); e != nil {
		return "", false
	}
	inst, e := config.LoadInstance(dir)
	if e != nil {
		return "", false
	}
	cell, _ := inst["cell"].(map[string]any)
	name, _ := cell["name"].(string)
	return name, name != ""
}

// locked runs f while holding the list's lock file, so concurrent kos commands do not drop each
// other's entries. A lock older than ten seconds is left over from a killed process and is broken.
func locked(f func() error) error {
	p := File()
	if p == "" {
		return errors.New("no configuration directory for the vault list")
	}
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		return e
	}
	lock := p + ".lock"
	for start := time.Now(); ; time.Sleep(10 * time.Millisecond) {
		h, e := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if e == nil {
			h.Close()
			break
		}
		if st, se := os.Stat(lock); se == nil && time.Since(st.ModTime()) > 10*time.Second {
			os.Remove(lock)
			continue
		}
		if time.Since(start) > 3*time.Second {
			return fmt.Errorf("the vault list is locked by another kos (%s)", lock)
		}
	}
	defer os.Remove(lock)
	return f()
}

// Register remembers the vault at root. It never fails a command: the list is a convenience.
func Register(root string) {
	if os.Getenv("CI") != "" {
		return
	}
	_, _ = remember(canonical(root), skipTemporary)
}

// remember adds the vault at p, or moves the entry of the same cell whose recorded path no longer
// holds it. It returns new, known or relocated.
func remember(p string, skipTemp bool) (string, error) {
	if skipTemp && temporary(p) {
		return "", nil
	}
	cell, ok := cellOf(p)
	if !ok {
		return "", fmt.Errorf("%s is not a knowledge-os vault: %s and a valid instance.yaml are required", p, kernel.LockName)
	}
	var status string
	e := locked(func() (e error) {
		status, e = rememberCell(p, cell)
		return e
	})
	return status, e
}

func rememberCell(p, cell string) (string, error) {
	r, e := load()
	if e != nil {
		return "", e
	}
	lost := -1
	for i, v := range r.Vaults {
		if v.Path == p {
			if v.Cell == cell {
				return "known", nil
			}
			r.Vaults[i].Cell = cell
			return "known", save(r)
		}
		if got, ok := cellOf(v.Path); v.Cell == cell && (!ok || got != cell) {
			if lost >= 0 {
				lost = -2 // more than one lost entry of this cell: which one moved is unknown
			} else if lost == -1 {
				lost = i
			}
		}
	}
	if lost >= 0 {
		r.Vaults[lost].Path = p
		return "relocated", save(r)
	}
	r.Vaults = append(r.Vaults, Entry{Path: p, Cell: cell, Added: time.Now().Format("2006-01-02")})
	return "new", save(r)
}

// State is an entry checked at its recorded path.
type State struct {
	Path        string `json:"path"`
	Cell        string `json:"cell"`
	Found       string `json:"found"` // yes | missing | not-a-vault | other-cell
	Branch      string `json:"branch,omitempty"`
	VaultKernel string `json:"vault_kernel,omitempty"`
	Kernel      string `json:"kernel,omitempty"` // current | outdated | newer, against the kernel kos carries
	Fix         string `json:"fix,omitempty"`
}

func check(v Entry) State {
	s := State{Path: v.Path, Cell: v.Cell, Found: "yes"}
	st, e := os.Stat(v.Path)
	cell, ok := cellOf(v.Path)
	switch {
	case e != nil || !st.IsDir():
		s.Found = "missing"
	case !ok:
		s.Found = "not-a-vault"
	case cell != v.Cell:
		s.Found = "other-cell"
	}
	if s.Found != "yes" {
		s.Fix = fmt.Sprintf("find it with `kos vaults scan <dir>`, or forget it with `kos vaults remove %q`", v.Path)
		return s
	}
	s.Branch = git(v.Path, "rev-parse", "--abbrev-ref", "HEAD")
	if l, e := kernel.ReadLock(v.Path); e == nil {
		s.VaultKernel = l.KernelVersion
	}
	switch c := release.Compare(s.VaultKernel, kernel.Version()); {
	case c < 0:
		s.Kernel = "outdated"
	case c > 0:
		s.Kernel, s.Fix = "newer", "kos update"
	default:
		s.Kernel = "current"
	}
	return s
}

// List checks every remembered vault at its recorded path; nothing is changed.
func List() ([]State, error) {
	r, e := load()
	if e != nil {
		return nil, e
	}
	out := []State{}
	for _, v := range r.Vaults {
		out = append(out, check(v))
	}
	return out, nil
}

func git(dir string, args ...string) string {
	out, e := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if e != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var skipDirs = map[string]bool{"node_modules": true, "vendor": true, "Library": true}

// Scan remembers every vault under dir, up to depth levels below it.
func Scan(dir string, depth int) ([]map[string]string, error) {
	root := canonical(dir)
	if st, e := os.Stat(root); e != nil || !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	found := []map[string]string{}
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if p != root && (strings.HasPrefix(d.Name(), ".") || skipDirs[d.Name()]) {
			return filepath.SkipDir
		}
		if rel, _ := filepath.Rel(root, p); rel != "." && strings.Count(rel, string(filepath.Separator))+1 > depth {
			return filepath.SkipDir
		}
		if _, ok := cellOf(p); ok {
			status, e := remember(p, false)
			if e != nil {
				return e
			}
			found = append(found, map[string]string{"path": p, "status": status})
			return filepath.SkipDir
		}
		return nil
	})
	return found, e
}

// UpdateAll brings every remembered vault whose kernel is older than this kos's to it. Vaults that are
// current, newer or not found are listed and left alone; one vault's refusal does not stop the rest.
func UpdateAll(o kernel.Options, commit bool) (map[string]any, error) {
	states, e := List()
	if e != nil {
		return nil, e
	}
	results, failed := []map[string]any{}, 0
	for _, s := range states {
		item := map[string]any{"path": s.Path, "cell": s.Cell, "branch": s.Branch, "vault_kernel": s.VaultKernel}
		results = append(results, item)
		switch {
		case s.Found != "yes":
			item["action"], item["reason"], item["next"] = "skipped", "not found at the recorded path: "+s.Found, s.Fix
			continue
		case s.Kernel != "outdated":
			item["action"], item["reason"] = "skipped", "kernel "+s.Kernel
			continue
		}
		if commit {
			if reason := commitBlocked(s); reason != "" {
				item["action"], item["reason"] = refused(o.DryRun), reason
				failed++
				continue
			}
		}
		res, e := kernel.Update(s.Path, kernel.Options{DryRun: o.DryRun})
		if e != nil {
			item["action"], item["reason"] = "refused", e.Error()
			for _, k := range []string{"conflicts", "foreign", "unsafe", "next"} {
				if res != nil && res[k] != nil {
					item[k] = res[k]
				}
			}
			failed++
			continue
		}
		item["changes"] = res["changes"]
		if o.DryRun {
			if reason := blocked(res); reason != "" {
				item["action"], item["reason"] = "would-refuse", reason
				for _, k := range []string{"conflicts", "foreign", "unsafe"} {
					item[k] = res[k]
				}
				failed++
				continue
			}
			item["action"], item["diff"] = "would-update", res["diff"]
			continue
		}
		item["action"] = "updated"
		if commit {
			sha, e := commitKernel(s.Path, res)
			if e != nil {
				item["action"], item["reason"] = "updated-not-committed", e.Error()
				failed++
				continue
			}
			item["commit"] = sha
		}
	}
	out := map[string]any{"kos_kernel": kernel.Version(), "vaults": results}
	switch {
	case o.DryRun:
		out["next"] = "kos kernel update --all --commit"
	case !commit:
		out["next"] = "review git diff in each updated vault and commit it as one change"
	default:
		out["next"] = "push each committed vault through the team's usual review"
	}
	if failed > 0 {
		if o.DryRun {
			return out, fmt.Errorf("%d vault(s) would be refused; see reason", failed)
		}
		return out, fmt.Errorf("%d vault(s) not updated or not committed; see reason", failed)
	}
	return out, nil
}

func refused(dry bool) string {
	if dry {
		return "would-refuse"
	}
	return "refused"
}

// blocked is why a previewed update would be refused: the checks Update runs before writing.
func blocked(res map[string]any) string {
	for _, c := range []struct{ key, reason string }{
		{"unsafe", "managed targets are not regular files or sit under a symbolic link"},
		{"foreign", "cell files sit where the kernel now ships its own: rename or move them"},
		{"conflicts", "managed files changed in the vault: keep cell logic in cell-owned files (a single-vault update with --force restores them)"},
	} {
		if list, _ := res[c.key].([]string); len(list) > 0 {
			return c.reason
		}
	}
	return ""
}

func commitBlocked(s State) string {
	switch {
	case s.Branch == "":
		return "not a Git checkout: update it without --commit"
	case strings.HasPrefix(s.Branch, "sync/"):
		return "on a publication branch (" + s.Branch + "): finish or leave it first"
	case git(s.Path, "status", "--porcelain", "--untracked-files=no") != "":
		return "tracked changes are pending: commit or stash them first"
	}
	return ""
}

// commitKernel stages only what the update wrote (kernel.Written) and commits it; the tracked tree was
// clean before the update, so nothing else is taken along.
func commitKernel(dir string, res map[string]any) (string, error) {
	want := map[string]bool{}
	written, _ := res["written"].([]string)
	for _, p := range written {
		want[p] = true
	}
	out, e := exec.Command("git", "-C", dir, "-c", "core.quotePath=false", "status", "--porcelain", "-z", "--no-renames", "--untracked-files=all").Output()
	if e != nil {
		return "", fmt.Errorf("git status: %w", e)
	}
	paths := []string{}
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) > 3 && want[string(rec[3:])] {
			paths = append(paths, string(rec[3:]))
		}
	}
	if len(paths) == 0 {
		return "", errors.New("nothing to commit")
	}
	if b, e := exec.Command("git", append([]string{"-C", dir, "add", "-A", "--"}, paths...)...).CombinedOutput(); e != nil {
		return "", fmt.Errorf("git add: %s", strings.TrimSpace(string(b)))
	}
	msg := "chore: update the knowledge-os kernel to " + kernel.Version()
	if b, e := exec.Command("git", "-C", dir, "commit", "-q", "-m", msg).CombinedOutput(); e != nil {
		return "", fmt.Errorf("git commit: %s", strings.TrimSpace(string(b)))
	}
	return git(dir, "rev-parse", "--short", "HEAD"), nil
}

const Help = `vaults [list]              every vault this machine remembers, checked at its recorded path:
                           found (yes | missing | not-a-vault | other-cell), branch, kernel
                           (current | outdated | newer than the one kos carries) and the fix
vaults add PATH            remember a vault (any kos command on a vault remembers it too)
vaults remove PATH         forget a vault
vaults scan DIR [--depth N]  remember every vault under DIR (default depth 6); a vault found
                           elsewhere replaces the lost entry of the same cell`

// Run serves the vaults command.
func Run(args []string, out io.Writer) error {
	op := "list"
	if len(args) > 0 {
		op, args = args[0], args[1:]
	}
	switch op {
	case "list":
		states, e := List()
		if e != nil {
			return e
		}
		res := map[string]any{"kos_kernel": kernel.Version(), "file": File(), "vaults": states}
		if next := Next(states); next != "" {
			res["next"] = next
		}
		return emit(out, res)
	case "add":
		if len(args) != 1 {
			return errors.New("usage: vaults add PATH")
		}
		status, e := remember(canonical(args[0]), false)
		if e != nil {
			return e
		}
		return emit(out, map[string]any{"path": canonical(args[0]), "status": status})
	case "remove":
		if len(args) != 1 {
			return errors.New("usage: vaults remove PATH")
		}
		e := locked(func() error {
			r, e := load()
			if e != nil {
				return e
			}
			kept := []Entry{}
			for _, v := range r.Vaults {
				if v.Path != args[0] && v.Path != canonical(args[0]) {
					kept = append(kept, v)
				}
			}
			if len(kept) == len(r.Vaults) {
				return fmt.Errorf("%s is not in the list", args[0])
			}
			r.Vaults = kept
			return save(r)
		})
		if e != nil {
			return e
		}
		return emit(out, map[string]any{"path": args[0], "status": "removed"})
	case "scan":
		fset := flag.NewFlagSet("vaults scan", flag.ContinueOnError)
		fset.SetOutput(io.Discard)
		depth := fset.Int("depth", 6, "")
		dirs := []string{}
		for len(args) > 0 {
			if e := fset.Parse(args); e != nil {
				return e
			}
			if args = fset.Args(); len(args) > 0 {
				dirs, args = append(dirs, args[0]), args[1:]
			}
		}
		if len(dirs) != 1 {
			return errors.New("usage: vaults scan DIR [--depth N]")
		}
		found, e := Scan(dirs[0], *depth)
		if e != nil {
			return e
		}
		return emit(out, map[string]any{"dir": canonical(dirs[0]), "found": found, "next": "kos vaults"})
	case "--help", "-h", "help":
		_, e := io.WriteString(out, Help+"\n")
		return e
	default:
		return fmt.Errorf("unknown vaults command %q", op)
	}
}

// Next is the step the list calls for: update the outdated vaults, or find the lost ones.
func Next(states []State) string {
	outdated, lost := 0, 0
	for _, s := range states {
		if s.Kernel == "outdated" {
			outdated++
		}
		if s.Found != "yes" {
			lost++
		}
	}
	switch {
	case len(states) == 0:
		return "no vault remembered yet: `kos vaults scan <dir>` finds them"
	case outdated > 0:
		return fmt.Sprintf("%d vault(s) carry an older kernel: `kos kernel update --all --dry-run`, then `--commit` on approval", outdated)
	case lost > 0:
		return fmt.Sprintf("%d vault(s) are not at their recorded path: see fix", lost)
	}
	return ""
}

func emit(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}
