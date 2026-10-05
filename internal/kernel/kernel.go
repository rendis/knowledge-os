// Package kernel installs and updates the kernel a cell vault carries: the router, the generic 90-Meta
// files, the skills and the specialist definitions, taken from the payload inside this binary.
package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	knowledgeos "knowledge-os"
	"knowledge-os/internal/config"
)

const (
	LockName    = ".knowledge-os.lock.yaml"
	hashComment = "# gitleaks:allow -- managed SHA-256 digest"
	catalogPath = "90-Meta/vault-catalog.yaml"
	lockFormat  = "5"
)

// Revision and Dirty describe the distribution commit this binary was built from (set at release).
var (
	Revision = "unknown"
	Dirty    = "true"
)

// Version is the kernel version this binary carries.
func Version() string {
	b, _ := fs.ReadFile(knowledgeos.Payload, "kernel/VERSION")
	return strings.TrimSpace(string(b))
}

// Lock is what the distribution installed in a cell: the kernel version and the hash of every file it owns.
type Lock struct {
	Version, KernelVersion, Revision string
	Dirty                            bool
	Adapters                         []string
	Hashes                           map[string]string
}

func ReadLock(vault string) (Lock, error) {
	b, e := os.ReadFile(filepath.Join(vault, LockName))
	if e != nil {
		return Lock{}, e
	}
	l := Lock{Hashes: map[string]string{}, Dirty: true}
	section := ""
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		value := func() string { _, v, _ := strings.Cut(line, ":"); return strings.Trim(strings.TrimSpace(v), `"`) }
		switch {
		case strings.HasPrefix(line, "version:"):
			l.Version = value()
		case strings.HasPrefix(line, "kernel_version:"):
			l.KernelVersion = value()
		case strings.HasPrefix(line, "distribution_revision:"):
			l.Revision = value()
		case strings.HasPrefix(line, "distribution_dirty:"):
			l.Dirty = strings.EqualFold(value(), "true")
		case strings.HasPrefix(line, "adapters:"):
			section = "adapters"
		case strings.HasPrefix(line, "managed_hashes:"):
			section = "hashes"
		case strings.HasPrefix(line, "runtime_release:"):
			section = ""
		case section == "adapters" && strings.HasPrefix(line, "- "):
			l.Adapters = append(l.Adapters, strings.TrimSpace(line[2:]))
		case section == "hashes" && strings.Contains(line, ":"):
			k, v, _ := strings.Cut(line, ":")
			l.Hashes[strings.Trim(strings.TrimSpace(k), `"`)] = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), hashComment))
		}
	}
	if l.KernelVersion == "" {
		return l, fmt.Errorf("%s has no kernel_version", LockName)
	}
	return l, nil
}

func (l Lock) dump() string {
	var b strings.Builder
	fmt.Fprintf(&b, "version: %q\nkernel_version: %q\ndistribution_revision: %q\ndistribution_dirty: %t\nadapters:\n", lockFormat, l.KernelVersion, l.Revision, l.Dirty)
	if len(l.Adapters) == 0 {
		b.WriteString("  []\n")
	}
	for _, a := range l.Adapters {
		b.WriteString("  - " + a + "\n")
	}
	b.WriteString("managed_hashes:\n")
	keys := make([]string, 0, len(l.Hashes))
	for k := range l.Hashes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %q: %s %s\n", k, l.Hashes[k], hashComment)
	}
	return b.String()
}

