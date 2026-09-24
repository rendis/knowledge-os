package handoff

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransactionCommit(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "AGENTS.md")
	os.WriteFile(p, []byte("old"), 0640)
	if err := ApplyTransaction(root, strings.Repeat("a", 64), map[string][]byte{"AGENTS.md": []byte("new"), storeName + "/issue--repo/manifest.yaml": []byte("manifest")}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "new" {
		t.Fatal(string(got))
	}
	if _, err := os.Stat(filepath.Join(root, storeName, transactionName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
func TestTransactionVerificationRollback(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("old"), 0600)
	err := WithStoreLock(root, func() error {
		return applyTransactionLocked(root, strings.Repeat("a", 64), map[string][]byte{"AGENTS.md": []byte("new"), storeName + "/issue--repo/manifest.yaml": []byte("manifest")}, func() error { return errors.New("invalid output") })
	})
	if err == nil || !strings.Contains(err.Error(), "invalid output") {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(raw) != "old" {
		t.Fatal(string(raw))
	}
	if _, err = os.Stat(filepath.Join(root, storeName, "issue--repo", "manifest.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
func interruptedJournal(t *testing.T, root string) {
	t.Helper()
	txn := filepath.Join(root, storeName, transactionName)
	os.MkdirAll(txn, 0700)
	before := []byte("old")
	desired := []byte("new")
	j := transactionJournal{Format: nativeJournalFormat, Token: strings.Repeat("a", 64), Images: []transactionImage{{Path: "AGENTS.md", Before: before, Desired: desired, BeforeHash: imageHash(before), DesiredHash: imageHash(desired), Mode: 0600}}}
	raw, _ := json.Marshal(j)
	os.WriteFile(filepath.Join(txn, "transaction.json"), raw, 0600)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), desired, 0600)
}
func TestTransactionRecovery(t *testing.T) {
	root := t.TempDir()
	interruptedJournal(t, root)
	if err := Recover(root, strings.Repeat("b", 64)); err == nil {
		t.Fatal("wrong token accepted")
	}
	if err := Recover(root, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(raw) != "old" {
		t.Fatal(string(raw))
	}
	if err := Recover(root, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
}
func TestTransactionRecoveryConflict(t *testing.T) {
	root := t.TempDir()
	interruptedJournal(t, root)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("user edit"), 0600)
	if err := Recover(root, strings.Repeat("a", 64)); err == nil {
		t.Fatal("edit overwritten")
	}
	raw, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(raw) != "user edit" {
		t.Fatal(string(raw))
	}
}
func TestTransactionLegacyJournalRefused(t *testing.T) {
	root := t.TempDir()
	txn := filepath.Join(root, storeName, transactionName)
	os.MkdirAll(txn, 0700)
	os.WriteFile(filepath.Join(txn, "transaction.json"), []byte(`{"schema-version":1,"plan-token":"x"}`), 0600)
	if err := Recover(root, strings.Repeat("a", 64)); err == nil || !strings.Contains(err.Error(), "legacy") {
		t.Fatal(err)
	}
	if _, err := os.Stat(txn); err != nil {
		t.Fatal(err)
	}
}
func TestTransactionUnsafePath(t *testing.T) {
	for _, path := range []string{"../AGENTS.md", "random.md", storeName + "/.ACTIVE.lock", storeName + "/issue--repo/../ACTIVE.yaml"} {
		t.Run(path, func(t *testing.T) {
			if err := ApplyTransaction(t.TempDir(), strings.Repeat("a", 64), map[string][]byte{path: []byte("x")}); err == nil {
				t.Fatal("unsafe path allowed")
			}
		})
	}
}
func TestTransactionSymlinkRefused(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, storeName)); err != nil {
		t.Skip(err)
	}
	if err := ApplyTransaction(root, strings.Repeat("a", 64), map[string][]byte{"AGENTS.md": []byte("x")}); err == nil {
		t.Fatal("symlink followed")
	}
}
func TestTransactionConcurrentLock(t *testing.T) {
	root := t.TempDir()
	if err := WithStoreLock(root, func() error {
		if err := ApplyTransaction(root, strings.Repeat("a", 64), map[string][]byte{"AGENTS.md": []byte("x")}); err == nil {
			return errors.New("second lock accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionProcessInterruption(t *testing.T) {
	if root := os.Getenv("HANDOFF_TRANSACTION_CRASH_ROOT"); root != "" {
		_ = WithStoreLock(root, func() error {
			return applyTransactionLocked(root, strings.Repeat("a", 64), map[string][]byte{"AGENTS.md": []byte("new")}, func() error { os.Exit(73); return nil })
		})
		os.Exit(74)
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("old"), 0600)
	cmd := exec.Command(os.Args[0], "-test.run=^TestTransactionProcessInterruption$")
	cmd.Env = append(os.Environ(), "HANDOFF_TRANSACTION_CRASH_ROOT="+root)
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 73 {
		t.Fatalf("unexpected child result: %v", err)
	}
	if err := Recover(root, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(raw) != "old" {
		t.Fatal(string(raw))
	}
}

func legacyFixture(t *testing.T, root string, familyExisted bool) string {
	t.Helper()
	txn := filepath.Join(root, storeName, transactionName)
	if err := os.MkdirAll(txn, 0700); err != nil {
		t.Fatal(err)
	}
	paths := []map[string]any{
		{"path": "AGENTS.md", "existed": true, "backup": "0000.bak", "postimage-sha256": imageHash([]byte("new"))},
		{"path": storeName + "/issue--repo/history/v0001.md", "existed": false, "backup": nil, "postimage-sha256": imageHash([]byte("event"))},
	}
	metadata := map[string]any{"schema-version": 1, "plan-token": strings.Repeat("a", 64), "bundle-fingerprint": strings.Repeat("b", 64), "handoff-id": strings.Repeat("c", 64), "family": "issue--repo", "family-existed": familyExisted, "paths": paths}
	raw, _ := json.Marshal(metadata)
	os.WriteFile(filepath.Join(txn, "transaction.json"), raw, 0600)
	os.WriteFile(filepath.Join(txn, "0000.bak"), []byte("old"), 0600)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("new"), 0600)
	family := filepath.Join(root, storeName, "issue--repo", "history")
	os.MkdirAll(family, 0700)
	os.WriteFile(filepath.Join(family, "v0001.md"), []byte("event"), 0600)
	return txn
}
func TestLegacyTransactionRecovery(t *testing.T) {
	root := t.TempDir()
	legacyFixture(t, root, false)
	if err := Recover(root, strings.Repeat("b", 64)); err == nil {
		t.Fatal("wrong token accepted")
	}
	if err := Recover(root, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(raw) != "old" {
		t.Fatal(string(raw))
	}
	if _, err := os.Stat(filepath.Join(root, storeName, "issue--repo")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
func TestLegacyRecoveryUnknownContentPreserved(t *testing.T) {
	root := t.TempDir()
	legacyFixture(t, root, false)
	os.WriteFile(filepath.Join(root, storeName, "issue--repo", "user-note.md"), []byte("keep"), 0600)
	if err := Recover(root, strings.Repeat("a", 64)); err == nil {
		t.Fatal("unexpected content accepted")
	}
	raw, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(raw) != "new" {
		t.Fatal("recovery mutated before preflight completed")
	}
}
func TestLegacyRecoveryChangedTargetPreserved(t *testing.T) {
	root := t.TempDir()
	legacyFixture(t, root, true)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("user edit"), 0600)
	if err := Recover(root, strings.Repeat("a", 64)); err == nil {
		t.Fatal("user edit overwritten")
	}
}
func TestLegacyPrepareCleanup(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, storeName)
	prepare := filepath.Join(store, ".APPLY.transaction.prepare")
	os.MkdirAll(prepare, 0700)
	os.WriteFile(filepath.Join(prepare, "0000.bak"), []byte("old"), 0600)
	if err := Recover(root, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(prepare); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestLegacyInterruptedCommittedCleanup(t *testing.T) {
	root := t.TempDir()
	txn := filepath.Join(root, storeName, transactionName)
	os.MkdirAll(txn, 0700)
	os.WriteFile(filepath.Join(txn, "COMMITTED"), []byte("committed\n"), 0600)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("committed content"), 0600)
	if err := Recover(root, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(raw) != "committed content" {
		t.Fatal(string(raw))
	}
}

// Older handoff versions included CLAUDE.md in authorized transactions.
func TestTransactionRecoveryPreviousClaudePolicy(t *testing.T) {
	for _, scenario := range []string{"restore", "remove-created", "preserve-user-edit"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			txn := filepath.Join(root, storeName, transactionName)
			if err := os.MkdirAll(txn, 0700); err != nil {
				t.Fatal(err)
			}
			before, desired := []byte("original repository instructions"), []byte("interrupted policy")
			if scenario == "remove-created" {
				before = nil
			}
			token := strings.Repeat("a", 64)
			j := transactionJournal{Format: nativeJournalFormat, Token: token, Images: []transactionImage{{
				Path: "CLAUDE.md", Before: before, Desired: desired,
				BeforeHash: imageHash(before), DesiredHash: imageHash(desired), Mode: 0600,
			}}}
			raw, err := json.Marshal(j)
			if err != nil {
				t.Fatal(err)
			}
			policyWrite(t, txn, "transaction.json", raw)
			current := desired
			if scenario == "preserve-user-edit" {
				current = []byte("subsequent user edit")
			}
			policyWrite(t, root, "CLAUDE.md", current)
			if err = Recover(root, strings.Repeat("b", 64)); err == nil {
				t.Fatal("wrong token accepted")
			}
			got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if err != nil || string(got) != string(current) {
				t.Fatal("wrong-token recovery changed instructions")
			}
			err = Recover(root, token)
			if scenario == "preserve-user-edit" {
				if err == nil || !strings.Contains(err.Error(), "recovery conflict") {
					t.Fatalf("expected conflict, got %v", err)
				}
				got, err = os.ReadFile(filepath.Join(root, "CLAUDE.md"))
				if err != nil || string(got) != string(current) {
					t.Fatal("user edit overwritten")
				}
				if _, err = os.Stat(txn); err != nil {
					t.Fatal("conflicted journal lost")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err = os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if scenario == "remove-created" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("created file not removed: %v", err)
				}
			} else if err != nil || string(got) != string(before) {
				t.Fatalf("preimage not restored: %v", err)
			}
			if err = Recover(root, token); err != nil {
				t.Fatalf("recovery not idempotent: %v", err)
			}
		})
	}
}
