package handoff

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CompletePlan struct {
	Status          string            `json:"status"`
	Operation       string            `json:"operation"`
	Authorization   string            `json:"authorization"`
	Source          map[string]string `json:"source"`
	Base            map[string]any    `json:"base"`
	Worktree        map[string]string `json:"worktree"`
	Handoff         map[string]any    `json:"handoff"`
	Effects         map[string]any    `json:"effects"`
	Excluded        []string          `json:"excluded_effects"`
	FailureBoundary string            `json:"failure_boundary"`
	Token           string            `json:"plan_token"`
	worktree        WorktreePlan
	family          FamilyPlan
}

func gitRaw(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s failed: %w", args[0], err)
	}
	return raw, nil
}

func PlanComplete(o WorktreeOptions) (CompletePlan, error) {
	p := CompletePlan{}
	w, err := PlanWorktree(o)
	if err != nil {
		return p, err
	}
	b, err := loadBundle(o.Bundle)
	if err != nil {
		return p, err
	}
	temp, err := os.MkdirTemp("", "vaultctl-handoff-plan-")
	if err != nil {
		return p, err
	}
	defer os.RemoveAll(temp)
	source := w.Source["path"]
	selected := w.Base["selected_commit"].(string)
	projectionSource := source
	if w.Base["fetch_required"] == true {
		projectionSource = filepath.Join(temp, "objects.git")
		if _, err = gitCall(temp, "init", "--bare", projectionSource); err != nil {
			return p, err
		}
		remoteURL, e := gitCall(source, "remote", "get-url", w.Source["remote_name"])
		if e != nil {
			return p, e
		}
		if _, err = gitCall(projectionSource, "fetch", "--no-tags", "--no-write-fetch-head", "--depth=1", remoteURL, selected); err != nil {
			return p, errors.New("exact remote base could not be inspected in temporary repository")
		}
	}
	tracked, err := gitCall(projectionSource, "ls-tree", selected, "--", storeName)
	if err != nil {
		return p, err
	}
	if tracked != "" {
		return p, errors.New("fresh handoff cannot start from tracked handoff store")
	}
	projection := filepath.Join(temp, "projection")
	if _, err = gitCall(temp, "clone", "--shared", "--no-checkout", projectionSource, projection); err != nil {
		return p, err
	}
	if _, err = gitCall(projection, "symbolic-ref", "HEAD", "refs/heads/"+w.Worktree["branch"]); err != nil {
		return p, err
	}
	if _, err = gitCall(projection, "update-ref", "HEAD", selected); err != nil {
		return p, err
	}
	if _, err = gitCall(projection, "read-tree", selected); err != nil {
		return p, err
	}
	names := []string{}
	for _, name := range []string{"AGENTS.md", "AGENTS.override.md", ".gitignore"} {
		entry, e := gitCall(projectionSource, "ls-tree", selected, "--", name)
		if e != nil {
			return p, e
		}
		if entry == "" {
			continue
		}
		fields := strings.Fields(entry)
		if len(fields) < 4 || fields[1] != "blob" {
			return p, errors.New("root policy must be regular blob or counterpart symlink")
		}
		names = append(names, name)
	}
	if err = projectCheckout(source, projectionSource, selected, projection, names); err != nil {
		return p, err
	}
	f, err := planFamilyResolved(FamilyOptions{Vault: o.Vault, Bundle: o.Bundle, Worktree: w.Worktree["path"]}, b, projection, w.Worktree["path"], w.Source["remote"], w.Worktree["branch"])
	if err != nil {
		return p, err
	}
	payload, _ := json.Marshal(map[string]any{"schema": 1, "operation": "prepare-handoff", "worktree_plan_token": w.Token, "family_plan_token": f.Token})
	p = CompletePlan{Status: "planned", Operation: "prepare-handoff", Authorization: "single-complete-plan", Source: w.Source, Base: w.Base, Worktree: w.Worktree, Handoff: f.Handoff, Effects: map[string]any{"git": w.Effects, "files": f.Effects}, Excluded: []string{"source-code", "commit", "push", "pull-request"}, FailureBoundary: "Sequential execution retains the created worktree if materialization fails; resume using its exact path.", Token: digest(payload), worktree: w, family: f}
	return p, nil
}

// Use a disposable index with the source repository's checkout configuration.
// This applies built-in checkout conversions while preserving the source index
// and worktree. External policy filters require an explicit worktree phase.
func projectCheckout(source, objectsSource, selected, projection string, names []string) error {
	index := filepath.Join(filepath.Dir(projection), "projection-index")
	objects, err := gitCall(objectsSource, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return err
	}
	run := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		all := []string{"--no-optional-locks", "-C", source, "--work-tree=" + projection}
		all = append(all, args...)
		cmd := exec.CommandContext(ctx, "git", all...)
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "GIT_INDEX_FILE=") && !strings.HasPrefix(v, "GIT_ALTERNATE_OBJECT_DIRECTORIES=") {
				env = append(env, v)
			}
		}
		alt := objects
		if existing := os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES"); existing != "" {
			alt += string(os.PathListSeparator) + existing
		}
		cmd.Env = append(env, "GIT_INDEX_FILE="+index, "GIT_ALTERNATE_OBJECT_DIRECTORIES="+alt, "GIT_TERMINAL_PROMPT=0")
		data, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("temporary policy checkout failed: %w", err)
		}
		return data, nil
	}
	if _, err = run("read-tree", selected); err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	attributes, err := run(append([]string{"check-attr", "--cached", "-z", "filter", "--"}, names...)...)
	if err != nil {
		return err
	}
	parts := strings.Split(strings.TrimSuffix(string(attributes), "\x00"), "\x00")
	if len(parts)%3 != 0 {
		return errors.New("invalid policy checkout attributes")
	}
	for i := 2; i < len(parts); i += 3 {
		if parts[i] != "unspecified" && parts[i] != "unset" {
			return errors.New("policy uses an external checkout filter; prepare the worktree explicitly before handoff planning")
		}
	}
	args := []string{"checkout-index", "--force", "--prefix=" + projection + string(filepath.Separator), "--"}
	_, err = run(append(args, names...)...)
	return err
}

func PrepareComplete(o WorktreeOptions, approved string) (map[string]any, error) {
	p, err := PlanComplete(o)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(p.Token), []byte(approved)) != 1 {
		return nil, errors.New("plan_stale: complete handoff plan changed")
	}
	w, err := CreateWorktree(o, p.worktree.Token)
	if err != nil {
		return nil, err
	}
	options := FamilyOptions{Vault: o.Vault, Bundle: o.Bundle, Worktree: w.Worktree["path"]}
	f, err := PlanFamily(options)
	if err != nil {
		return nil, fmt.Errorf("worktree-created at %s; materialization planning failed: %w", options.Worktree, err)
	}
	if f.Token != p.family.Token {
		return nil, fmt.Errorf("worktree-created at %s; observed materialization differs from approved plan", options.Worktree)
	}
	f, err = ApplyFamily(options, f.Token)
	if err != nil {
		return nil, fmt.Errorf("worktree-created at %s; materialization failed: %w", options.Worktree, err)
	}
	validation, err := ValidateFamilyRepository(o.Vault, p.Source["remote"], options.Worktree)
	if err != nil {
		return nil, fmt.Errorf("handoff-applied at %s; validation failed: %w", options.Worktree, err)
	}
	return map[string]any{"status": "prepared", "operation": "prepare-handoff", "source": w.Source, "base": w.Base, "worktree": w.Worktree, "handoff": f.Handoff, "validation": validation}, nil
}
