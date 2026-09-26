// platformcheck runs a compiled CLI against disposable synthetic data. It is a
// development verification executable, never part of the installed vault payload.
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed fixture
var fixture embed.FS

type runner struct {
	cli, root, emptyPath string
	passed               int
}

func main() {
	cli := flag.String("cli", "", "CLI path; defaults to kos beside this runner (kos.exe on Windows)")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: platformcheck [--cli PATH]")
		os.Exit(2)
	}
	if *cli == "" {
		executable, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: cannot locate runner:", err)
			os.Exit(1)
		}
		name := "kos"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		*cli = filepath.Join(filepath.Dir(executable), name)
	}
	if e := run(*cli); e != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", e)
		os.Exit(1)
	}
}
func run(cli string) (err error) {
	cli, err = filepath.Abs(cli)
	if err != nil {
		return err
	}
	st, err := os.Stat(cli)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return errors.New("CLI path must be a regular executable")
	}
	workspace, err := os.MkdirTemp("", "kos platform ñ ")
	if err != nil {
		return err
	}
	defer func() {
		if e := os.RemoveAll(workspace); e != nil && err == nil {
			err = fmt.Errorf("fixture cleanup: %w", e)
		}
	}()
	root := filepath.Join(workspace, "vault")
	if err = os.Mkdir(root, 0700); err != nil {
		return err
	}
	r := runner{cli: cli, root: root, emptyPath: filepath.Join(workspace, "empty-path")}
	if err = os.Mkdir(r.emptyPath, 0700); err != nil {
		return err
	}
	err = fs.WalkDir(fixture, "fixture", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if path == "fixture" {
			return nil
		}
		dest := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(path, "fixture/")))
		if d.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		b, e := fixture.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(dest, b, 0600)
	})
	if err != nil {
		return err
	}
	fmt.Printf("Native CLI smoke: %s/%s; disposable synthetic vault; external-tool PATH empty\n", runtime.GOOS, runtime.GOARCH)
	b, err := r.command("version")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(b)) == "" {
		return errors.New("version output is empty")
	}
	r.pass("version")
	status, err := r.object("config", "status", "--vault", root)
	if err != nil {
		return err
	}
	if status["vault_root"] == nil || status["orientation"] == nil {
		return errors.New("config status lacks root or orientation")
	}
	r.pass("config status")
	resolved, err := r.object("config", "resolve", "--vault", root)
	if err != nil {
		return err
	}
	if resolved["status"] != "resolved" {
		return errors.New("config resolve did not resolve fixture")
	}
	r.pass("config resolve without Git")
	b, err = r.command("audit", "--vault", root)
	if err != nil {
		return err
	}
	if !bytes.Contains(b, []byte("ISSUES: 0")) {
		return fmt.Errorf("audit did not confirm zero issues: %s", b)
	}
	r.pass("structural audit")
	links, err := r.object("check", "links", "--vault", root)
	if err != nil {
		return err
	}
	for _, key := range []string{"broken", "alias_targets", "hidden", "duplicates", "orphans"} {
		if a, ok := links[key].([]any); !ok || len(a) != 0 {
			return fmt.Errorf("unexpected link findings in %s", key)
		}
	}
	r.pass("link checks")
	bases, err := r.object("check", "bases", "--vault", root)
	if err != nil {
		return err
	}
	if bases["bases"] != float64(1) {
		return errors.New("expected exactly one checked Base")
	}
	if a, ok := bases["issues"].([]any); !ok || len(a) != 0 {
		return errors.New("unexpected Base issues")
	}
	r.pass("Base checks")
	fmt.Printf("PASS %d/%d checks on %s/%s\n", r.passed, r.passed, runtime.GOOS, runtime.GOARCH)
	fmt.Println("Scope: native executable, config and structural checks. Git-dependent handoffs, installers and full package suites are not covered.")
	return nil
}
func (r *runner) pass(name string) { r.passed++; fmt.Printf("PASS %02d %s\n", r.passed, name) }
func (r *runner) command(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.cli, args...)
	cmd.Dir = r.root
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		if !strings.EqualFold(key, "PATH") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "PATH="+r.emptyPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		return nil, fmt.Errorf("%s: %w; stderr=%s; stdout=%s", strings.Join(args, " "), e, stderr.String(), b)
	}
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("unexpected stderr for %s: %s", args[0], &stderr)
	}
	return b, nil
}
func (r *runner) object(args ...string) (map[string]any, error) {
	b, e := r.command(args...)
	if e != nil {
		return nil, e
	}
	var m map[string]any
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, fmt.Errorf("%s returned invalid JSON: %w", args[0], e)
	}
	return m, nil
}
