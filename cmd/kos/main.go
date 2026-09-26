package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"knowledge-os/internal/audit"
	"knowledge-os/internal/cases"
	"knowledge-os/internal/check"
	"knowledge-os/internal/config"
	"knowledge-os/internal/devhandoff"
	"knowledge-os/internal/discover"
	"knowledge-os/internal/gitsync"
	"knowledge-os/internal/inventory"
	"knowledge-os/internal/kernel"
	"knowledge-os/internal/release"
	"knowledge-os/internal/retrieval"
	"knowledge-os/internal/vaults"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
		os.Exit(1)
	}
}

const help = `kos — the knowledge OS of a team: evidence-first vault operations

overview --vault PATH [--folder 20-Repos]   one line per knowledge note (Markdown)
search --vault PATH --query TEXT [--limit 1..10] [--visibility all|public]
index --vault PATH [--rebuild]
links --vault PATH --node BASENAME
inventory --vault PATH [--repo NAME] [--github-user LOGIN]
discover run|questions|answer|platform|report|check|claims|corrections --vault PATH ...
config status|detect|resolve|workspace|locate|capability|bind|catalog|areas|operation ...
check links|bases --vault PATH
audit --vault PATH
check obsidian-binding --vault PATH --vault-name NAME
check map-closure --vault PATH --checkpoint FILE
investigation new|list|check|add|state|absorb|close|reopen --vault PATH ...
handoff start|status|refresh|reconcile --vault PATH ...
sync start|status|review|verify|acknowledge|finish|pull --vault PATH ...
kernel status|update [--dry-run] [--diff] [--force] --vault PATH
kernel update --all [--dry-run] [--commit]   every remembered vault with an older kernel
vaults [list] | add PATH | remove PATH | scan DIR [--depth N]   the vaults this machine knows
version [--vault PATH]     this kos, the kernel it carries, the vault's kernel, the latest release
update [--version X]       install the latest (or a given) kos release in place of this one, then
                           list the remembered vaults and the ones its kernel would update

--vault defaults to KOS_VAULT, else the vault that contains the current directory. Notices (a newer
kos, a vault kernel older or newer than this kos) go to stderr, one line each; stdout stays JSON.
Search keeps a private local SQLite index outside the vault; results are pointers, not answers.`

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" || args[0] == "-h" {
		_, e := fmt.Fprintln(stdout, help)
		return e
	}
	switch args[0] {
	case "version", "--version":
		return showVersion(ctx, args[1:], stdout)
	case "update":
		fs := flag.NewFlagSet("update", flag.ContinueOnError)
		to := fs.String("version", "", "")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		self, e := os.Executable()
		if e != nil {
			return e
		}
		res, e := release.Update(ctx, version, *to, self)
		if e != nil {
			return e
		}
		listAfterUpdate(res, self)
		return emit(stdout, res)
	case "vaults":
		return vaults.Run(args[1:], stdout)
	}
	if all(args) {
		res, e := vaults.UpdateAll(kernel.Options{DryRun: has(args, "--dry-run")}, has(args, "--commit"))
		if res != nil {
			_ = emit(stdout, res)
		}
		return e
	}
	args = withVault(args)
	if e := notices(args, stderr); e != nil {
		return e
	}
	if root, vk := vaultKernel(args); vk != "" {
		vaults.Register(root)
	}
	switch args[0] {
	case "inventory":
		return inventory.Run(args[1:], stdout)
	case "discover":
		return discover.Run(args[1:], stdout)
	case "config":
		return config.Run(args[1:], stdout)
	case "audit":
		return audit.Run(args[1:], stdout)
	case "check":
		return check.Run(args[1:], stdout)
	case "investigation":
		return cases.Run(args[1:], stdout)
	case "handoff":
		if len(args) > 1 && args[1] == "reconcile" {
			// Reconciliation writes the development case, so the case package owns it.
			return cases.Run(args[1:], stdout)
		}
		return devhandoff.Run(args[1:], stdout)
	case "sync":
		return gitsync.Run(args[1:], stdout)
	case "kernel":
		return kernel.Run(args[1:], stdout)
	case "search", "index", "links", "overview":
		return retrieval.Run(ctx, args[0], args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q; use --help", args[0])
	}
}

