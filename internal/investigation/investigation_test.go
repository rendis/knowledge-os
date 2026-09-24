package investigation

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSnapshotPythonCompatibility(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode fixture; Windows permissions differ")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	for p, data := range map[string]string{"a/ñ😀<&>.md": "hello\n", "a.md": ""} {
		f := filepath.Join(root, p)
		if err := os.WriteFile(f, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(f, 0644); err != nil {
			t.Fatal(err)
		}
	}
	digest, entries, err := Snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	// Generated with Python json.dumps(sort_keys=True,separators=(",",":")),
	// hashlib.sha256 and the same bytes/permissions as the existing lifecycle tool.
	if digest != "bebd880ae17aa5692e8d05d83dc498b7afa2ea223c23d37ba585112f186b2e36" {
		t.Fatalf("incompatible snapshot %s: %#v", digest, entries)
	}
	if len(entries) != 3 || entries[1].Path != "a/ñ😀<&>.md" {
		t.Fatalf("wrong path ordering: %#v", entries)
	}
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	changed, _, err := Snapshot(root)
	if err != nil || changed == digest {
		t.Fatalf("change undetected: %v", err)
	}
}

func TestSnapshotEmptyAndSymlinks(t *testing.T) {
	root := t.TempDir()
	digest, entries, err := Snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if digest != "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945" || entries == nil {
		t.Fatalf("empty hash mismatch: %s", digest)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := Snapshot(root); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, _, err := Snapshot(filepath.Join(root, "link")); err == nil {
		t.Fatal("symlink root accepted")
	}
}

func TestUnknownCommandsFailClosed(t *testing.T) {
	for _, verb := range []string{"not-a-command"} {
		var out bytes.Buffer
		err := Run([]string{verb}, &out)
		if !errors.Is(err, ErrUnknownCommand) || out.Len() != 0 {
			t.Fatalf("%s: %v %s", verb, err, &out)
		}
	}
}

func TestRunSnapshot(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"snapshot", "--case-dir", t.TempDir()}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"status":"snapshotted"`)) {
		t.Fatal(out.String())
	}
}
