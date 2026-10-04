package kernel

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

type savedEntry struct {
	mode    fs.FileMode
	content string
}

func TestRecoveryFilesStayOutOfGit(t *testing.T) {
	v := install(t)
	// A retained transaction must stay private even when rollback restores old cell ignore rules.
	if e := os.WriteFile(filepath.Join(v, ".gitignore"), []byte("cell-owned-rule\n"), 0o644); e != nil {
		t.Fatal(e)
	}
	git := func(t *testing.T, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "core.excludesFile=" + filepath.Join(t.TempDir(), "absent-excludes"), "-C", v}, args...)...)
		b, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %s %v", args, b, e)
		}
		return string(b)
	}
	git(t, "init", "-q", "--template=", "--initial-branch=main")
	tx, e := beginUpdate(v, []string{"AGENTS.md"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = tx.cleanup("test completed") })
	git(t, "add", "-A")
	if staged := git(t, "ls-files", "--", updateDir); staged != "" {
		t.Fatalf("git add -A staged private recovery files:\n%s", staged)
	}
	t.Run("partial-cleanup", func(t *testing.T) {
		backupDir := filepath.Join(tx.dir, "before")
		t.Cleanup(func() { _ = os.Chmod(backupDir, 0o700) })
		if e := os.Chmod(backupDir, 0o500); e != nil {
			t.Fatal(e)
		}
		if probe, e := os.CreateTemp(backupDir, "permission-probe-*"); e == nil {
			_ = probe.Close()
			_ = os.Remove(probe.Name())
			t.Skip("filesystem does not enforce directory write permissions")
		} else if !errors.Is(e, os.ErrPermission) {
			t.Fatal(e)
		}
		if e := tx.cleanup("kernel update applied"); !errors.Is(e, os.ErrPermission) {
			t.Fatalf("unwritable backups must report a cleanup error: %v", e)
		}
		if b, e := os.ReadFile(filepath.Join(tx.dir, ".gitignore")); e != nil || string(b) != "*\n" {
			t.Fatalf("partial cleanup must keep the recovery ignore rule: %s %v", b, e)
		}
		git(t, "add", "-A")
		if staged := git(t, "ls-files", "--", updateDir); staged != "" {
			t.Fatalf("partial cleanup exposed retained recovery files to git add:\n%s", staged)
		}
		if _, e := os.Stat(filepath.Join(backupDir, "AGENTS.md")); e != nil {
			t.Fatalf("the unwritable backup must remain available: %v", e)
		}
		if e := os.Chmod(backupDir, 0o700); e != nil {
			t.Fatal(e)
		}
		if e := tx.cleanup("kernel update applied"); e != nil {
			t.Fatalf("retry cleanup after fixing permissions: %v", e)
		}
		if _, e := os.Stat(tx.dir); !os.IsNotExist(e) {
			t.Fatalf("successful cleanup must release the writer lock: %v", e)
		}
	})
}

func TestUpdatePreservesIgnorePermissions(t *testing.T) {
	for _, mode := range []fs.FileMode{0o600, 0o640, 0o660} {
		t.Run(mode.String(), func(t *testing.T) {
			v := install(t)
			full := filepath.Join(v, ".gitignore")
			if e := os.WriteFile(full, []byte("cell-owned-rule\n"), mode); e != nil {
				t.Fatal(e)
			}
			if e := os.Chmod(full, mode); e != nil {
				t.Fatal(e)
			}
			before, e := os.Stat(full)
			if e != nil {
				t.Fatal(e)
			}
			if before.Mode().Perm() != mode {
				t.Skip("filesystem does not support the requested Unix file permissions")
			}
			if _, e := Update(v, Options{}); e != nil {
				t.Fatal(e)
			}
			after, e := os.Stat(full)
			if e != nil {
				t.Fatal(e)
			}
			if after.Mode().Perm() != mode {
				t.Fatalf("successful update changed .gitignore permissions: before=%04o after=%04o", mode, after.Mode().Perm())
			}
			b, e := os.ReadFile(full)
			if e != nil || !strings.Contains(string(b), "cell-owned-rule\n") || !strings.Contains(string(b), "/.investigations/\n") {
				t.Fatalf("update must keep local rules and add kernel rules: %s %v", b, e)
			}
		})
	}
}

