package config

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Detect proposes this developer's workspace from what the machine already has, read-only: where the
// cell's repositories are cloned, where worktrees fit, which cloud CLIs are installed and logged in,
// and which database proxy ports are missing. Nothing is written; the proposal ends in the exact
// workspace command to confirm.
func Detect(root string) (Object, error) {
	inst, e := LoadInstance(root)
	if e != nil {
		return nil, e
	}
	w, e := Workspace(root)
	if e != nil {
		return nil, e
	}
	prefixes := []string{}
	for _, p := range list(obj(inst["sources"])["repo_prefixes"]) {
		if s := strings.TrimRight(str(p), "-"); s != "" {
			prefixes = append(prefixes, s+"-")
		}
	}
	configured := []string{}
	for _, r := range list(obj(w["source_context"])["roots"]) {
		configured = append(configured, str(obj(r)["path"]))
	}
	candidates := repositoryRoots(root, prefixes)
	repos := Object{"prefixes": prefixes, "configured_roots": configured, "candidates": candidates}

	worktree := Object{"configured": w["worktree_root"]}
	chosen := configured
	if len(chosen) == 0 && len(candidates) > 0 {
		chosen = []string{str(candidates[0]["root"])}
	}
	if w["worktree_root"] == nil && len(chosen) > 0 {
		c := filepath.Join(filepath.Dir(chosen[0]), "worktrees")
		_, err := os.Stat(c)
		worktree["candidate"], worktree["exists"] = c, err == nil
	}

	clouds := []Object{}
	for _, p := range PlatformProviders(inst) {
		clouds = append(clouds, cloudLogin(p))
	}
	tools := Object{}
	for _, t := range []string{"git", "gh", "kubectl", "psql"} {
		_, err := lookPath(t)
		tools[t] = err == nil
	}
	if tools["gh"] == true {
		_, err := detectRun(10*time.Second, "gh", "auth", "status")
		tools["gh_logged_in"] = err == nil
	}

	ports := obj(w["proxy_ports"])
	dbs := []Object{}
	for _, t := range list(inst["database_targets"]) {
		key := str(obj(t)["port_key"])
		if key == "" {
			continue
		}
		p, ok := ports[key]
		dbs = append(dbs, Object{"target": str(obj(t)["id"]), "port_key": key, "port": p, "configured": ok})
	}

	next := []string{}
	missing := []string{}
	if w["status"] != "initialized" || len(configured) == 0 {
		if len(chosen) == 0 {
			missing = append(missing, "no clone of the cell's repositories found: ask where they are, or where to clone them (--managed-clone-root)")
		} else {
			cmd := "config --vault \"" + root + "\" workspace-init --repository-root \"" + chosen[0] + "\""
			if c := str(worktree["candidate"]); c != "" {
				cmd += " --development-worktree-root \"" + c + "\""
			}
			next = append(next, cmd)
		}
	}
	for _, c := range clouds {
		if c["status"] != "logged-in" {
			missing = append(missing, str(c["provider"])+": "+str(c["fix"]))
		}
	}
	for _, d := range dbs {
		if d["configured"] != true {
			missing = append(missing, "database target "+str(d["target"])+": its local proxy port ("+str(d["port_key"])+"=PORT, workspace-update --proxy-port)")
		}
	}
	return Object{"workspace": w["status"], "repositories": repos, "worktree_root": worktree, "clouds": clouds, "tools": tools,
		"databases": dbs, "propose": next, "missing": missing}, nil
}

// detectRun runs a local tool with a timeout; tests replace it.
var detectRun = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

var lookPath = exec.LookPath

var cloudCLI = map[string]string{"gcp": "gcloud", "aws": "aws", "azure": "az"}

