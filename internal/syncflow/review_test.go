package syncflow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(s), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestCanonicalCompatibility(t *testing.T) {
	v := map[string]any{"á": "😀<&\n", "z": json.Number("1")}
	b, e := canonical(v)
	if e != nil {
		t.Fatal(e)
	}
	want := `{"z":1,"\u00e1":"\ud83d\ude00<&\n"}`
	if string(b) != want {
		t.Fatalf("%s != %s", b, want)
	}
}
func TestDuplicateJSONRejected(t *testing.T) {
	for _, s := range []string{`{"a":1,"a":2}`, `{"x":{"a":1,"a":2}}`, `{} {}`} {
		if _, e := decode([]byte(s)); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
func TestFreezeCheckDriftAndPublication(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "vault")
	c := filepath.Join(dir, "candidate")
	ev := filepath.Join(dir, "evidence")
	n := "20-Repos/Café.md"
	write(t, filepath.Join(v, n), "old\n")
	write(t, filepath.Join(c, n), "new\n<!-- connection:connection.api -->\n")
	write(t, filepath.Join(ev, "source.go"), "package test\n")
	mp := filepath.Join(dir, "manifest.json")
	rp := filepath.Join(dir, "review.json")
	var out bytes.Buffer
	args := []string{"review", "freeze", "--vault", v, "--candidate", c, "--evidence-root", ev, "--evidence", "source.go", "--output", mp}
	if e := Run(args, &out); e != nil {
		t.Fatal(e)
	}
	if e := Run(args, &out); e == nil {
		t.Fatal("overwrote immutable manifest")
	}
	value, e := readJSON(mp)
	if e != nil {
		t.Fatal(e)
	}
	m, e := manifest(value)
	if e != nil {
		t.Fatal(e)
	}
	d, _ := digest(m)
	review := map[string]any{"version": 1, "manifest_digest": d, "verdict": "accept", "findings": []any{}, "connection_decisions": map[string]any{n + "#connection.api": map[string]any{"action": "create", "reason": "source verifies this connection"}}}
	b, _ := json.Marshal(review)
	write(t, rp, string(b))
	args = []string{"check", "--vault", v, "--candidate", c, "--evidence-root", ev, "--manifest", mp, "--review", rp}
	if e := Run(args, &out); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(ev, "source.go"), "changed\n")
	if e := Run(args, &out); e == nil || !strings.Contains(e.Error(), "stale") {
		t.Fatalf("wanted drift, got %v", e)
	}
	write(t, filepath.Join(ev, "source.go"), "package test\n")
	b, _ = os.ReadFile(filepath.Join(c, n))
	write(t, filepath.Join(v, n), string(b))
	if e := Run([]string{"verify-published", "--vault", v, "--manifest", mp, "--review", rp}, &out); e != nil {
		t.Fatal(e)
	}
	if e := Run(args, &out); e == nil {
		t.Fatal("accepted changed base")
	}
}
func TestAbsentAndEmptyBasesDiffer(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "v")
	os.Mkdir(v, 0700)
	c := filepath.Join(dir, "c")
	ev := filepath.Join(dir, "e")
	write(t, filepath.Join(c, "20-Repos/n.md"), "new")
	write(t, filepath.Join(ev, "e"), "e")
	a, e := freeze(v, c, ev, []string{"e"}, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(v, "20-Repos/n.md"), "")
	b, e := freeze(v, c, ev, []string{"e"}, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	da, _ := digest(a)
	db, _ := digest(b)
	if da == db {
		t.Fatal("absence collapsed into empty base")
	}
}
func TestUnsafeAndDeletion(t *testing.T) {
	for _, p := range []string{"../a", "/a", "20-Repos/../a.md", "20-Repos/a\\b.md", "90-Meta/a.md"} {
		if e := relative(p, true); e == nil {
			t.Fatalf("accepted %s", p)
		}
	}
	dir := t.TempDir()
	v := filepath.Join(dir, "v")
	c := filepath.Join(dir, "c")
	ev := filepath.Join(dir, "e")
	os.Mkdir(c, 0700)
	write(t, filepath.Join(v, "20-Repos/a.md"), "old")
	write(t, filepath.Join(ev, "e"), "proof")
	m, e := freeze(v, c, ev, []string{"e"}, []string{"20-Repos/a.md"}, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = manifest(m); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(filepath.Join(ev, "e"), filepath.Join(c, "link.md")); e == nil {
		if _, e = freeze(v, c, ev, []string{"e"}, nil, ""); e == nil {
			t.Fatal("accepted symlink")
		}
	}
}
func TestMutationRequiresState(t *testing.T) {
	var b bytes.Buffer
	if e := Run([]string{"apply-unit"}, &b); e == nil {
		t.Fatal("mutation accepted without state")
	}
}

func TestPublishedLatestReviewedImageWins(t *testing.T) {
	dir := t.TempDir()
	v, c, ev := filepath.Join(dir, "vault"), filepath.Join(dir, "candidate"), filepath.Join(dir, "evidence")
	n := "20-Repos/n.md"
	write(t, filepath.Join(v, n), "old")
	write(t, filepath.Join(c, n), "one")
	write(t, filepath.Join(ev, "source"), "proof")
	pairs := []string{}
	for _, version := range []string{"one", "two"} {
		write(t, filepath.Join(c, n), version)
		m, e := freeze(v, c, ev, []string{"source"}, nil, "")
		if e != nil {
			t.Fatal(e)
		}
		md, _ := digest(m)
		r := map[string]any{"version": json.Number("1"), "manifest_digest": md, "verdict": "accept", "findings": []any{}, "connection_decisions": map[string]any{}}
		mp, rp := filepath.Join(dir, version+"-manifest.json"), filepath.Join(dir, version+"-review.json")
		saveJSON(t, mp, m)
		saveJSON(t, rp, r)
		pairs = append(pairs, "--reviewed", mp, rp)
		write(t, filepath.Join(v, n), version)
	}
	var out bytes.Buffer
	if e := Run(append([]string{"review", "verify-published", "--vault", v}, pairs...), &out); e != nil {
		t.Fatal(e)
	}
}