func TestApplyReportsCleanupFailure(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		name := "applied"
		if rollback {
			name = "restored"
		}
		t.Run(name, func(t *testing.T) {
			v := install(t)
			if e := os.Chmod(v, 0o555); e != nil {
				t.Fatal(e)
			}
			probe, permission := os.CreateTemp(v, "permission-probe-*")
			if e := os.Chmod(v, 0o755); e != nil {
				t.Fatal(e)
			}
			if permission == nil {
				_ = probe.Close()
				_ = os.Remove(probe.Name())
				t.Skip("filesystem does not enforce directory write permissions")
			}
			if !errors.Is(permission, os.ErrPermission) {
				t.Fatal(permission)
			}
			if e := os.WriteFile(filepath.Join(v, "AGENTS.md"), []byte("previous router\n"), 0o644); e != nil {
				t.Fatal(e)
			}
			p, e := Preview(v)
			if e != nil {
				t.Fatal(e)
			}
			before := vaultState(t, v)
			dir := filepath.Join(v, updateDir)
			t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
			e = apply(v, p, func(full string, data []byte) error {
				if e := writeFile(full, data); e != nil {
					return e
				}
				if rollback || filepath.Base(full) == LockName {
					if e := os.Chmod(dir, 0o500); e != nil {
						return e
					}
					if rollback {
						return &os.PathError{Op: "write", Path: full, Err: syscall.ENOSPC}
					}
				}
				return nil
			})
			if !errors.Is(e, os.ErrPermission) || !strings.Contains(e.Error(), "cleanup failed") || !strings.Contains(e.Error(), dir) {
				t.Fatalf("cleanup failure must be reported with its directory: %v", e)
			}
			if rollback {
				if !errors.Is(e, syscall.ENOSPC) || !strings.Contains(e.Error(), "previous state restored") || strings.Contains(e.Error(), "kernel update applied") {
					t.Fatalf("cleanup must preserve the write error and completed rollback: %v", e)
				}
			} else {
				if !strings.Contains(e.Error(), "kernel update applied") || strings.Contains(e.Error(), "previous state restored") {
					t.Fatalf("cleanup failure must report that the update was applied: %v", e)
				}
				if b, e := os.ReadFile(filepath.Join(v, "AGENTS.md")); e != nil || string(b) != string(p.sources["AGENTS.md"]) {
					t.Fatalf("cleanup must not roll back an applied update: %s %v", b, e)
				}
			}
			if e := os.Chmod(dir, 0o700); e != nil {
				t.Fatal(e)
			}
			if e := Apply(v, p); e == nil || !strings.Contains(e.Error(), "needs recovery") {
				t.Fatalf("leftover recovery files must still be protected: %v", e)
			}
			if e := os.RemoveAll(dir); e != nil {
				t.Fatal(e)
			}
			if rollback {
				assertVaultState(t, v, before)
			}
			if _, e := Update(v, Options{Force: true}); e != nil {
				t.Fatalf("retry after cleanup: %v", e)
			}
		})
	}
}

func TestApplyRestoresFilesAndLockAfterWriteErrors(t *testing.T) {
	for _, failAt := range []string{"first", "second", "lock"} {
		t.Run(failAt, func(t *testing.T) {
			v := install(t)
			for rel, text := range map[string]string{
				"AGENTS.md":          "old router\n",
				".gitignore":         "cell-ignore\n",
				".obsidian/app.json": `{"userIgnoreFilters":[]}`,
			} {
				if e := os.WriteFile(filepath.Join(v, filepath.FromSlash(rel)), []byte(text), 0o644); e != nil {
					t.Fatal(e)
				}
			}
			lock, e := ReadLock(v)
			if e != nil {
				t.Fatal(e)
			}
			lock.KernelVersion = "0.0.1"
			if e := os.WriteFile(filepath.Join(v, LockName), []byte(lock.dump()), 0o640); e != nil {
				t.Fatal(e)
			}
			p, e := Preview(v)
			if e != nil {
				t.Fatal(e)
			}
			before := vaultState(t, v)
			writes := 0
			e = apply(v, p, func(full string, data []byte) error {
				if e := writeFile(full, data); e != nil {
					return e
				}
				writes++
				if failAt == "first" && writes == 1 || failAt == "second" && writes == 2 || failAt == "lock" && filepath.Base(full) == LockName {
					return &os.PathError{Op: "write", Path: full, Err: syscall.ENOSPC}
				}
				return nil
			})
			if !errors.Is(e, syscall.ENOSPC) || !strings.Contains(e.Error(), "previous state restored") {
				t.Fatalf("the write error and completed rollback must be reported: %v", e)
			}
			assertVaultState(t, v, before)
			// An ordinary failed write must not leave a lock that blocks the next attempt.
			if e := Apply(v, p); e != nil {
				t.Fatalf("retry after rollback: %v", e)
			}
			updated, e := ReadLock(v)
			if e != nil || updated.KernelVersion != Version() {
				t.Fatalf("retry must install the current lock: %+v %v", updated, e)
			}
		})
	}
}

