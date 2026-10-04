package config

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const Help = `config --vault PATH COMMAND [--read-only] [options]
  status                         Identity, orientation and configured capabilities
  resolve                        Validate explicit vault markers and Git identity
  capability --capability ID     Resolve operational procedures
  bind --capability ID --procedure BASENAME [--procedure ...] [--expected-hash SHA256]
  database-targets               List portable database identities
  database-target --target ID    Resolve target procedure
  workspace                      Local workspace and proxy configuration
  detect                         Propose this developer's workspace from the machine (read-only):
                                 clones of the cell's repositories, worktree root, cloud logins,
                                 missing database proxy ports, and the command that records them
  workspace-update | workspace-init
    [--repository-root PATH ...] [--managed-clone-root PATH | --disable-managed-clone]
    [--development-worktree-root PATH | --disable-development-worktree-root]
    [--proxy-port ENV=PORT ...] [--expected-hash SHA256]
  locate --remote URL            Match repository Git identity in configured roots
  locate --repo NAME             Checkout and reference branch of a repository, note or alias (no network)
  schema-repository              Locate configured schema repository
  proxy-port --environment NAME  Resolve local database proxy port
  worktree-root                  Resolve configured development worktree root
  catalog                        Validate and return related-vault catalog
  areas | reports                List operational catalog entries
  operation --basename NAME | --report-id ID
All output is JSON. No configured procedure is executed.
--read-only skips machine catalog/update-cache writes and refuses bind and workspace writes.
`

