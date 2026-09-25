package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"documentation-vault/internal/audit"
	"documentation-vault/internal/cases"
	"documentation-vault/internal/check"
	"documentation-vault/internal/config"
	"documentation-vault/internal/discover"
	"documentation-vault/internal/gitsync"
	"documentation-vault/internal/handoff"
	"documentation-vault/internal/inventory"
	"documentation-vault/internal/investigation"
	"documentation-vault/internal/retrieval"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Println(`vaultctl — local vault operations

overview --vault PATH [--folder 20-Repos]   one line per knowledge note (Markdown)
search --vault PATH --query TEXT [--limit 1..10] [--visibility all|public]
index --vault PATH [--rebuild]
links --vault PATH --node BASENAME
inventory --vault PATH [--repo NAME] [--github-user LOGIN]
discover run|questions|answer|platform|report|check|claims|corrections --vault PATH ...
config status|resolve|workspace|locate|capability|bind|catalog|areas|operation ...
check links|bases --vault PATH
audit --vault PATH
check visual FILE [--kind diagram|spatial] [--temporal]
check visual-context FILE
check obsidian-binding --vault PATH --vault-name NAME
check map-closure --vault PATH --checkpoint FILE
investigation new|list|check --vault PATH ...   (legacy transactional verbs remain during migration)
handoff inspect|plan-worktree|create-worktree|plan-handoff|prepare-handoff|plan|apply|validate|resolve-branch|set-state ...
sync start|status|review|verify|acknowledge|finish|pull --vault PATH ...

Search refreshes a private local SQLite index before querying. --cache PATH
selects a cache outside the vault. Results are evidence pointers, not answers.
No hooks or model runtimes are used. Inventory and source acquisition can use
Git/GitHub; local search does not send vault content to a remote service.
version`)
		return nil
	}
	switch args[0] {
	case "version", "--version":
		fmt.Println(version)
		return nil
	case "inventory":
		return inventory.Run(args[1:], os.Stdout)
	case "discover":
		return discover.Run(args[1:], os.Stdout)
	case "config":
		return config.Run(args[1:], os.Stdout)
	case "audit":
		return audit.Run(args[1:], os.Stdout)
	case "check":
		return check.Run(args[1:], os.Stdout)
	case "investigation":
		// new/list/check with --vault are the case workflow; the remaining verbs are the legacy
		// transactional helper, kept until development handoffs stop depending on it.
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" || args[1] == "new" || args[1] == "check" || args[1] == "list" && !contains(args, "--root") {
			return cases.Run(args[1:], os.Stdout)
		}
		return investigation.Run(args[1:], os.Stdout)
	case "handoff":
		return handoff.Run(args[1:], os.Stdout)
	case "sync":
		return gitsync.Run(args[1:], os.Stdout)
	case "search", "index", "links", "overview":
		return retrieval.Run(ctx, args[0], args[1:], os.Stdout)
	default:
		return fmt.Errorf("unknown command %q; use --help", args[0])
	}
}

func contains(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}