func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// Sources returns every file the distribution owns in a cell with the given adapters, by vault path.
func Sources(adapters []string) (map[string][]byte, error) {
	list, e := fs.ReadFile(knowledgeos.Payload, "MANAGED_PATHS")
	if e != nil {
		return nil, e
	}
	out := map[string][]byte{}
	addTree := func(root, target string) error {
		return fs.WalkDir(knowledgeos.Payload, root, func(p string, d fs.DirEntry, e error) error {
			if e != nil || d.IsDir() || strings.Contains(p, "__pycache__") || strings.HasSuffix(p, ".pyc") || d.Name() == ".DS_Store" {
				return e
			}
			b, e := fs.ReadFile(knowledgeos.Payload, p)
			out[path.Join(target, strings.TrimPrefix(p, root+"/"))] = b
			return e
		})
	}
	for _, raw := range strings.Split(string(list), "\n") {
		entry := strings.TrimSpace(raw)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		src := "kernel/" + strings.TrimSuffix(entry, "/")
		if strings.HasSuffix(entry, "/") {
			if e := addTree(src, strings.TrimSuffix(entry, "/")); e != nil {
				return nil, fmt.Errorf("managed directory %s: %w", entry, e)
			}
		} else {
			b, e := fs.ReadFile(knowledgeos.Payload, src)
			if e != nil {
				return nil, fmt.Errorf("managed file %s: %w", entry, e)
			}
			out[entry] = b
		}
	}
	for _, a := range adapters {
		skills, e := fs.ReadDir(knowledgeos.Payload, "adapters/"+a)
		if e != nil {
			return nil, fmt.Errorf("unknown adapter %s", a)
		}
		for _, s := range skills {
			if s.IsDir() {
				if e := addTree("adapters/"+a+"/"+s.Name(), ".agents/skills/"+s.Name()); e != nil {
					return nil, e
				}
			}
		}
	}
	delete(out, catalogPath) // a cell's catalog is never payload
	return out, nil
}

// Change is one file an update adds, changes or removes.
type Change struct {
	Path   string `json:"path"`
	Action string `json:"action"` // add | change | remove
}

// Plan is what an update would do, computed without writing.
type Plan struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	Changes   []Change `json:"changes"`
	Unchanged int      `json:"unchanged"`
	// Conflicts are managed files changed in the vault since the lock recorded them, or, on a first
	// installation, files the distribution would own that already exist with other content.
	Conflicts []string `json:"conflicts"`
	// Foreign are cell files at a path a newer kernel now ships (a cell skill with a kernel skill's
	// name): the lock never recorded them, so nothing is written, even with --force, until they move.
	Foreign []string `json:"foreign,omitempty"`
	// Unsafe are targets that are not regular files or sit under a symbolic link: nothing is written,
	// even with --force, until they are moved explicitly.
	Unsafe   []string `json:"unsafe,omitempty"`
	sources  map[string][]byte
	adapters []string
}

