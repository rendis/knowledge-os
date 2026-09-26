package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knowledge-os/internal/kernel"
)

func vaultWithKernel(t *testing.T, kv string) string {
	t.Helper()
	v := t.TempDir()
	os.WriteFile(filepath.Join(v, "instance.yaml"), []byte("version: 1\ncell:\n  name: C\n  purpose: p\nsystems:\n  - id: s\n    name: S\n"), 0o644)
	os.WriteFile(filepath.Join(v, "AGENTS.md"), []byte("# router\n"), 0o644)
	os.WriteFile(filepath.Join(v, "00-Home.md"), []byte("# Home\n"), 0o644)
	os.WriteFile(filepath.Join(v, kernel.LockName), []byte("version: \"5\"\nkernel_version: \""+kv+"\"\nadapters:\n  []\nmanaged_hashes:\n"), 0o644)
	return v
}

func TestVaultDefaultsToTheEnclosingDirectory(t *testing.T) {
	v := vaultWithKernel(t, kernel.Version())
	sub := filepath.Join(v, "20-Repos")
	os.MkdirAll(sub, 0o755)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(sub)
	got := withVault([]string{"sync", "status"})
	if len(got) != 4 || got[2] != "--vault" || !strings.HasSuffix(got[3], filepath.Base(v)) {
		t.Fatalf("--vault is inserted after the verb: %v", got)
	}
	t.Setenv("KOS_VAULT", "/elsewhere")
	if got := withVault([]string{"audit"}); got[2] != "/elsewhere" {
		t.Fatalf("KOS_VAULT wins: %v", got)
	}
	if got := withVault([]string{"sync", "status", "--vault", "/given"}); len(got) != 4 {
		t.Fatalf("an explicit --vault is kept: %v", got)
	}
}

func TestKernelVersionNoticesAndGates(t *testing.T) {
	t.Setenv("KOS_NO_UPDATE_CHECK", "1")
	var out, errs bytes.Buffer
	older := vaultWithKernel(t, "0.0.1")
	_ = run(context.Background(), []string{"config", "status", "--vault", older}, &out, &errs)
	if !strings.Contains(errs.String(), "kos kernel update") {
		t.Fatalf("an older vault kernel is announced: %s", errs.String())
	}
	newer := vaultWithKernel(t, "99.0.0")
	errs.Reset()
	if e := run(context.Background(), []string{"sync", "verify", "--vault", newer}, &out, &errs); e == nil || !strings.Contains(e.Error(), "kos update") {
		t.Fatalf("gates refuse a kos older than the vault: %v", e)
	}
	errs.Reset()
	_ = run(context.Background(), []string{"config", "status", "--vault", newer}, &out, &errs)
	if !strings.Contains(errs.String(), "kos update") {
		t.Fatalf("reads only warn: %s", errs.String())
	}
}