func TestApplyPreservesExistingRecoveryFiles(t *testing.T) {
	v := install(t)
	dir := filepath.Join(v, updateDir)
	if e := os.Mkdir(dir, 0o700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "recovery.txt"), []byte("existing recovery data\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	p, e := Preview(v)
	if e != nil {
		t.Fatal(e)
	}
	before := vaultState(t, v)
	if e := Apply(v, p); e == nil || !strings.Contains(e.Error(), "needs recovery") {
		t.Fatalf("an active or unfinished transaction must block a new writer: %v", e)
	}
	assertVaultState(t, v, before)
}

func TestApplyKeepsBackupsIfRestorationFails(t *testing.T) {
	v := install(t)
	if e := os.Chmod(v, 0o555); e != nil {
		t.Fatal(e)
	}
	probe, permission := os.CreateTemp(v, "permission-probe-*")
	if e := os.Chmod(v, 0o755); e != nil {
		t.Fatal(e)
	}
	if permission == nil {
		_ = probe.Close()
		_ = os.Remove(probe.Name())
		t.Skip("filesystem does not enforce directory write permissions")
	}
	if !errors.Is(permission, os.ErrPermission) {
		t.Fatal(permission)
	}
	const original = "original router before the update\n"
	if e := os.WriteFile(filepath.Join(v, "AGENTS.md"), []byte(original), 0o644); e != nil {
		t.Fatal(e)
	}
	p, e := Preview(v)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.Chmod(v, 0o755) })
	e = apply(v, p, func(full string, data []byte) error {
		if e := writeFile(full, data); e != nil {
			return e
		}
		// Simulate a filesystem becoming unwritable after a replacement succeeded.
		if e := os.Chmod(v, 0o555); e != nil {
			return e
		}
		return &os.PathError{Op: "write", Path: full, Err: syscall.ENOSPC}
	})
	if e == nil || !strings.Contains(e.Error(), "rollback failed") || strings.Contains(e.Error(), "previous state restored") {
		t.Fatalf("an incomplete restoration must be reported as a failure: %v", e)
	}
	backup := filepath.Join(v, updateDir, "before", "AGENTS.md")
	if b, e := os.ReadFile(backup); e != nil || string(b) != original {
		t.Fatalf("the previous file must remain available for recovery: %s %v", b, e)
	}
	if e := os.Chmod(v, 0o755); e != nil {
		t.Fatal(e)
	}
	before := vaultState(t, v)
	if e := Apply(v, p); e == nil || !strings.Contains(e.Error(), "needs recovery") {
		t.Fatalf("a new update must keep the available recovery files: %v", e)
	}
	assertVaultState(t, v, before)
}

func TestUpdatePreservesCellOwnedBaseLinks(t *testing.T) {
	v := install(t)
	base := filepath.Join(v, bases[0])
	if e := os.Remove(base); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("missing-cell-owned-base", base); e != nil {
		t.Fatal(e)
	}
	if _, e := Update(v, Options{}); e != nil {
		t.Fatal(e)
	}
	if link, e := os.Readlink(base); e != nil || link != "missing-cell-owned-base" {
		t.Fatalf("an existing cell-owned Base link must be kept: %q %v", link, e)
	}
}

func TestApplyBlocksAConcurrentWriter(t *testing.T) {
	v := install(t)
	if e := os.WriteFile(filepath.Join(v, "AGENTS.md"), []byte("previous router\n"), 0o644); e != nil {
		t.Fatal(e)
	}
	p, e := Preview(v)
	if e != nil {
		t.Fatal(e)
	}
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	defer close(release)
	go func() {
		first := true
		done <- apply(v, p, func(full string, data []byte) error {
			if e := writeFile(full, data); e != nil {
				return e
			}
			if first {
				first = false
				close(started)
				<-release
			}
			return nil
		})
	}()
	select {
	case <-started:
	case e := <-done:
		t.Fatalf("first writer failed before the barrier: %v", e)
	case <-time.After(5 * time.Second):
		t.Fatal("first writer did not reach the barrier")
	}
	before := vaultState(t, v)
	if e := Apply(v, p); e == nil || !strings.Contains(e.Error(), "another kernel update is active") {
		t.Fatalf("the second writer must be refused: %v", e)
	}
	assertVaultState(t, v, before)
	release <- struct{}{}
	if e := <-done; e != nil {
		t.Fatalf("first writer must complete normally: %v", e)
	}
}

