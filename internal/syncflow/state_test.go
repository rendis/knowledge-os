package syncflow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

func call(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var b bytes.Buffer
	if e := Run(args, &b); e != nil {
		t.Fatal(e)
	}
	v, e := decode(b.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	return obj(v)
}
func fixture(t *testing.T, p string) map[string]any {
	t.Helper()
	v, e := readJSON(filepath.Join("testdata", p))
	if e != nil {
		t.Fatal(e)
	}
	return obj(v)
}
func saveJSON(t *testing.T, p string, v any) {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	write(t, p, string(b))
}
func TestNativeSyncBeginCheckpointSealStatus(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	args := []string{"begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", "APP00000-source-adapter", strings.Repeat("a", 40), "--package", "APP00000-target-service", strings.Repeat("b", 40)}
	start := call(t, args...)
	id := str(start["run_id"])
	if start["next_command"] != "checkpoint-package" || start["reused"] != false {
		t.Fatal(start)
	}
	if call(t, args...)["reused"] != true {
		t.Fatal("begin not idempotent")
	}
	for _, name := range []string{"APP00000-source-adapter", "APP00000-target-service"} {
		a := filepath.Join("testdata", name+".json")
		out := call(t, "checkpoint-package", "--state-root", root, "--run-id", id, "--repository", name, "--artifact", a)
		if out["status"] != "pass" {
			t.Fatal(out)
		}
		if call(t, "checkpoint-package", "--state-root", root, "--run-id", id, "--repository", name, "--artifact", a)["reused"] != true {
			t.Fatal("checkpoint not idempotent")
		}
	}
	status := call(t, "status", "--state-root", root, "--run-id", id)
	if status["next_command"] != "seal-gate" {
		t.Fatal(status)
	}
	sealed := call(t, "seal-gate", "--state-root", root, "--run-id", id, "--gate", "testdata/gate.json")
	if sealed["code"] != "gate-sealed" || sealed["next_command"] != "validate-unit" {
		t.Fatal(sealed)
	}
	replay := call(t, "seal-gate", "--state-root", root, "--run-id", id, "--gate", "testdata/gate.json")
	if replay["reused"] != true {
		t.Fatal(replay)
	}
	if call(t, "status", "--state-root", root, "--run-id", id)["receipt_digest"] != sealed["receipt_digest"] {
		t.Fatal("status unstable")
	}
	p := filepath.Join(root, "active", id, "packages", "APP00000-source-adapter", "artifact.json")
	write(t, p, "{}")
	var out bytes.Buffer
	if e := Run([]string{"status", "--state-root", root, "--run-id", id}, &out); e == nil {
		t.Fatal("accepted tampered checkpoint")
	}
}
func TestPackageLineageAndGateAuthority(t *testing.T) {
	p := fixture(t, "APP00000-source-adapter.json")
	if e := validatePackage(p); e != nil {
		t.Fatal(e)
	}
	obj(p["analysis"])["claims"] = []any{}
	d, _ := digest(p["analysis"])
	obj(p["validation"])["analysis_digest"] = d
	obj(p["review"])["analysis_digest"] = d
	d, _ = digest(p["review"])
	obj(p["validation"])["review_digest"] = d
	if e := validatePackage(p); e == nil {
		t.Fatal("accepted claims not bound to analysis")
	}
	g := fixture(t, "gate.json")
	gr := obj(arr(g["write_groups"])[0])
	obj(arr(gr["grants"])[0])["claim_id"] = "unreviewed"
	if e := validateGate(g); e == nil {
		t.Fatal("accepted unreviewed grant")
	}
}
func TestSyncLockAndInvalidState(t *testing.T) {
	root := t.TempDir()
	l := flock.New(filepath.Join(root, ".native.lock"))
	if e := l.Lock(); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	e := Run([]string{"begin", "--state-root", root, "--inventory-digest", strings.Repeat("a", 64), "--package", "repo=" + strings.Repeat("b", 40)}, &b)
	l.Unlock()
	if e == nil || e.Error() != "sync-state-busy" {
		t.Fatalf("lock: %v", e)
	}
	out := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("a", 64), "--package", "repo="+strings.Repeat("b", 40))
	id := str(out["run_id"])
	p := filepath.Join(root, "active", id, "run.json")
	v, e := readJSON(p)
	if e != nil {
		t.Fatal(e)
	}
	obj(v)["inventory_digest"] = strings.Repeat("c", 64)
	saveJSON(t, p, v)
	if e = Run([]string{"status", "--state-root", root, "--run-id", id}, &b); e == nil {
		t.Fatal("accepted fingerprint drift")
	}
}
func TestStateSymlinkRefused(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()
	if e := os.Symlink(elsewhere, filepath.Join(root, "active")); e != nil {
		t.Skip(e)
	}
	var b bytes.Buffer
	if e := Run([]string{"begin", "--state-root", root, "--inventory-digest", strings.Repeat("a", 64), "--package", "repo=" + strings.Repeat("b", 40)}, &b); e == nil {
		t.Fatal("followed state symlink")
	}
	entries, e := os.ReadDir(elsewhere)
	if e != nil || len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
}

func TestPublicDispatchReturnsMutationErrors(t *testing.T) {
	root := t.TempDir()
	start := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("a", 64), "--package", "repo="+strings.Repeat("b", 40))
	id := str(start["run_id"])
	for _, cmd := range []string{"validate-unit", "review-unit", "apply-unit", "close"} {
		var b bytes.Buffer
		e := Run([]string{cmd, "--state-root", root, "--run-id", id}, &b)
		if e == nil {
			t.Fatalf("%s swallowed mutation error: %s", cmd, b.String())
		}
	}
}

func TestPermissionIsNotCredentialButNestedAssignmentIs(t *testing.T) {
	for _, s := range []string{"id-token: write", "permissions: {id-token: none}", "id-token:\tread"} {
		if sensitive(s) {
			t.Fatalf("permission falsely classified: %s", s)
		}
	}
	for _, s := range []string{"id-token: write,token: secretvalue", "xid-token: write", "id-token: writer", "token: secretvalue"} {
		if !sensitive(s) {
			t.Fatalf("credential assignment missed: %s", s)
		}
	}
}

func TestMissingCheckpointMarksSourceStale(t *testing.T) {
	root := t.TempDir()
	name := "APP00000-source-adapter"
	start := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", name, strings.Repeat("a", 40))
	id := str(start["run_id"])
	call(t, "checkpoint-package", "--state-root", root, "--run-id", id, "--repository", name, "--artifact", "testdata/"+name+".json")
	p := fixture(t, name+".json")
	gp := filepath.Join(t.TempDir(), "gate.json")
	saveJSON(t, gp, p["gate"])
	call(t, "seal-gate", "--state-root", root, "--run-id", id, "--gate", gp)
	if e := os.Remove(filepath.Join(root, "active", id, "gate.json")); e != nil {
		t.Fatal(e)
	}
	call(t, "status", "--state-root", root, "--run-id", id)
	if _, e := os.Stat(filepath.Join(root, "active", id, "gate.json")); e != nil {
		t.Fatal("gate history was not restored", e)
	}
	if e := os.Remove(filepath.Join(root, "active", id, "packages", name, "artifact.json")); e != nil {
		t.Fatal(e)
	}
	result := call(t, "status", "--state-root", root, "--run-id", id)
	if result["next_command"] != "checkpoint-package" || obj(arr(result["units"])[0])["stale_reason"] != "source" {
		t.Fatal(result)
	}
}
