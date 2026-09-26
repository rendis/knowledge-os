package check

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("KOS_TEST_OBSIDIAN_HELPER") == "1" {
		if len(os.Args) != 4 || os.Args[1] != "vault=Test vault" || os.Args[2] != "vault" || os.Args[3] != "info=path" {
			os.Exit(23)
		}
		if os.Getenv("KOS_TEST_OBSIDIAN_SLEEP") == "1" {
			time.Sleep(time.Minute)
		}
		fmt.Print(os.Getenv("KOS_TEST_OBSIDIAN_OUTPUT"))
		if os.Getenv("KOS_TEST_OBSIDIAN_FAIL") == "1" {
			os.Exit(4)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func mockObsidian(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	name := "obsidian"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	src, e := os.Open(exe)
	if e != nil {
		t.Fatal(e)
	}
	defer src.Close()
	dst, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY, 0700)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = io.Copy(dst, src); e != nil {
		t.Fatal(e)
	}
	if e = dst.Close(); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir)
	t.Setenv("KOS_TEST_OBSIDIAN_HELPER", "1")
}
func TestObsidianBindingCLI(t *testing.T) {
	mockObsidian(t)
	root := t.TempDir()
	t.Setenv("KOS_TEST_OBSIDIAN_OUTPUT", "startup log\n"+root+"\n")
	r, e := Binding(context.Background(), root, " Test vault ")
	if e != nil || !r.Pass {
		t.Fatalf("%+v %v", r, e)
	}
	for _, output := range []string{"", "relative/path", t.TempDir(), strings.Repeat("x", 70000)} {
		t.Setenv("KOS_TEST_OBSIDIAN_OUTPUT", output)
		if _, e := Binding(context.Background(), root, "Test vault"); e == nil {
			t.Errorf("accepted invalid output")
		}
	}
	t.Setenv("KOS_TEST_OBSIDIAN_OUTPUT", root)
	t.Setenv("KOS_TEST_OBSIDIAN_FAIL", "1")
	if _, e := Binding(context.Background(), root, "Test vault"); e == nil {
		t.Fatal("accepted failed command")
	}
}
func TestObsidianBindingDeadlineAndPreflight(t *testing.T) {
	mockObsidian(t)
	root := t.TempDir()
	t.Setenv("KOS_TEST_OBSIDIAN_SLEEP", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, e := Binding(ctx, root, "Test vault"); e == nil {
		t.Fatal("accepted timeout")
	}
	for _, v := range [][2]string{{"relative", "Test vault"}, {root, " "}, {filepath.Join(root, "missing"), "Test vault"}} {
		if _, e := Binding(context.Background(), v[0], v[1]); e == nil {
			t.Fatal("accepted invalid input")
		}
	}
	t.Setenv("PATH", t.TempDir())
	if _, e := Binding(context.Background(), root, "Test vault"); e == nil {
		t.Fatal("missing binary accepted")
	}
}
