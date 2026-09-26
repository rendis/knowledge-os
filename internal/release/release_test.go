package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func server(t *testing.T, version string, asset []byte, sum string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/VERSION", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(version + "\n")) })
	mux.HandleFunc("/"+Asset(), func(w http.ResponseWriter, _ *http.Request) { w.Write(asset) })
	mux.HandleFunc("/SHA256SUMS", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(sum + "  " + Asset() + "\n")) })
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	t.Setenv("KOS_DOWNLOAD_URL", s.URL)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("CI", "")
	t.Setenv("KOS_NO_UPDATE_CHECK", "")
}

func TestUpdateVerifiesAndReplacesInPlace(t *testing.T) {
	next := []byte("#!/bin/sh\necho next\n")
	sum := sha256.Sum256(next)
	server(t, "0.16.0", next, hex.EncodeToString(sum[:]))
	self := filepath.Join(t.TempDir(), "kos")
	os.WriteFile(self, []byte("old"), 0o755)
	res, e := Update(context.Background(), "0.15.0", "", self)
	if e != nil || res["to"] != "0.16.0" {
		t.Fatalf("%v %v", e, res)
	}
	if b, _ := os.ReadFile(self); string(b) != string(next) {
		t.Fatalf("replaced in place: %q", b)
	}
	if st, _ := os.Stat(self); st.Mode().Perm()&0o100 == 0 {
		t.Fatal("the new kos is executable")
	}
	if res, _ := Update(context.Background(), "0.16.0", "", self); res["status"] != "current" {
		t.Fatalf("nothing to do when current: %v", res)
	}
}

func TestUpdateRefusesAChecksumMismatch(t *testing.T) {
	server(t, "0.16.0", []byte("tampered"), "00")
	self := filepath.Join(t.TempDir(), "kos")
	os.WriteFile(self, []byte("old"), 0o755)
	if _, e := Update(context.Background(), "0.15.0", "", self); e == nil {
		t.Fatal("a mismatched download is refused")
	}
	if b, _ := os.ReadFile(self); string(b) != "old" {
		t.Fatal("the installed kos is untouched")
	}
}

func TestNewerIsCachedAndSilentWhenCurrent(t *testing.T) {
	server(t, "0.16.0", nil, "")
	if v := Newer("0.15.0"); v != "0.16.0" {
		t.Fatalf("newer release announced: %q", v)
	}
	os.Setenv("KOS_DOWNLOAD_URL", "http://127.0.0.1:1") // unreachable: the cached answer is used
	if v := Newer("0.15.0"); v != "0.16.0" {
		t.Fatalf("answered from the daily cache: %q", v)
	}
	if v := Newer("0.16.0"); v != "" {
		t.Fatalf("silent when current: %q", v)
	}
	t.Setenv("CI", "true")
	if v := Newer("0.1.0"); v != "" {
		t.Fatal("no check in CI")
	}
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{{"0.15.0", "0.15.0", 0}, {"0.15.1", "0.15.0", 1}, {"v0.9.9", "0.10.0", -1}, {"1.0", "0.99.99", 1}} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("%s vs %s = %d", c.a, c.b, got)
		}
	}
}
