package handoff

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func testGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}

func worktreeFixture(t *testing.T) WorktreeOptions {
	t.Helper()
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	source := filepath.Join(root, "repositories", "Example")
	remote := filepath.Join(root, "remote.git")
	worktrees := filepath.Join(root, "worktrees")
	bundlePath := filepath.Join(root, "bundle")
	for _, path := range []string{vault, source, remote, worktrees, bundlePath} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, remote, "init", "--bare", "--initial-branch=main")
	testGit(t, source, "init", "--initial-branch=main")
	testGit(t, source, "config", "user.name", "Fixture")
	testGit(t, source, "config", "user.email", "fixture@example.invalid")
	testGit(t, source, "config", "commit.gpgsign", "false")
	write(t, filepath.Join(source, "README.md"), []byte("Example\n"))
	testGit(t, source, "add", "README.md")
	testGit(t, source, "commit", "-m", "initial")
	remoteURL := "https://example.invalid/team/Example.git"
	testGit(t, source, "config", "url."+filepath.ToSlash(remote)+".insteadOf", remoteURL)
	testGit(t, source, "remote", "add", "origin", remoteURL)
	testGit(t, source, "push", "origin", "main")
	// Locator reads get-url, which applies insteadOf. Keep network rewriting in
	// process Git config instead, while the stored remote stays canonical.
	testGit(t, source, "config", "--unset-all", "url."+filepath.ToSlash(remote)+".insteadOf")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+filepath.ToSlash(remote)+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", remoteURL)
	// The shared locator currently uses get-url; canonical identities must be
	// read from stored config. This fixture exercises that distinction.
	instance := `version: 1
cell:
  name: Example
  purpose: Test
systems:
  - id: example
    name: Example
    aliases: []
trackers:
  - id: tracker
    provider: jira
    url: https://tracker.example.invalid
evidence:
  profile: documented-source
locale:
  notes: en
`
	write(t, filepath.Join(vault, "instance.yaml"), []byte(instance))
	workspace := map[string]any{"version": 1, "workspace": map[string]any{"repository_roots": []string{filepath.Dir(source)}}, "skills": map[string]any{"manage-development-handoff": map[string]any{"worktree_root": worktrees}}}
	data, _ := yaml.Marshal(workspace)
	write(t, filepath.Join(vault, ".knowledge-os-config.yaml"), data)
	metadata := map[string]any{"schema-version": 2, "source": map[string]any{"investigation-id": "case", "investigation-updated-at": "2026-09-24T00:00:00Z", "story-id": "S-001"}, "work-item": map[string]any{"tracker-id": "tracker", "provider": "jira", "tracker-url": "https://tracker.example.invalid", "reference": "EX-123", "url": "https://tracker.example.invalid/browse/EX-123", "updated-at": "2026-09-24T00:00:00Z", "captured-at": "2026-09-24T00:00:00Z", "freshness": "current", "snapshot-source": "user-supplied-export"}, "repository": map[string]any{"remote": remoteURL}, "change": map[string]any{"summary": "initial export"}}
	data, _ = yaml.Marshal(metadata)
	write(t, filepath.Join(bundlePath, "bundle.yaml"), data)
	for _, name := range []string{"context.md", "work-item.md", "scope.md"} {
		write(t, filepath.Join(bundlePath, name), []byte("# EX-123\n\nExample content.\n"))
	}
	return WorktreeOptions{Vault: vault, Bundle: bundlePath, BaseBranch: "main", BaseSource: "local", Description: "bounded change", BranchPrefix: "issue"}
}

func TestBundleValidatesIdentityAndSecret(t *testing.T) {
	o := worktreeFixture(t)
	if _, err := loadBundle(o.Bundle); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(o.Bundle, "scope.md"), []byte("password: private-value"))
	if _, err := loadBundle(o.Bundle); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected secret rejection: %v", err)
	}
}

