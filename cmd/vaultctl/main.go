package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"documentation-vault/internal/audit"
	"documentation-vault/internal/check"
	"documentation-vault/internal/config"
	"documentation-vault/internal/discover"
	"documentation-vault/internal/handoff"
	"documentation-vault/internal/inventory"
	"documentation-vault/internal/investigation"
	"documentation-vault/internal/retrieval"
	"documentation-vault/internal/syncflow"
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
discover run|questions|answer|platform|report --vault PATH ...
config status|resolve|workspace|locate|capability|bind|catalog|areas|operation ...
check links|bases --vault PATH
audit --vault PATH
check visual FILE [--kind diagram|spatial] [--temporal]
check visual-context FILE
check obsidian-binding --vault PATH --vault-name NAME
check map-closure --vault PATH --checkpoint FILE
investigation open|load|list|snapshot|save|save-resources|publish|transition|close|bind|retire|consolidate|validate ...
handoff inspect|plan-worktree|create-worktree|plan-handoff|prepare-handoff|plan|apply|validate|resolve-branch|set-state ...
sync scan --vault PATH --repo NOTE [--source-repo PATH] [--query TEXT] [--max-hits N]
sync build|build-new|init-analysis|close-package|gate-batch ...
sync analysis finalize-analysis|check ...
sync review freeze|check|publish|verify-published ...
sync review-finalize ...
sync correction prepare|check ...
sync begin|checkpoint-package|seal-gate|status|validate-unit|review-unit|apply-unit|resume|close|abandon-empty ...

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
		return investigation.Run(args[1:], os.Stdout)
	case "handoff":
		return handoff.Run(args[1:], os.Stdout)
	case "sync":
		return syncflow.Run(args[1:], os.Stdout)
	case "search", "index", "links", "overview":
		return retrieval.Run(ctx, args[0], args[1:], os.Stdout)
	default:
		return fmt.Errorf("unknown command %q; use --help", args[0])
	}
}