func has(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// all is `kernel update --all`: every remembered vault instead of one.
func all(args []string) bool {
	return len(args) > 1 && args[0] == "kernel" && args[1] == "update" && has(args, "--all")
}

// listAfterUpdate adds the remembered vaults, checked by the kos now installed (its kernel is the one
// that counts), and the step they call for.
func listAfterUpdate(res map[string]any, self string) {
	bin := self
	if p, ok := res["path"].(string); ok && res["status"] == "updated" {
		bin = p
	}
	cmd := exec.Command(bin, "vaults")
	cmd.Env = append(os.Environ(), "KOS_NO_UPDATE_CHECK=1")
	out, e := cmd.Output()
	var list map[string]any
	if e != nil || json.Unmarshal(out, &list) != nil {
		res["next"] = "run `kos vaults` to see which vaults carry an older kernel"
		return
	}
	res["vaults"] = list["vaults"]
	if next, ok := list["next"].(string); ok {
		res["next"] = next
	} else {
		delete(res, "next")
	}
}

// vaultAt is where a command takes --vault: after the command, or after its verb.
func vaultAt(args []string) int {
	switch args[0] {
	case "overview", "search", "index", "links", "inventory", "audit":
		return 1
	case "discover", "config", "investigation", "handoff", "sync", "kernel":
		if len(args) > 1 {
			return 2
		}
	case "check":
		if len(args) > 1 && (args[1] == "links" || args[1] == "bases" || args[1] == "obsidian-binding" || args[1] == "map-closure") {
			return 2
		}
	}
	return -1
}

func hasVault(args []string) (string, bool) {
	for i, a := range args {
		if a == "--vault" && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// withVault adds --vault when the command takes one and the caller gave none.
func withVault(args []string) []string {
	at := vaultAt(args)
	if _, ok := hasVault(args); ok || at < 0 || at > len(args) {
		return args
	}
	root := os.Getenv("KOS_VAULT")
	if root == "" {
		root = enclosingVault()
	}
	if root == "" {
		return args
	}
	out := append([]string{}, args[:at]...)
	out = append(out, "--vault", root)
	return append(out, args[at:]...)
}

// enclosingVault is the nearest directory above the working directory that holds a kernel lock.
func enclosingVault() string {
	dir, e := os.Getwd()
	if e != nil {
		return ""
	}
	for {
		if _, e := os.Stat(filepath.Join(dir, kernel.LockName)); e == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func vaultKernel(args []string) (string, string) {
	root, ok := hasVault(args)
	if !ok {
		return "", ""
	}
	if r, e := config.Resolve(root); e == nil {
		root, _ = r["vault_root"].(string)
	}
	l, e := kernel.ReadLock(root)
	if e != nil {
		return root, ""
	}
	return root, l.KernelVersion
}

// gated commands publish or judge publication; they run only with a kos at least as new as the vault.
func gated(args []string) bool {
	if args[0] == "sync" && len(args) > 1 {
		switch args[1] {
		case "review", "verify", "finish":
			return true
		}
	}
	return args[0] == "discover" && len(args) > 1 && args[1] == "check"
}

func notices(args []string, stderr io.Writer) error {
	if latest := release.Newer(version); latest != "" {
		fmt.Fprintf(stderr, "kos notice: kos %s is available (this is %s); tell the user and, with their approval, run `kos update`: it lists the vaults whose kernel it would update\n", latest, version)
	}
	root, vk := vaultKernel(args)
	if vk == "" || args[0] == "kernel" {
		return nil
	}
	switch c := release.Compare(vk, kernel.Version()); {
	case c < 0:
		fmt.Fprintf(stderr, "kos notice: this vault's kernel is %s and kos carries %s; tell the user and offer `kos kernel update --vault %q --dry-run`\n", vk, kernel.Version(), root)
	case c > 0:
		msg := fmt.Sprintf("this vault's kernel %s is newer than kos %s; run `kos update`", vk, kernel.Version())
		if gated(args) {
			return errors.New(msg + " before publishing: gates must run with the kernel's own rules")
		}
		fmt.Fprintf(stderr, "kos notice: %s\n", msg)
	}
	return nil
}

func showVersion(ctx context.Context, args []string, stdout io.Writer) error {
	args = withVault(append([]string{"kernel", "status"}, args...))
	res := map[string]any{"kos": version, "kernel": kernel.Version()}
	if root, vk := vaultKernel(args); vk != "" {
		res["vault"], res["vault_kernel"] = root, vk
	}
	if v := release.Newer(version); v != "" {
		res["latest"] = v
		res["next"] = "kos update"
	}
	return emit(stdout, res)
}

func emit(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}
