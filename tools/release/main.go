// Release builds are development tooling; consumers receive only kos.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func build() error {
	targets := flag.String("targets", "darwin/arm64,darwin/amd64,linux/arm64,linux/amd64,windows/arm64,windows/amd64", "comma-separated OS/architecture")
	output := flag.String("output", "dist", "artifact directory")
	flag.Parse()
	version, err := os.ReadFile("kernel/VERSION")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	fingerprint, err := sourceFingerprint(".")
	if err != nil {
		return err
	}
	artifacts := map[string]map[string]string{}
	// An interrupted or failed build must not leave an older release descriptor usable.
	if err = os.Remove(filepath.Join(*output, "release.json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	revision, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return err
	}
	dirty, err := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	if err != nil {
		return err
	}
	stamp := fmt.Sprintf("-s -w -X main.version=%s -X knowledge-os/internal/kernel.Revision=%s -X knowledge-os/internal/kernel.Dirty=%t",
		strings.TrimSpace(string(version)), strings.TrimSpace(string(revision)), len(bytes.TrimSpace(dirty)) > 0)
	var sums []string
	for _, target := range strings.Split(*targets, ",") {
		parts := strings.Split(target, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid target %s", target)
		}
		name := "kos-" + parts[0] + "-" + parts[1]
		if parts[0] == "windows" {
			name += ".exe"
		}
		path := filepath.Join(*output, name)
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags="+stamp, "-o", path, "./cmd/kos")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+parts[0], "GOARCH="+parts[1])
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", target, err)
		}
		f, e := os.Open(path)
		if e != nil {
			return e
		}
		h := sha256.New()
		_, e = io.Copy(h, f)
		f.Close()
		if e != nil {
			return e
		}
		checksum := hex.EncodeToString(h.Sum(nil))
		artifacts[target] = map[string]string{"file": name, "sha256": checksum}
		sums = append(sums, checksum+"  "+name)
		fmt.Println(name)
	}
	sort.Strings(sums)
	if err = os.WriteFile(filepath.Join(*output, "SHA256SUMS"), []byte(strings.Join(sums, "\n")+"\n"), 0644); err != nil {
		return err
	}
	if err = notices(*output, strings.Split(*targets, ",")); err != nil {
		return err
	}
	noticesBytes, err := os.ReadFile(filepath.Join(*output, "THIRD_PARTY_NOTICES.txt"))
	if err != nil {
		return err
	}
	noticeHash := sha256.Sum256(noticesBytes)
	// The installer scripts and the version ship with the binaries: a mirror of dist/ is a release.
	for _, script := range []string{"install-kos.sh", "install-kos.ps1"} {
		b, e := os.ReadFile(filepath.Join("scripts", script))
		if e != nil {
			return e
		}
		if e := os.WriteFile(filepath.Join(*output, script), b, 0755); e != nil {
			return e
		}
	}
	if err = os.WriteFile(filepath.Join(*output, "VERSION"), version, 0644); err != nil {
		return err
	}
	finalFingerprint, err := sourceFingerprint(".")
	if err != nil {
		return err
	}
	if finalFingerprint != fingerprint {
		return fmt.Errorf("native source changed during build; rebuild the complete release")
	}
	manifest := map[string]any{"schema": 1, "version": strings.TrimSpace(string(version)), "source_revision": strings.TrimSpace(string(revision)), "source_dirty": len(bytes.TrimSpace(dirty)) > 0, "source_fingerprint": fingerprint, "artifacts": artifacts, "notices_sha256": hex.EncodeToString(noticeHash[:])}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(*output, "release.json"), append(encoded, '\n'), 0644)
}
func notices(output string, targets []string) error {
	type module struct {
		Path, Version, Dir string
		Main               bool
		Replace            *module
	}
	modules := map[string]module{}
	for _, target := range targets {
		parts := strings.Split(target, "/")
		cmd := exec.Command("go", "list", "-deps", "-json", "./cmd/kos")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+parts[0], "GOARCH="+parts[1])
		raw, err := cmd.Output()
		if err != nil {
			return err
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		for {
			var pkg struct{ Module *module }
			err = decoder.Decode(&pkg)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if pkg.Module != nil {
				m := *pkg.Module
				if m.Replace != nil {
					m = *m.Replace
				}
				modules[m.Path] = m
			}
		}
	}
	keys := make([]string, 0, len(modules))
	for key := range modules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	runtimeLicense, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "LICENSE"))
	if err != nil {
		return err
	}
	out.WriteString("=== Go runtime " + runtime.Version() + " ===\n" + string(runtimeLicense) + "\n")
	for _, key := range keys {
		mod := modules[key]
		out.WriteString("\n=== " + mod.Path + " " + mod.Version + " ===\n")
		entries, err := os.ReadDir(mod.Dir)
		if err != nil {
			return err
		}
		found := false
		for _, entry := range entries {
			name := strings.ToUpper(entry.Name())
			if entry.IsDir() || !(strings.HasPrefix(name, "LICENSE") || strings.HasPrefix(name, "COPYING") || name == "NOTICE" || name == "COPYRIGHT") {
				continue
			}
			b, e := os.ReadFile(filepath.Join(mod.Dir, entry.Name()))
			if e != nil {
				return e
			}
			out.WriteString(entry.Name() + "\n" + string(b) + "\n")
			found = true
		}
		if !found {
			return fmt.Errorf("missing redistribution notice for %s", mod.Path)
		}
	}
	return os.WriteFile(filepath.Join(output, "THIRD_PARTY_NOTICES.txt"), []byte(strings.TrimRight(out.String(), "\r\n")+"\n"), 0644)
}

func sourceFingerprint(root string) (string, error) {
	files := []string{"go.mod", "go.sum", "payload.go", "MANAGED_PATHS"}
	for _, base := range []string{"cmd", "internal", "kernel", "adapters"} {
		if err := filepath.WalkDir(filepath.Join(root, base), func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("native build input must not be a symlink: %s", path)
			}
			if d.IsDir() || strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		}); err != nil {
			return "", err
		}
	}
	sort.Strings(files)
	h := sha256.New()
	for _, relative := range files {
		p := filepath.Join(root, filepath.FromSlash(relative))
		st, e := os.Lstat(p)
		if e != nil {
			return "", e
		}
		if !st.Mode().IsRegular() {
			return "", fmt.Errorf("unsafe native build input %s", relative)
		}
		f, e := os.Open(p)
		if e != nil {
			return "", e
		}
		sum := sha256.New()
		_, e = io.Copy(sum, f)
		closeErr := f.Close()
		if e != nil {
			return "", e
		}
		if closeErr != nil {
			return "", closeErr
		}
		fmt.Fprintf(h, "%s\x00%s\n", relative, hex.EncodeToString(sum.Sum(nil)))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