// cloudLogin reports whether the cloud's CLI is installed and which identity it is logged in as.
func cloudLogin(provider string) Object {
	cli := cloudCLI[provider]
	o := Object{"provider": provider, "cli": cli}
	if _, e := lookPath(cli); e != nil {
		o["status"], o["fix"] = "missing", "install "+cli
		return o
	}
	switch provider {
	case "gcp":
		b, e := detectRun(20*time.Second, "gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
		if a := strings.TrimSpace(string(b)); e == nil && a != "" {
			o["status"], o["account"] = "logged-in", a
		} else {
			o["status"], o["fix"] = "not-logged-in", "gcloud auth login"
		}
	case "aws":
		b, e := detectRun(20*time.Second, "aws", "sts", "get-caller-identity", "--output", "json")
		var id struct{ Account, Arn string }
		if e == nil && json.Unmarshal(b, &id) == nil && id.Account != "" {
			o["status"], o["account"] = "logged-in", id.Account
		} else {
			o["status"], o["fix"] = "not-logged-in", "aws sso login --profile <profile of the cell's account> (then AWS_PROFILE=<profile>)"
		}
	case "azure":
		b, e := detectRun(20*time.Second, "az", "account", "show", "--output", "json")
		var acc struct {
			Name string
			User struct{ Name string }
		}
		if e == nil && json.Unmarshal(b, &acc) == nil && acc.User.Name != "" {
			o["status"], o["account"], o["subscription"] = "logged-in", acc.User.Name, acc.Name
		} else {
			o["status"], o["fix"] = "not-logged-in", "az login"
		}
	}
	return o
}

var skipDir = map[string]bool{"node_modules": true, "vendor": true, "Library": true, "Applications": true, "Pictures": true, "Music": true, "Movies": true}

// repositoryRoots finds directories whose immediate children are Git clones of the cell's
// repositories: near the vault first, then the usual places under the home directory.
func repositoryRoots(vault string, prefixes []string) []Object {
	home, _ := os.UserHomeDir()
	bases := []string{filepath.Dir(vault), filepath.Dir(filepath.Dir(vault))}
	for _, d := range []string{"Projects", "projects", "code", "src", "dev", "Developer", "workspace", "git", "repos", "work"} {
		if home != "" {
			bases = append(bases, filepath.Join(home, d))
		}
	}
	type cand struct {
		root     string
		matching []string
	}
	found := map[string]*cand{}
	// Visited directories, case-folded: the usual base names overlap and macOS and Windows fold case.
	walked := map[string]bool{}
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		key := strings.ToLower(dir)
		if len(walked) > 20000 || depth > 3 || walked[key] {
			return
		}
		walked[key] = true
		entries, e := os.ReadDir(dir)
		if e != nil {
			return
		}
		for _, en := range entries {
			if !en.IsDir() || strings.HasPrefix(en.Name(), ".") || skipDir[en.Name()] {
				continue
			}
			p := filepath.Join(dir, en.Name())
			if p == vault {
				continue
			}
			if _, e := os.Stat(filepath.Join(p, ".git")); e == nil {
				if matches(en.Name(), prefixes) {
					if found[dir] == nil {
						found[dir] = &cand{root: dir}
					}
					found[dir].matching = append(found[dir].matching, en.Name())
				}
				continue // a clone's own directories are not searched
			}
			walk(p, depth+1)
		}
	}
	for _, b := range bases {
		if abs, e := filepath.Abs(b); e == nil {
			walk(abs, 0)
		}
	}
	cands := []*cand{}
	for _, c := range found {
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool {
		if len(cands[i].matching) != len(cands[j].matching) {
			return len(cands[i].matching) > len(cands[j].matching)
		}
		return cands[i].root < cands[j].root
	})
	out := []Object{}
	for i, c := range cands {
		if i == 5 {
			break
		}
		sort.Strings(c.matching)
		ex := c.matching
		if len(ex) > 5 {
			ex = ex[:5]
		}
		out = append(out, Object{"root": c.root, "matching": len(c.matching), "examples": ex})
	}
	return out
}

func matches(name string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
