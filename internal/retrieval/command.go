package retrieval

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"documentation-vault/internal/config"
)

func Run(ctx context.Context, command string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	vault := fs.String("vault", ".", "vault root")
	cache := fs.String("cache", "", "local cache outside vault")
	query := fs.String("query", "", "literal search terms")
	node := fs.String("node", "", "canonical note basename")
	limit := fs.Int("limit", 5, "source limit 1..10")
	visibility := fs.String("visibility", "all", "all or public")
	rebuild := fs.Bool("rebuild", false, "rebuild index")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if command == "search" && *rebuild {
		return fmt.Errorf("--rebuild belongs to index")
	}
	if _, err := config.LoadInstance(*vault); err != nil {
		return fmt.Errorf("invalid vault configuration: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	idx, err := Open(ctx, Options{*vault, *cache, *rebuild})
	if err != nil {
		return err
	}
	defer idx.Close()
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	if command == "index" {
		return encoder.Encode(idx.Stats)
	}
	if command == "links" {
		result, err := idx.Neighbors(ctx, *node)
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	}
	result, err := idx.Search(ctx, *query, *limit, *visibility)
	if err != nil {
		return err
	}
	return encoder.Encode(result)
}
