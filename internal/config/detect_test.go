package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectProposesTheDevelopersWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repos := filepath.Join(home, "Projects", "acme", "repos")
	for _, r := range []string{"APP1-orders", "APP1-billing", "OTHER-tool"} {
		if e := os.MkdirAll(filepath.Join(repos, r, ".git"), 0o755); e != nil {
			t.Fatal(e)
		}
	}
	vault := filepath.Join(repos, "APP1-documentation-vault")
	write(t, vault, "instance.yaml", instanceFixture+"sources:\n  repo_prefixes: [\"APP1\"]\nplatform:\n  providers: [gcp, aws]\ndatabase_targets:\n  - id: orders-prd\n    system: demo\n    environment: prd\n    instance: orders\n    database: orders\n    procedure: Inspect DB\n    port_key: prd\n")
	origLook, origRun := lookPath, detectRun
	t.Cleanup(func() { lookPath, detectRun = origLook, origRun })
	lookPath = func(name string) (string, error) {
		if name == "gcloud" || name == "aws" {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	detectRun = func(_ time.Duration, name string, args ...string) ([]byte, error) {
		if name == "gcloud" {
			return []byte("dev@example.test\n"), nil
		}
		return nil, errors.New("Unable to locate credentials")
	}
	d, e := Detect(vault)
	if e != nil {
		t.Fatal(e)
	}
	cands := d["repositories"].(Object)["candidates"].([]Object)
	if len(cands) == 0 || cands[0]["root"] != repos || cands[0]["matching"] != 2 {
		t.Fatalf("the directory holding the cell's clones is proposed: %v", cands)
	}
	if d["worktree_root"].(Object)["candidate"] != filepath.Join(home, "Projects", "acme", "worktrees") {
		t.Fatalf("worktrees beside the clones: %v", d["worktree_root"])
	}
	clouds := d["clouds"].([]Object)
	if clouds[0]["status"] != "logged-in" || clouds[0]["account"] != "dev@example.test" || clouds[1]["status"] != "not-logged-in" {
		t.Fatalf("cloud logins: %v", clouds)
	}
	propose := strings.Join(d["propose"].([]string), " ")
	missing := strings.Join(d["missing"].([]string), " | ")
	if !strings.Contains(propose, "workspace-init --repository-root \""+repos+"\"") || !strings.Contains(missing, "aws sso login") || !strings.Contains(missing, "orders-prd") {
		t.Fatalf("one command to confirm and each gap with its fix: %s / %s", propose, missing)
	}
	for _, c := range cands {
		for _, ex := range c["examples"].([]string) {
			if ex == "APP1-documentation-vault" || ex == "OTHER-tool" {
				t.Fatalf("the vault and repositories outside the prefixes are not sources: %v", c)
			}
		}
	}
}