func adaptersOf(vault string) ([]string, error) {
	inst, e := config.LoadInstance(vault)
	if e != nil {
		return nil, e
	}
	out := []string{}
	if list, ok := inst["adapters"].([]any); ok {
		for _, a := range list {
			if s, ok := a.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func readRegular(p string) ([]byte, bool, error) {
	st, e := os.Lstat(p)
	if os.IsNotExist(e) {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	if !st.Mode().IsRegular() {
		return nil, true, fmt.Errorf("%s is not a regular file", p)
	}
	b, e := os.ReadFile(p)
	return b, true, e
}

// Preview computes the update of a vault to the kernel this binary carries. A vault without a lock
// gets its first installation: every existing file with other content is a conflict.
func Preview(vault string) (Plan, error) {
	lock, e := ReadLock(vault)
	installed := e == nil
	if os.IsNotExist(e) {
		lock, e = Lock{KernelVersion: "none", Hashes: map[string]string{}}, nil
	}
	if e != nil {
		return Plan{}, e
	}
	adapters, e := adaptersOf(vault)
	if e != nil {
		return Plan{}, e
	}
	src, e := Sources(adapters)
	if e != nil {
		return Plan{}, e
	}
	p := Plan{From: lock.KernelVersion, To: Version(), Changes: []Change{}, Conflicts: []string{}, sources: src, adapters: adapters}
	if st, e := os.Lstat(filepath.Join(vault, ".claude", "skills")); e == nil && st.Mode()&os.ModeSymlink == 0 {
		p.Unsafe = append(p.Unsafe, ".claude/skills")
	}
	for _, rel := range sortedKeys(src) {
		if safePath(vault, rel) != nil {
			p.Unsafe = append(p.Unsafe, rel)
			continue
		}
		cur, exists, e := readRegular(filepath.Join(vault, filepath.FromSlash(rel)))
		if e != nil {
			p.Unsafe = append(p.Unsafe, rel)
			continue
		}
		prev, owned := lock.Hashes[rel]
		switch {
		case !exists:
			p.Changes = append(p.Changes, Change{rel, "add"})
		case bytes.Equal(cur, src[rel]):
			p.Unchanged++
		case !owned && installed:
			p.Foreign = append(p.Foreign, rel)
		case owned && digest(cur) != prev, !owned:
			p.Conflicts = append(p.Conflicts, rel)
			p.Changes = append(p.Changes, Change{rel, "change"})
		default:
			p.Changes = append(p.Changes, Change{rel, "change"})
		}
	}
	for _, rel := range sortedKeys(lock.Hashes) {
		if _, still := src[rel]; still {
			continue
		}
		if safePath(vault, rel) != nil {
			p.Unsafe = append(p.Unsafe, rel)
			continue
		}
		cur, exists, e := readRegular(filepath.Join(vault, filepath.FromSlash(rel)))
		switch {
		case e != nil:
			p.Unsafe = append(p.Unsafe, rel)
		case !exists:
		case digest(cur) != lock.Hashes[rel]:
			p.Conflicts = append(p.Conflicts, rel)
			p.Changes = append(p.Changes, Change{rel, "remove"})
		default:
			p.Changes = append(p.Changes, Change{rel, "remove"})
		}
	}
	return p, nil
}

// safePath refuses a managed path that escapes the vault or passes through a symbolic link.
func safePath(vault, rel string) error {
	if path.IsAbs(rel) || strings.Contains("/"+rel+"/", "/../") {
		return fmt.Errorf("unsafe managed path: %s", rel)
	}
	parent := vault
	parts := strings.Split(rel, "/")
	for _, part := range parts[:len(parts)-1] {
		parent = filepath.Join(parent, part)
		if st, e := os.Lstat(parent); e == nil && (st.Mode()&os.ModeSymlink != 0 || !st.IsDir()) {
			return fmt.Errorf("managed path has an unsafe parent: %s", rel)
		}
	}
	return nil
}

// Apply writes the plan: managed files, retired files removed, the local ignore rules and the lock.
// bases are the root Bases a cell starts with; Apply writes the missing ones and never changes the others.
var bases = []string{"Arquitectura.base", "Auditoria.base", "Operacion.base", "Repos.base"}

// Written lists every path Apply may write for p: the plan's files, the starting Bases, the Claude
// skills link, the ignore rules, Obsidian's app settings and the lock.
func Written(p Plan) []string {
	out := []string{LockName, ".gitignore", ".claude/skills", ".obsidian/app.json"}
	out = append(out, bases...)
	for _, c := range p.Changes {
		out = append(out, c.Path)
	}
	return out
}

// Apply writes each file atomically and the lock last: running an interrupted update again finishes it,
// because files that already match the kernel count as unchanged.
func Apply(vault string, p Plan) error {
	// Cell settings are parsed before anything is written, so invalid ones change nothing.
	settings, e := prepareIgnores(vault)
	if e != nil {
		return e
	}
	for _, c := range p.Changes {
		target := filepath.Join(vault, filepath.FromSlash(c.Path))
		if c.Action == "remove" {
			if e := os.Remove(target); e != nil && !os.IsNotExist(e) {
				return e
			}
			for dir := filepath.Dir(target); dir != vault && strings.HasPrefix(dir, vault); dir = filepath.Dir(dir) {
				if entries, e := os.ReadDir(dir); e != nil || len(entries) > 0 || os.Remove(dir) != nil {
					break
				}
			}
			continue
		}
		if e := writeFile(target, p.sources[c.Path]); e != nil {
			return e
		}
	}
	for _, base := range bases {
		if _, e := os.Lstat(filepath.Join(vault, base)); os.IsNotExist(e) {
			b, _ := fs.ReadFile(knowledgeos.Payload, "kernel/"+base)
			if e := writeFile(filepath.Join(vault, base), b); e != nil {
				return e
			}
		}
	}
	if e := claudeSkillsLink(vault); e != nil {
		return e
	}
	for _, f := range settings {
		if e := writeFile(f.path, f.contents); e != nil {
			return e
		}
	}
	hashes := map[string]string{}
	for rel := range p.sources {
		b, exists, e := readRegular(filepath.Join(vault, filepath.FromSlash(rel)))
		if e != nil {
			return e
		}
		if !exists {
			return fmt.Errorf("managed file missing after kernel update: %s", rel)
		}
		hashes[rel] = digest(b)
	}
	lock := Lock{KernelVersion: Version(), Revision: Revision, Dirty: Dirty != "false", Adapters: p.adapters, Hashes: hashes}
	return writeFile(filepath.Join(vault, LockName), []byte(lock.dump()))
}

// writeFile replaces target atomically and keeps an existing file's permissions. The temporary file
// has a fixed name, so running an interrupted update again reuses and removes it. There is no fsync:
// kernel files are reproducible, and running the update again restores them after a crash.
func writeFile(target string, b []byte) error {
	mode := os.FileMode(0o644)
	if st, e := os.Lstat(target); e == nil {
		if !st.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", target)
		}
		mode = st.Mode().Perm()
	} else if !os.IsNotExist(e) {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(target), 0o755); e != nil {
		return e
	}
	tmp := target + ".kos-tmp"
	if e := os.Remove(tmp); e != nil && !os.IsNotExist(e) {
		return e
	}
	f, e := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if e != nil {
		return e
	}
	defer os.Remove(tmp)
	defer f.Close()
	if e := f.Chmod(mode); e != nil { // OpenFile's mode passes through the umask
		return e
	}
	if _, e := f.Write(b); e != nil {
		return e
	}
	if e := f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, target)
}

// claudeSkillsLink points .claude/skills at the shared skills so Claude Code finds them.
func claudeSkillsLink(vault string) error {
	link := filepath.Join(vault, ".claude", "skills")
	if target, e := os.Readlink(link); e == nil && target == filepath.Join("..", ".agents", "skills") {
		return nil
	}
	if st, e := os.Lstat(link); e == nil {
		if st.Mode()&os.ModeSymlink == 0 {
			return errors.New(".claude/skills must be moved explicitly before the kernel links it")
		}
		if e := os.Remove(link); e != nil {
			return e
		}
	}
	if e := os.MkdirAll(filepath.Dir(link), 0o755); e != nil {
		return e
	}
	return os.Symlink(filepath.Join("..", ".agents", "skills"), link)
}

var ignored = []string{"/AGENTS.personal.md", "/.investigations/", "/.investigations-private/", "/.operations/",
	"/.knowledge-os-config.yaml", "/.knowledge-os-config.*.tmp", "/.agents/state/discovery/", "/.plan/", "/.scratch/", "/.venv/"}

type settingsFile struct {
	path     string
	contents []byte
}

// prepareIgnores computes the ignore rules that keep local stores out of Git and out of Obsidian's graph.
func prepareIgnores(vault string) ([]settingsFile, error) {
	writes := []settingsFile{}
	p, e := settingsPath(filepath.Join(vault, ".gitignore"))
	if e != nil {
		return nil, e
	}
	b, _, e := readRegular(p)
	if e != nil {
		return nil, e
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(b) == 0 {
		lines = nil
	}
	// The lock is portable and committed; a visible plan/ would be graph content.
	kept, changed := []string{}, false
	for _, l := range lines {
		switch strings.TrimSpace(l) {
		case ".knowledge-os.lock.yaml", "/.knowledge-os.lock.yaml", "plan/", "/plan/":
			changed = true
		default:
			kept = append(kept, l)
		}
	}
	lines = kept
	for _, want := range ignored {
		if !contains(lines, want) {
			lines, changed = append(lines, want), true
		}
	}
	if changed {
		writes = append(writes, settingsFile{p, []byte(strings.Join(lines, "\n") + "\n")})
	}
	app, e := settingsPath(filepath.Join(vault, ".obsidian", "app.json"))
	if e != nil {
		return nil, e
	}
	payload := map[string]any{}
	b, exists, e := readRegular(app)
	if e != nil {
		return nil, e
	}
	if exists {
		if e := json.Unmarshal(b, &payload); e != nil {
			return nil, fmt.Errorf("%s: %w", app, e)
		}
		if payload == nil {
			return nil, fmt.Errorf("%s: expected a JSON object", app)
		}
	}
	filters, dropped := []string{}, false
	if list, ok := payload["userIgnoreFilters"].([]any); ok {
		for _, f := range list {
			switch s, _ := f.(string); s {
			case "plan/", "/plan/", "investigations/", "/investigations/": // published cases stay visible
				dropped = true
			default:
				filters = append(filters, s)
			}
		}
	}
	before := len(filters)
	for _, want := range []string{".plan/", ".scratch/", ".investigations/", "AGENTS.personal.md"} {
		if !contains(filters, want) {
			filters = append(filters, want)
		}
	}
	if len(filters) == before && !dropped && exists {
		return writes, nil
	}
	payload["userIgnoreFilters"] = filters
	out, _ := json.MarshalIndent(payload, "", "  ")
	return append(writes, settingsFile{app, append(out, '\n')}), nil
}

// settingsPath follows links to a cell's settings (a shared .obsidian is common), so they are updated
// where they live instead of being replaced by a regular file.
func settingsPath(p string) (string, error) {
	if real, e := filepath.EvalSymlinks(p); e == nil {
		return real, nil
	} else if !os.IsNotExist(e) {
		return "", e
	}
	dir, e := filepath.EvalSymlinks(filepath.Dir(p))
	if os.IsNotExist(e) {
		return p, nil
	}
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, filepath.Base(p)), nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if strings.TrimSpace(x) == s {
			return true
		}
	}
	return false
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// diff renders the plan with git diff --no-index between the current and the new files.
func diff(vault string, p Plan, stat bool) (string, error) {
	tmp, e := os.MkdirTemp("", "kos-kernel-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(tmp)
	for _, c := range p.Changes {
		if b, exists, _ := readRegular(filepath.Join(vault, filepath.FromSlash(c.Path))); exists {
			if e := writeFile(filepath.Join(tmp, "a", filepath.FromSlash(c.Path)), b); e != nil {
				return "", e
			}
		}
		if c.Action != "remove" {
			if e := writeFile(filepath.Join(tmp, "b", filepath.FromSlash(c.Path)), p.sources[c.Path]); e != nil {
				return "", e
			}
		}
	}
	for _, d := range []string{"a", "b"} {
		_ = os.MkdirAll(filepath.Join(tmp, d), 0o755)
	}
	args := []string{"-c", "core.quotePath=false", "diff", "--no-index", "--no-color"}
	if stat {
		args = append(args, "--stat=200")
	}
	cmd := exec.Command("git", append(args, "a", "b")...)
	cmd.Dir = tmp
	out, e := cmd.Output()
	var exit *exec.ExitError
	if e != nil && !(errors.As(e, &exit) && exit.ExitCode() == 1) {
		return "", fmt.Errorf("git diff: %w", e)
	}
	return string(out), nil
}

const Help = `kernel COMMAND --vault PATH
  status                     the kernel this vault carries and the one kos carries
  update [--dry-run] [--diff] [--force]
                             bring the vault's kernel to the one kos carries. --dry-run lists what
                             changes (with a diffstat; --diff adds the full diff) and writes nothing.
                             A managed file changed in the vault since it was installed is a conflict:
                             nothing is written until it is kept in a cell-owned file or --force
                             restores the distribution version. A cell file the kernel never installed,
                             at a path it now ships, is never replaced, even with --force: rename it.
                             Each file is replaced atomically and the lock is written last: if an
                             update is interrupted or fails, fix the cause and run it again with the
                             same options to finish it.
                             Commit the result as one change.
  update --all [--dry-run] [--commit]
                             every vault this machine remembers (kos vaults) whose kernel is older:
                             current, newer and not-found vaults are listed and left alone; a refusal
                             never uses --force and does not stop the others. --commit commits only
                             what the update wrote, on the current branch, and refuses a vault with
                             pending tracked changes or on a sync/ branch.`

func Run(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, e := io.WriteString(out, Help+"\n")
		return e
	}
	fset := flag.NewFlagSet("kernel "+args[0], flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	vault := fset.String("vault", "", "vault root")
	dry := fset.Bool("dry-run", false, "")
	full := fset.Bool("diff", false, "")
	force := fset.Bool("force", false, "")
	if e := fset.Parse(args[1:]); e != nil {
		return e
	}
	if *vault == "" {
		return errors.New("--vault is required")
	}
	root, e := vaultRoot(*vault)
	if e != nil {
		return e
	}
	switch args[0] {
	case "status":
		res, e := Status(root)
		if e != nil {
			return e
		}
		return emit(out, res)
	case "update":
		res, e := Update(root, Options{DryRun: *dry, Diff: *full, Force: *force})
		if res != nil {
			_ = emit(out, res)
		}
		return e
	default:
		return fmt.Errorf("unknown kernel command %q", args[0])
	}
}

func summary(p Plan) map[string]any {
	return map[string]any{"vault_kernel": p.From, "kos_kernel": p.To, "changes": len(p.Changes), "conflicts": p.Conflicts, "unsafe": p.Unsafe, "foreign": p.Foreign}
}

// Status compares the vault's kernel with the one kos carries.
func Status(root string) (map[string]any, error) {
	p, e := Preview(root)
	if e != nil {
		return nil, e
	}
	res := summary(p)
	res["current"] = len(p.Changes) == 0 && p.From == p.To
	if !res["current"].(bool) {
		res["next"] = "kos kernel update --vault \"" + root + "\" --dry-run"
	}
	return res, nil
}

// Options select how Update runs.
type Options struct{ DryRun, Diff, Force bool }

// Update brings the vault's kernel to the one kos carries. It returns the result to report even when it
// refuses (status conflict, foreign or unsafe) together with the error.
func Update(root string, o Options) (map[string]any, error) {
	p, e := Preview(root)
	if e != nil {
		return nil, e
	}
	res := summary(p)
	res["files"] = p.Changes
	res["unchanged"] = p.Unchanged
	if o.DryRun {
		if d, e := diff(root, p, !o.Diff); e == nil {
			res["diff"] = d
		}
		res["next"] = "kos kernel update --vault \"" + root + "\""
		return res, nil
	}
	if len(p.Unsafe) > 0 {
		res["status"] = "unsafe"
		res["next"] = "these targets are not regular files or sit under a symbolic link: move them explicitly, then run again"
		return res, errors.New("unsafe managed targets; nothing written")
	}
	if len(p.Foreign) > 0 {
		res["status"] = "foreign"
		res["next"] = "these cell files sit where the kernel now ships its own: rename or move them (a cell skill takes another name), then run again"
		return res, errors.New("cell files at kernel paths; nothing written")
	}
	if len(p.Conflicts) > 0 && !o.Force {
		res["status"] = "conflict"
		res["next"] = "these managed files changed in the vault (git diff shows how): keep cell logic in cell-owned files, then run again, or pass --force to restore the distribution version"
		return res, errors.New("kernel files changed locally; nothing written")
	}
	if e := Apply(root, p); e != nil {
		return nil, e
	}
	res["status"] = "updated"
	res["written"] = Written(p)
	res["next"] = "review git diff and commit the kernel update as one change"
	return res, nil
}

func emit(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}

// vaultRoot accepts an installed vault, or a directory being installed that holds instance.yaml.
func vaultRoot(p string) (string, error) {
	if r, e := config.Resolve(p); e == nil {
		root, _ := r["vault_root"].(string)
		return root, nil
	}
	abs, e := filepath.Abs(p)
	if e != nil {
		return "", e
	}
	if _, e := os.Stat(filepath.Join(abs, "instance.yaml")); e != nil {
		return "", fmt.Errorf("%s is not a vault: instance.yaml is missing", p)
	}
	return abs, nil
}
