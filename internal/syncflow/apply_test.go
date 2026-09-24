package syncflow

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func applyFixture(t *testing.T) (string, string, map[string]any, map[string]any) {
	t.Helper()
	root := t.TempDir()
	vault := t.TempDir()
	g := fixture(t, "gate.json")
	gd, _ := digest(g)
	patch := []byte("--- a/20-Repos/a.md\n+++ b/20-Repos/a.md\n@@ -1 +1 @@\n-old\n+new\n--- /dev/null\n+++ b/20-Repos/b.md\n@@ -0,0 +1 @@\n+created\n")
	p := map[string]any{"gate_digest": gd}
	pd, _ := digest(p)
	u := newUnit("unit-a", "acknowledgements", []any{"repo"}, []any{})
	u["status"] = "validated"
	u["gate_digest"] = gd
	u["projection_digest"] = pd
	u["patch_digest"] = hash(patch)
	u["base_files"] = map[string]any{"20-Repos/a.md": hash([]byte("old\n")), "20-Repos/b.md": hash(nil)}
	u["result_files"] = map[string]any{"20-Repos/a.md": hash([]byte("new\n")), "20-Repos/b.md": hash([]byte("created\n"))}
	u["base_kinds"] = map[string]any{"20-Repos/a.md": "file", "20-Repos/b.md": "missing"}
	u["result_kinds"] = map[string]any{"20-Repos/a.md": "file", "20-Repos/b.md": "file"}
	r := map[string]any{"version": json.Number("1"), "run_id": "run-test", "units": []any{u}, "gate_digest": gd, "gate_stale": false, "vault_locator": "", "status": "projecting", "fingerprint": strings.Repeat("a", 64), "tool_digest": strings.Repeat("b", 64), "inventory_digest": strings.Repeat("c", 64), "packages": []any{map[string]any{"repository": "repo", "oid": strings.Repeat("d", 40), "status": "checkpointed", "artifact_digest": "", "checkpoint_count": json.Number("1")}}}
	for _, v := range []struct {
		parts []string
		m     map[string]any
	}{{[]string{"active", "run-test", "gates", gd + ".json"}, g}, {[]string{"active", "run-test", "units", "unit-a", "projection.json"}, p}} {
		if e := atomicState(root, v.parts, v.m); e != nil {
			t.Fatal(e)
		}
	}
	write(t, filepath.Join(root, "active/run-test/units/unit-a/unit.patch"), string(patch))
	write(t, filepath.Join(vault, "20-Repos/a.md"), "old\n")
	if e := saveRun(root, r); e != nil {
		t.Fatal(e)
	}
	return root, vault, r, u
}
func TestApplyRollbackAndReplay(t *testing.T) {
	root, vault, r, u := applyFixture(t)
	if _, e := applyUnit(root, r, "unit-a", vault, 1, 0, false); e == nil || !strings.Contains(e.Error(), "simulated-write-failure") {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(vault, "20-Repos/a.md"))
	if string(b) != "old\n" {
		t.Fatal("failed rollback")
	}
	if _, e := os.Stat(filepath.Join(vault, "20-Repos/b.md")); !os.IsNotExist(e) {
		t.Fatal("addition leaked")
	}
	out, e := applyUnit(root, r, "unit-a", vault, 0, 0, false)
	if e != nil || out["reused"] != false || u["status"] != "applied" {
		t.Fatal(out, e)
	}
	out, e = applyUnit(root, r, "unit-a", vault, 0, 0, false)
	if e != nil || out["reused"] != true {
		t.Fatal(out, e)
	}
	write(t, filepath.Join(vault, "20-Repos/a.md"), "edited\n")
	if _, e = applyUnit(root, r, "unit-a", vault, 0, 0, false); e == nil {
		t.Fatal("ignored drift")
	}
}
func TestResumeAllBytesAndRejectMixed(t *testing.T) {
	root, vault, r, u := applyFixture(t)
	if _, e := applyUnit(root, r, "unit-a", vault, 0, 0, true); e == nil {
		t.Fatal("expected simulated crash")
	}
	out, e := resumeRun(root, r, nil, nil)
	if e != nil || u["status"] != "applied" || len(arr(out["reconciled_units"])) != 1 {
		t.Fatal(out, e)
	}
	root, vault, r, u = applyFixture(t)
	loc, _ := filepath.Rel(root, vault)
	r["vault_locator"] = loc
	write(t, filepath.Join(vault, "20-Repos/a.md"), "new\n")
	if _, e = resumeRun(root, r, nil, nil); e == nil || !strings.Contains(e.Error(), "atomic-unit-interrupted") {
		t.Fatal(e)
	}
	if u["status"] != "apply-failed" {
		t.Fatal(u)
	}
}
func TestStaleSourceRetractsAndCloseChecksBytes(t *testing.T) {
	root, vault, r, u := applyFixture(t)
	if _, e := applyUnit(root, r, "unit-a", vault, 0, 0, false); e != nil {
		t.Fatal(e)
	}
	out, e := resumeRun(root, r, map[string]string{"repo": strings.Repeat("e", 40)}, nil)
	if e != nil || out["code"] != "source-stale" || u["stale_reason"] != "source-retracted" {
		t.Fatal(out, e)
	}
	b, _ := os.ReadFile(filepath.Join(vault, "20-Repos/a.md"))
	if string(b) != "old\n" {
		t.Fatal("did not retract")
	}
	if _, e = os.Stat(filepath.Join(vault, "20-Repos/b.md")); !os.IsNotExist(e) {
		t.Fatal("did not retract addition")
	}
	root, vault, r, _ = applyFixture(t)
	if _, e = applyUnit(root, r, "unit-a", vault, 0, 0, false); e != nil {
		t.Fatal(e)
	}
	out, e = closeRun(root, r)
	if e != nil || out["code"] != "run-closed" {
		t.Fatal(out, e)
	}
	if _, e = os.Stat(filepath.Join(root, "active/run-test")); !os.IsNotExist(e) {
		t.Fatal("active retained")
	}
}
func TestApplyRejectsMissingReviewAndSymlink(t *testing.T) {
	root, vault, r, u := applyFixture(t)
	u["unit_type"] = "write-group"
	if _, e := applyUnit(root, r, "unit-a", vault, 0, 0, false); e == nil || !strings.Contains(e.Error(), "final-note-review-required") {
		t.Fatal(e)
	}
	u["unit_type"] = "acknowledgements"
	target := filepath.Join(vault, "20-Repos/a.md")
	if e := os.Remove(target); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join(t.TempDir(), "other"), target); e != nil {
		t.Skip(e)
	}
	if _, e := applyUnit(root, r, "unit-a", vault, 0, 0, false); e == nil {
		t.Fatal("accepted symlink")
	}
}

