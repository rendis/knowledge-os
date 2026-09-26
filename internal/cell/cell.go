// Package cell creates, adopts and checks a cell vault: its identity (instance.yaml, Home, system
// notes) and the first installation of the kernel the binary carries.
package cell

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"knowledge-os/internal/config"
	"knowledge-os/internal/kernel"
	"knowledge-os/internal/vaults"
)

const Help = `init --vault PATH [answers]   create a cell vault; unanswered questions are asked on stderr
  --cell-name NAME --purpose TEXT --system Name|id:Name (repeat) --github-org ORG
  --repo-prefix PREFIX (repeat) --reference-branch BRANCH (repeat, in preference order)
  --platform gcp|aws|azure (repeat) --tracker URL|id:provider:URL (repeat) --locale es|en
  --adapter NAME (repeat) --evidence-profile production-gate|documented-source|mixed
  --vault-remote URL --disable-topics --yes (defaults, no questions) --force (re-initialize)
adopt --vault PATH [--force]   install the kernel into existing notes with a valid instance.yaml
doctor --vault PATH [--strict] read-only health of a vault; --strict fails on drift or an
                               unreproducible kernel

--vault defaults to the current directory. All output is JSON.`

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// Run executes init, adopt or doctor. in is read only by init's questions.
func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("init, adopt or doctor is required")
	}
	if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
		_, e := io.WriteString(out, Help+"\n")
		return e
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o Options
	var systems, trackers, adapters, platforms, prefixes, branches multi
	vault := fs.String("vault", "", "")
	strict := fs.Bool("strict", false, "")
	fs.StringVar(&o.CellName, "cell-name", "", "")
	fs.StringVar(&o.Purpose, "purpose", "", "")
	fs.StringVar(&o.GithubOrg, "github-org", "", "")
	fs.StringVar(&o.Locale, "locale", "", "")
	fs.StringVar(&o.EvidenceProfile, "evidence-profile", "", "")
	fs.StringVar(&o.VaultRemote, "vault-remote", "", "")
	fs.Var(&systems, "system", "")
	fs.Var(&trackers, "tracker", "")
	fs.Var(&adapters, "adapter", "")
	fs.Var(&platforms, "platform", "")
	fs.Var(&prefixes, "repo-prefix", "")
	fs.Var(&branches, "reference-branch", "")
	fs.BoolVar(&o.DisableTopics, "disable-topics", false, "")
	fs.BoolVar(&o.Yes, "yes", false, "")
	fs.BoolVar(&o.Force, "force", false, "")
	if e := fs.Parse(args[1:]); e != nil {
		return e
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	o.Systems, o.Trackers, o.Adapters, o.Platforms, o.RepoPrefixes, o.ReferenceBranches = systems, trackers, adapters, platforms, prefixes, branches
	root := *vault
	if root == "" {
		root = "."
	}
	root, e := filepath.Abs(root)
	if e != nil {
		return e
	}
	var res map[string]any
	switch args[0] {
	case "init":
		res, e = Init(ctx, root, o, in, errOut)
	case "adopt":
		res, e = Adopt(root, o.Force)
	case "doctor":
		res, e = Doctor(root, *strict)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	if res != nil {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(res)
	}
	return e
}

// Init asks what the flags did not answer, then writes the cell's identity and installs the kernel.
// Nothing is written until every answer is in and valid.
func Init(ctx context.Context, root string, o Options, in io.Reader, questions io.Writer) (map[string]any, error) {
	switch state(root) {
	case "installed":
		if !o.Force {
			return nil, fmt.Errorf("already installed: %s (use kos kernel update, or --force to re-initialize)", root)
		}
	case "knowledge-without-lock":
		if !o.Force {
			return nil, fmt.Errorf("%s has knowledge files but no kernel lock: use kos adopt, or --force", root)
		}
	}
	inst, e := buildInstance(o, newAsker(ctx, in, questions, o.Yes))
	if e != nil {
		return nil, e
	}
	text := dumpInstance(inst)
	if e := validate(text); e != nil {
		return map[string]any{"status": "invalid-instance", "error": e.Error()}, e
	}
	if e := os.MkdirAll(root, 0o755); e != nil {
		return nil, e
	}
	if e := bootstrap(root, text, inst); e != nil {
		return nil, e
	}
	res, e := kernel.Update(root, kernel.Options{Force: o.Force})
	if e != nil {
		return ownershipConflict(res, e)
	}
	vaults.Register(root)
	return map[string]any{"status": "initialized", "vault": root, "cell": inst["cell"],
		"next": "Commit the vault (git init, add and commit), open it with your agent and ask it to finish onboarding " +
			"(onboard-cell): it lists the repositories, runs the first discovery and reports what the vault can answer. " +
			"Each teammate is onboarded the first time they open it (onboard-developer)."}, nil
}

func ownershipConflict(res map[string]any, e error) (map[string]any, error) {
	if res == nil {
		return nil, e
	}
	files := []any{}
	for _, key := range []string{"conflicts", "foreign", "unsafe"} {
		if list, ok := res[key].([]string); ok {
			for _, f := range list {
				files = append(files, f)
			}
		}
	}
	return map[string]any{"status": "ownership-conflict", "files": files, "next": res["next"]}, e
}

// Adopt installs the kernel into an existing knowledge vault; its notes and Home are never rewritten.
func Adopt(root string, force bool) (map[string]any, error) {
	switch s := state(root); s {
	case "installed":
		return nil, fmt.Errorf("already installed: %s (use kos kernel update)", root)
	case "knowledge-without-lock":
	default:
		return nil, fmt.Errorf("adopt requires an existing knowledge vault without a kernel lock (state=%s)", s)
	}
	if _, e := os.Stat(filepath.Join(root, "00-Home.md")); e != nil {
		return nil, errors.New("00-Home.md is required for adopt")
	}
	if notes, _ := filepath.Glob(filepath.Join(root, "10-Sistemas", "*.md")); len(notes) == 0 {
		return nil, errors.New("10-Sistemas/*.md is required for adopt")
	}
	if _, e := os.Stat(filepath.Join(root, "instance.yaml")); e != nil {
		return nil, errors.New("write instance.yaml (cell identity) before adopt; Home is never rewritten")
	}
	inst, e := config.LoadInstance(root)
	if e != nil {
		return nil, fmt.Errorf("instance.yaml: %w", e)
	}
	res, e := kernel.Update(root, kernel.Options{Force: force})
	if e != nil {
		return ownershipConflict(res, e)
	}
	vaults.Register(root)
	return map[string]any{"status": "adopted", "vault": root, "cell": inst["cell"]}, nil
}

// Doctor reports a vault's health without writing: identity, catalog, kernel drift, adapters, the
// Claude skills link, the personal instructions file and orientation.
func Doctor(root string, strict bool) (map[string]any, error) {
	st := state(root)
	res := map[string]any{"vault": root, "state": st}
	tracked := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", "--", personal).Run() == nil
	_, exists := os.Stat(filepath.Join(root, personal))
	ignored := personalIgnored(root)
	res["personal_instructions"] = map[string]any{"path": personal, "exists": exists == nil, "ignored": ignored, "tracked": tracked}
	problems := []string{}
	if catalog, e := config.Catalog(root); e != nil {
		res["vault_catalog"] = map[string]any{"status": "invalid", "error": e.Error()}
		problems = append(problems, "invalid vault catalog")
	} else {
		status := "absent"
		if _, e := os.Stat(filepath.Join(root, "90-Meta", "vault-catalog.yaml")); e == nil {
			status = "valid"
		}
		res["vault_catalog"] = map[string]any{"status": status, "count": len(items(catalog["vaults"]))}
	}
	if st != "installed" {
		if st != "missing" && st != "empty" {
			problems = append(problems, "not an installed vault (state "+st+")")
		}
		return res, failure(problems, strict && st != "installed")
	}
	inst, e := config.LoadInstance(root)
	if e != nil {
		res["instance"] = map[string]any{"status": "invalid", "error": e.Error()}
		problems = append(problems, "invalid instance.yaml")
	} else {
		res["instance"] = map[string]any{"status": "valid"}
	}
	lock, e := kernel.ReadLock(root)
	if e != nil {
		res["lock"] = map[string]any{"status": "invalid", "error": e.Error()}
		return res, errors.New("unreadable kernel lock")
	}
	configured := []string{}
	for _, a := range items(inst["adapters"]) {
		configured = append(configured, fmt.Sprint(a))
	}
	adapterDrift := []string{}
	for _, a := range append(append([]string{}, lock.Adapters...), configured...) {
		if slices.Contains(lock.Adapters, a) != slices.Contains(configured, a) && !slices.Contains(adapterDrift, a) {
			adapterDrift = append(adapterDrift, a)
		}
	}
	slices.Sort(adapterDrift)
	res["adapters_installed"], res["adapters_configured"], res["adapter_configuration_drift"] = nonNil(lock.Adapters), configured, adapterDrift
	res["lock_version"], res["portable_lock"] = lock.Version, lock.Version == "5"
	res["kernel_version_installed"], res["kernel_version_kos"] = lock.KernelVersion, kernel.Version()
	res["kernel_revision"] = lock.Revision
	reproducible := lock.Revision != "" && lock.Revision != "unknown" && lock.Revision != "unversioned" && !lock.Dirty
	res["reproducible_kernel"] = reproducible
	drift, current := []string{}, false
	if inst != nil {
		if p, e := kernel.Preview(root); e == nil {
			drift = p.Conflicts
			current = len(p.Changes) == 0 && p.From == p.To
		}
	}
	res["drift"], res["kernel_current"] = drift, current
	topology := []string{}
	if target, e := os.Readlink(filepath.Join(root, ".claude", "skills")); e != nil || target != filepath.Join("..", ".agents", "skills") {
		topology = append(topology, ".claude/skills")
	}
	res["topology_drift"] = topology
	if inst != nil {
		orientation := config.Orientation(root, inst)
		res["orientation"], res["start_here"] = orientation, orientation["start_here"]
	}
	if strict {
		for _, c := range []struct {
			failed bool
			msg    string
		}{
			{lock.Version != "5", "lock is not portable"}, {!reproducible, "kernel was installed from an unreproducible build"},
			{len(drift) > 0, "kernel files changed locally"}, {len(topology) > 0, ".claude/skills is not linked to .agents/skills"},
			{len(adapterDrift) > 0, "adapters differ from instance.yaml"}, {!current, "kernel differs from the one kos carries"},
			{!ignored, personal + " is not ignored"}, {tracked, personal + " is tracked"},
		} {
			if c.failed {
				problems = append(problems, c.msg)
			}
		}
	}
	return res, failure(problems, false)
}

// failure turns the problems found into one error; notInstalled adds the strict failure of a vault
// without a kernel.
func failure(problems []string, notInstalled bool) error {
	if notInstalled {
		problems = append(problems, "no kernel installed")
	}
	if len(problems) == 0 {
		return nil
	}
	slices.Sort(problems)
	return errors.New("doctor: " + strings.Join(problems, "; "))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// personalIgnored reports whether Git, or the portable root rule, ignores the personal instructions.
func personalIgnored(root string) bool {
	cmd := exec.Command("git", "-C", root, "check-ignore", "--no-index", "--quiet", "--", personal)
	if e := cmd.Run(); e == nil {
		return true
	} else if exit, ok := e.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false
	}
	b, e := os.ReadFile(filepath.Join(root, ".gitignore"))
	if e != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if l := strings.TrimSpace(line); l == "/"+personal || l == personal {
			return true
		}
	}
	return false
}
