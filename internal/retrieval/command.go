package retrieval

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"knowledge-os/internal/config"
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
	fs.String("folder", "", "overview: limit to one knowledge folder, e.g. 20-Repos")
	budget := fs.Int("budget", DefaultBudget, "ask, read, code: output characters")
	code := fs.Bool("code", true, "read: include the cited source lines")
	note := fs.String("note", "", "read: note basename, alias or path")
	section := fs.String("section", "", "read: heading text")
	lines := fs.String("lines", "", "read: FROM-TO")
	match := fs.String("match", "", "read: keep the paragraphs holding one of these terms")
	repo := fs.String("repo", "", "code: repository, its note's basename or alias, or all")
	grep := fs.String("grep", "", "code: extended regular expression to search at the reference branch")
	show := fs.String("show", "", "code: path[:FROM-TO] to show at the reference branch")
	fn := fs.String("func", "", "code: function or method to show with its callers")
	ignoreCase := fs.Bool("i", false, "code: case-insensitive --grep")
	tests := fs.Bool("tests", false, "code: include tests in --grep")
	path := fs.String("path", "", "code: glob limiting --grep and --func to some files")
	up := fs.Int("up", 0, "code: --func follows the callers this many levels up")
	down := fs.Bool("down", false, "code: --func lists the repository's functions it calls, each with its exits")
	brief := fs.Bool("brief", false, "read: paragraphs with their source marks only, no code or footnote text")
	if err := fs.Parse(args); err != nil {
		return err
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if command == "ask" && *query == "" && fs.NArg() > 0 {
		*query = strings.Join(fs.Args(), " ") // kos ask "the question" works too
	} else if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	folder := fs.Lookup("folder")
	if command == "overview" {
		if _, err := config.LoadInstance(*vault); err != nil {
			return fmt.Errorf("invalid vault configuration: %w", err)
		}
		return WriteOverview(*vault, folder.Value.String(), out)
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
	if command == "ask" {
		return idx.Ask(ctx, *query, AskOptions{Visibility: *visibility, Budget: *budget}, out)
	}
	if command == "code" {
		if !set["budget"] {
			*budget = ReadBudget
		}
		return idx.Code(ctx, CodeOptions{Repo: *repo, Grep: *grep, Show: *show, Func: *fn, Path: *path, Up: *up, Down: *down, IgnoreCase: *ignoreCase, Tests: *tests, Budget: *budget}, out)
	}
	if command == "read" {
		if *note == "" {
			return fmt.Errorf("--note is required")
		}
		if !set["budget"] {
			*budget = ReadBudget
		}
		if *lines != "" && !set["code"] {
			*code = false // a follow-up read of lines wants the text; --code brings the cited lines
		}
		return idx.Read(ctx, *note, ReadOptions{Section: *section, Lines: *lines, Match: *match, Budget: *budget, Code: *code && !*brief, Brief: *brief}, out)
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