func TestApplyProcessCrashJournal(t *testing.T) {
	if root := os.Getenv("VAULT_SYNC_CRASH_ROOT"); root != "" {
		r, e := readState(root, "active", "run-test", "run.json")
		if e != nil {
			t.Fatal(e)
		}
		n, _ := strconv.Atoi(os.Getenv("VAULT_SYNC_CRASH_COUNT"))
		_, e = applyUnit(root, r, "unit-a", os.Getenv("VAULT_SYNC_CRASH_VAULT"), 0, n, false)
		t.Fatalf("crash did not exit: %v", e)
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range []int{1, 2} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			root, vault, _, _ := applyFixture(t)
			cmd := exec.Command(exe, "-test.run=^TestApplyProcessCrashJournal$")
			cmd.Env = append(os.Environ(), "VAULT_SYNC_CRASH_ROOT="+root, "VAULT_SYNC_CRASH_VAULT="+vault, "VAULT_SYNC_CRASH_COUNT="+strconv.Itoa(n))
			err := cmd.Run()
			ee, ok := err.(*exec.ExitError)
			if !ok || ee.ExitCode() != 99 {
				t.Fatalf("expected abrupt exit 99: %v", err)
			}
			j, e := readState(root, "active", "run-test", "units", "unit-a", "apply-journal.json")
			if e != nil || j["status"] != "applying" {
				t.Fatal(j, e)
			}
			r, e := readState(root, "active", "run-test", "run.json")
			if e != nil {
				t.Fatal(e)
			}
			out, e := resumeRun(root, r, nil, nil)
			if n == 1 {
				if e == nil || !strings.Contains(e.Error(), "atomic-unit-interrupted") {
					t.Fatal(out, e)
				}
			} else if e != nil || r["status"] != "closing" {
				t.Fatal(out, e)
			}
		})
	}
}