func vaultState(t *testing.T, vault string) map[string]savedEntry {
	t.Helper()
	state := map[string]savedEntry{}
	e := filepath.WalkDir(vault, func(full string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if full == vault {
			return nil
		}
		rel, _ := filepath.Rel(vault, full)
		info, e := entry.Info()
		if e != nil {
			return e
		}
		saved := savedEntry{mode: info.Mode()}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			saved.content, e = os.Readlink(full)
		case info.Mode().IsRegular():
			var b []byte
			b, e = os.ReadFile(full)
			saved.content = string(b)
		}
		if e != nil {
			return e
		}
		state[filepath.ToSlash(rel)] = saved
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return state
}

func assertVaultState(t *testing.T, vault string, before map[string]savedEntry) {
	t.Helper()
	after := vaultState(t, vault)
	changed := []string{}
	for rel, entry := range before {
		if current, exists := after[rel]; !exists || current != entry {
			changed = append(changed, rel)
		}
	}
	for rel := range after {
		if _, exists := before[rel]; !exists {
			changed = append(changed, rel)
		}
	}
	slices.Sort(changed)
	if len(changed) > 0 {
		t.Fatalf("failed update changed %d paths: %v", len(changed), changed)
	}
}

func TestApplyRestoresTheVaultAfterALateWriteFailure(t *testing.T) {
	v := install(t)
	put := func(rel, text string, mode fs.FileMode) {
		t.Helper()
		full := filepath.Join(v, filepath.FromSlash(rel))
		if e := os.MkdirAll(filepath.Dir(full), 0o755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(full, []byte(text), mode); e != nil {
			t.Fatal(e)
		}
		if e := os.Chmod(full, mode); e != nil {
			t.Fatal(e)
		}
	}
	put("AGENTS.md", "local router edit\n", 0o640)
	put(".gitignore", "cell-owned-rule\n", 0o600)
	put(".obsidian/app.json", `{"theme":"fixture","userIgnoreFilters":[]}`, 0o644)
	put("90-Meta/retired-dir/retired.md", "previous kernel file\n", 0o640)
	put("10-Sistemas/cell-owned.md", "cell knowledge\n", 0o600)
	lock, e := ReadLock(v)
	if e != nil {
		t.Fatal(e)
	}
	lock.Hashes["90-Meta/retired-dir/retired.md"] = digest([]byte("previous kernel file\n"))
	put(LockName, lock.dump(), 0o640)
	link := filepath.Join(v, ".claude", "skills")
	if e := os.Remove(link); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("previous-skills", link); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(v, bases[0])); e != nil {
		t.Fatal(e)
	}
	p, e := Preview(v)
	if e != nil {
		t.Fatal(e)
	}
	// A release may introduce a managed file in a new directory.
	const added = "90-Meta/new-dir/nested/new.md"
	p.sources[added] = []byte("new kernel file\n")
	p.Changes = append(p.Changes, Change{added, "add"})
	settingsDir := filepath.Join(v, ".obsidian")
	if e := os.Chmod(settingsDir, 0o555); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.Chmod(settingsDir, 0o755) })
	if probe, e := os.CreateTemp(settingsDir, "permission-probe-*"); e == nil {
		_ = probe.Close()
		_ = os.Remove(probe.Name())
		t.Skip("filesystem does not enforce directory write permissions")
	} else if !os.IsPermission(e) {
		t.Fatal(e)
	}
	before := vaultState(t, v)
	if e := Apply(v, p); e == nil || !errors.Is(e, os.ErrPermission) {
		t.Fatalf("the final settings write must fail with a permission error: %v", e)
	}
	assertVaultState(t, v, before)
	if _, e := os.Stat(filepath.Join(v, added)); !os.IsNotExist(e) {
		t.Fatalf("failed update kept the new file: %v", e)
	}
}