func TestPlanCreateWorktree(t *testing.T) {
	o := worktreeFixture(t)
	p, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(p.Worktree["path"]); !os.IsNotExist(err) {
		t.Fatal("planning wrote destination")
	}
	created, err := CreateWorktree(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "created" {
		t.Fatal(created)
	}
	if got := testGit(t, created.Worktree["path"], "rev-parse", "HEAD"); got != p.Base["selected_commit"] {
		t.Fatal("wrong base")
	}
}

func TestPlanRejectsChangedBundle(t *testing.T) {
	o := worktreeFixture(t)
	p, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(o.Bundle, "context.md"), []byte("new bytes"))
	if _, err = CreateWorktree(o, p.Token); err == nil || !strings.Contains(err.Error(), "plan_stale") {
		t.Fatalf("expected stale token: %v", err)
	}
	if _, err = os.Stat(p.Worktree["path"]); !os.IsNotExist(err) {
		t.Fatal("stale plan created path")
	}
}

func TestPlanRejectsOccupiedDestination(t *testing.T) {
	o := worktreeFixture(t)
	p, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(p.Worktree["path"], 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = CreateWorktree(o, p.Token); err == nil {
		t.Fatal("accepted occupied path")
	}
}

func TestRunWorktreePlanning(t *testing.T) {
	o := worktreeFixture(t)
	var out bytes.Buffer
	err := Run([]string{"plan-worktree", "--vault-root", o.Vault, "--bundle", o.Bundle, "--base-branch", "main", "--base-source", "local", "--description", "example"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "plan_token") {
		t.Fatal(fmt.Sprintf("missing token: %s", out.String()))
	}
}

func TestRemoteBaseAdvanceInvalidatesPlan(t *testing.T) {
	o := worktreeFixture(t)
	p, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	source := p.Source["path"]
	write(t, filepath.Join(source, "README.md"), []byte("advance\n"))
	testGit(t, source, "add", "README.md")
	testGit(t, source, "commit", "-m", "advance")
	testGit(t, source, "push", "origin", "main")
	if _, err = CreateWorktree(o, p.Token); err == nil || !strings.Contains(err.Error(), "plan_stale") {
		t.Fatalf("accepted moved base: %v", err)
	}
}

func TestRemoteBaseFetchAndCAS(t *testing.T) {
	o := worktreeFixture(t)
	p, err := PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	source := p.Source["path"]
	root := filepath.Dir(o.Vault)
	other := filepath.Join(root, "other")
	remote := filepath.Join(root, "remote.git")
	testGit(t, root, "clone", remote, other)
	testGit(t, other, "config", "user.name", "Fixture")
	testGit(t, other, "config", "user.email", "fixture@example.invalid")
	testGit(t, other, "config", "commit.gpgsign", "false")
	write(t, filepath.Join(other, "new.md"), []byte("remote change\n"))
	testGit(t, other, "add", "new.md")
	testGit(t, other, "commit", "-m", "remote change")
	testGit(t, other, "push", "origin", "main")
	o.BaseSource = "remote"
	p, err = PlanWorktree(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Base["fetch_required"] != true || p.Base["tracking_update_required"] != true {
		t.Fatalf("expected remote effects: %+v", p.Base)
	}
	localBefore := commit(source, "refs/heads/main")
	created, err := CreateWorktree(o, p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if commit(created.Worktree["path"], "HEAD") != p.Base["remote_commit"] {
		t.Fatal("did not use approved remote commit")
	}
	if commit(source, "refs/heads/main") != localBefore {
		t.Fatal("modified local base branch")
	}
}

func TestSlugNormalizationAndEmpty(t *testing.T) {
	if slug("Revisión Straße") != "revision-strasse" {
		t.Fatal(slug("Revisión Straße"))
	}
	if slug("中文") != "" {
		t.Fatal("non-Latin slug should fail canonical generation")
	}
}

func TestSemanticBytesNormalization(t *testing.T) {
	if got := string(semanticBytes([]byte("\ufeff\r\nCafe\u0301 \t\r\n\r\n"))); got != "Café\n" {
		t.Fatalf("unexpected semantic normalization %q", got)
	}
}
