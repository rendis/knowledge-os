package check

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type BindingResult struct {
	Pass      bool   `json:"binding_pass"`
	Expected  string `json:"expected"`
	Observed  string `json:"observed,omitempty"`
	VaultName string `json:"vault_name"`
}

func samePath(left, right string) bool {
	a, e1 := os.Stat(left)
	b, e2 := os.Stat(right)
	if e1 == nil && e2 == nil && os.SameFile(a, b) {
		return true
	}
	aPath, bPath := resolved(left), resolved(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(aPath, bPath)
	}
	return aPath == bPath
}

// Binding resolves only through Obsidian's CLI. Installation discovery and launch
// policy remain separate from checking an explicitly selected vault identity.
func Binding(ctx context.Context, root, name string) (BindingResult, error) {
	r := BindingResult{Expected: root, VaultName: strings.TrimSpace(name)}
	if !filepath.IsAbs(root) {
		return r, errors.New("OBSIDIAN_BINDING: --vault-root must be absolute")
	}
	st, e := os.Stat(root)
	if e != nil || !st.IsDir() {
		return r, fmt.Errorf("OBSIDIAN_BINDING: expected vault directory does not exist: %s", root)
	}
	r.Expected = resolved(root)
	if r.VaultName == "" {
		return r, errors.New("OBSIDIAN_BINDING: --vault-name must not be empty")
	}
	executable, e := exec.LookPath("obsidian")
	if e != nil {
		return r, errors.New("OBSIDIAN_BINDING: Obsidian CLI is unavailable")
	}
	cmd := exec.CommandContext(ctx, executable, "vault="+r.VaultName, "vault", "info=path")
	var stdout, stderr limitedOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	e = cmd.Run()
	if stdout.truncated || stderr.truncated {
		return r, errors.New("OBSIDIAN_BINDING: CLI output exceeds 65536 bytes")
	}
	if ctx.Err() != nil {
		return r, fmt.Errorf("OBSIDIAN_BINDING: resolution interrupted: %w", ctx.Err())
	}
	lines := []string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			lines = append(lines, s)
		}
	}
	if e != nil || len(lines) == 0 {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = fmt.Sprint(e)
		}
		return r, fmt.Errorf("OBSIDIAN_BINDING: unable to resolve %q: %s", r.VaultName, detail)
	}
	r.Observed = lines[len(lines)-1]
	if !filepath.IsAbs(r.Observed) || !samePath(r.Observed, r.Expected) {
		return r, fmt.Errorf("OBSIDIAN_BINDING: mismatch for %q; expected %s, observed %s", r.VaultName, r.Expected, r.Observed)
	}
	r.Pass = true
	return r, nil
}

// Keep a faulty external command's output from consuming unbounded memory.
type limitedOutput struct {
	value     []byte
	truncated bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 65536 - len(b.value)
	if len(p) > remaining {
		b.truncated = true
	}
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.value = append(b.value, p...)
	}

	return n, nil
}
func (b *limitedOutput) String() string { return string(b.value) }
func runBinding(args []string, out io.Writer) error {
	root, name := "", ""
	for i := 0; i < len(args); i++ {
		key := args[i]
		if key != "--vault-root" && key != "--vault" && key != "--vault-name" {
			return fmt.Errorf("unknown binding argument %q", key)
		}
		i++
		if i >= len(args) {
			return fmt.Errorf("%s requires a value", key)
		}
		if key == "--vault-name" {
			name = args[i]
		} else {
			root = args[i]
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r, e := Binding(ctx, root, name)
	if err := json.NewEncoder(out).Encode(r); err != nil {
		return err
	}
	return e
}

// resolved is the absolute path with symbolic links followed when the path exists.
func resolved(path string) string {
	abs, e := filepath.Abs(path)
	if e != nil {
		return filepath.Clean(path)
	}
	real, e := filepath.EvalSymlinks(abs)
	if e == nil {
		return real
	}
	return abs
}