func Run(args []string, out io.Writer) error {
	opts := map[string][]string{}
	cmd := ""
	bools := map[string]bool{"--read-only": true, "--disable-managed-clone": true, "--disable-development-worktree-root": true}
	values := map[string]bool{}
	for _, n := range strings.Fields("--vault --capability --procedure --target --repository-root --managed-clone-root --development-worktree-root --proxy-port --expected-hash --remote --repo --environment --basename --report-id") {
		values[n] = true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--help" || a == "-h" {
			_, e := io.WriteString(out, Help)
			return e
		}
		if bools[a] {
			opts[a] = []string{"true"}
			continue
		}
		if values[a] {
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			i++
			opts[a] = append(opts[a], args[i])
			continue
		}
		if strings.HasPrefix(a, "-") {
			return fmt.Errorf("unknown option %s", a)
		}
		if cmd != "" {
			return errors.New("expected exactly one config command")
		}
		cmd = a
	}
	one := func(k string) string {
		a := opts[k]
		if len(a) == 0 {
			return ""
		}
		return a[0]
	}
	for k, v := range opts {
		if len(v) > 1 && k != "--procedure" && k != "--repository-root" && k != "--proxy-port" {
			return fmt.Errorf("duplicate option %s", k)
		}
	}
	if cmd == "" {
		return errors.New("config command is required; use config --help")
	}
	accepted := map[string]string{
		"status": "", "detect": "", "resolve": "", "capability": "--capability", "bind": "--capability --procedure --expected-hash", "database-targets": "", "database-target": "--target", "workspace": "", "workspace-update": "--repository-root --managed-clone-root --disable-managed-clone --development-worktree-root --disable-development-worktree-root --proxy-port --expected-hash", "workspace-init": "--repository-root --managed-clone-root --disable-managed-clone --development-worktree-root --disable-development-worktree-root --proxy-port --expected-hash", "locate": "--remote --repo", "schema-repository": "", "proxy-port": "--environment", "worktree-root": "", "catalog": "", "areas": "", "reports": "", "operation": "--basename --report-id",
	}
	allowedFlags, known := accepted[cmd]
	if !known {
		return fmt.Errorf("unknown config command %s", cmd)
	}
	if _, readOnly := opts["--read-only"]; readOnly {
		switch cmd {
		case "bind", "workspace-init", "workspace-update":
			return fmt.Errorf("--read-only refuses writing config command %s", cmd)
		}
	}
	for flag := range opts {
		if flag != "--vault" && flag != "--read-only" && !strings.Contains(" "+allowedFlags+" ", " "+flag+" ") {
			return fmt.Errorf("%s is not supported for %s", flag, cmd)
		}
	}
	if cmd == "resolve" {
		result, e := ResolvePath(one("--vault"))
		if e != nil {
			return e
		}
		return emit(out, result)
	}
	root, e := CanonicalRoot(one("--vault"))
	if e != nil {
		return e
	}
	var result any
	switch cmd {
	case "resolve":
		result, e = Resolve(root)
	case "workspace":
		result, e = Workspace(root)
	case "detect":
		result, e = Detect(root)
	case "catalog":
		result, e = Catalog(root)
	case "locate":
		switch {
		case (one("--remote") == "") == (one("--repo") == ""):
			e = errors.New("locate takes exactly one of --remote URL or --repo NAME")
		case one("--repo") != "":
			result, e = LocateRepositoryByName(root, one("--repo"))
		default:
			result, e = LocateRepository(root, one("--remote"))
		}
	case "schema-repository":
		result, e = SchemaRepository(root)
	case "areas":
		result, e = OperationalAreas(root)
	case "reports":
		result, e = OperationalReports(root)
	case "operation":
		var notes []Object
		notes, e = OperationalNotes(root)
		if e == nil {
			key, value := "basename", one("--basename")
			if one("--report-id") != "" {
				if value != "" {
					return errors.New("choose basename or report-id")
				}
				key, value = "report-id", one("--report-id")
			}
			if value == "" {
				return errors.New("operation requires basename or report-id")
			}
			matches := []Object{}
			for _, n := range notes {
				if n[key] == value {
					matches = append(matches, n)
				}
			}
			if len(matches) != 1 {
				return errors.New("operational note missing or ambiguous")
			}
			result = matches[0]
		}
	case "proxy-port", "worktree-root":
		var w Object
		w, e = Workspace(root)
		if e == nil {
			if cmd == "worktree-root" {
				result = Object{"worktree_root": w["worktree_root"]}
			} else {
				env := one("--environment")
				if env == "" {
					return errors.New("environment is required")
				}
				p, ok := obj(w["proxy_ports"])[env]
				status := "ok"
				if !ok {
					status = "proxy_port_not_configured"
				}
				result = Object{"status": status, "environment": env, "port": p}
			}
		}
	case "workspace-init", "workspace-update":
		u := WorkspaceUpdate{Roots: opts["--repository-root"], Initialize: cmd == "workspace-init", ExpectedHash: one("--expected-hash"), Ports: map[string]string{}}
		if v, ok := opts["--managed-clone-root"]; ok {
			u.ManagedRoot = &v[0]
		}
		if v, ok := opts["--development-worktree-root"]; ok {
			u.WorktreeRoot = &v[0]
		}
		if _, ok := opts["--disable-managed-clone"]; ok {
			if u.ManagedRoot != nil {
				return errors.New("conflicting managed clone options")
			}
			s := ""
			u.ManagedRoot = &s
		}
		if _, ok := opts["--disable-development-worktree-root"]; ok {
			if u.WorktreeRoot != nil {
				return errors.New("conflicting worktree options")
			}
			s := ""
			u.WorktreeRoot = &s
		}
		for _, v := range opts["--proxy-port"] {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				return errors.New("proxy-port requires ENV=PORT")
			}
			env, p := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if _, ok := u.Ports[env]; ok {
				return errors.New("duplicate environment proxy port")
			}
			u.Ports[env] = p
		}
		result, e = UpdateWorkspace(root, u)
	case "bind":
		result, e = Bind(root, one("--capability"), opts["--procedure"], one("--expected-hash"))
	case "status", "capability", "database-targets", "database-target":
		var m Object
		m, e = LoadInstance(root)
		if e != nil {
			break
		}
		switch cmd {
		case "status":
			names := []string{}
			for name := range obj(m["capabilities"]) {
				names = append(names, name)
			}
			sort.Strings(names)
			caps := []Object{}
			for _, n := range names {
				c, ce := ResolveCapability(root, m, n)
				if ce != nil {
					return ce
				}
				caps = append(caps, c)
			}
			result = Object{"vault_root": root, "orientation": Orientation(root, m), "capabilities": caps, "platform_providers": PlatformProviders(m)}
			if w, we := Workspace(root); we == nil {
				result.(Object)["workspace"] = w["status"] // uninitialized: this developer's onboarding is pending
			}
		case "capability":
			if one("--capability") == "" {
				return errors.New("capability is required")
			}
			result, e = ResolveCapability(root, m, one("--capability"))
		case "database-targets":
			targets := m["database_targets"]
			if targets == nil {
				targets = []any{}
			}
			result = Object{"targets": targets}
		case "database-target":
			id := one("--target")
			if id == "" {
				return errors.New("target is required")
			}
			result = Object{"status": "not_configured", "target": id}
			for _, x := range list(m["database_targets"]) {
				target := obj(x)
				if target["id"] == id {
					c, ce := ResolveCapability(root, Object{"capabilities": Object{"database-inspection": []any{target["procedure"]}}}, "database-inspection")
					if ce != nil {
						return ce
					}
					result = Object{"status": "configured", "target": target, "procedures": c["procedures"]}
					break
				}
			}
		}
	default:
		return fmt.Errorf("unknown config command %s", cmd)
	}
	if e != nil {
		return e
	}
	return emit(out, result)
}
