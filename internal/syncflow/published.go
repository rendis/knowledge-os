package syncflow

import (
	"encoding/json"
	"flag"
	"io"
	"strings"
)

func runPublished(args []string, out io.Writer) error {
	f := flag.NewFlagSet("verify-published", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	vault := f.String("vault", "", "")
	mp := f.String("manifest", "", "")
	rp := f.String("review", "", "")
	var pairs repeated
	f.Var(&pairs, "reviewed", "MANIFEST REVIEW")
	normalized := []string{}
	for i := 1; i < len(args); i++ {
		if args[i] == "--reviewed" {
			if i+2 >= len(args) {
				return fail("reviewed-pair-required")
			}
			normalized = append(normalized, "--reviewed", args[i+1]+"\x00"+args[i+2])
			i += 2
		} else {
			normalized = append(normalized, args[i])
		}
	}
	if e := f.Parse(normalized); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return fail("unexpected-arguments")
	}
	if *mp != "" || *rp != "" {
		if len(pairs) > 0 || *mp == "" || *rp == "" {
			return fail("reviewed-pair-required")
		}
		pairs = append(pairs, *mp+"\x00"+*rp)
	}
	if len(pairs) == 0 {
		return fail("reviewed-pair-required")
	}
	root, e := safe(*vault, true)
	if e != nil {
		return e
	}
	expected := map[string]string{}
	deleted := map[string]bool{}
	for _, pair := range pairs {
		mPath, rPath, ok := strings.Cut(pair, "\x00")
		if !ok {
			return fail("reviewed-pair-required")
		}
		v, e := readJSON(mPath)
		if e != nil {
			return e
		}
		m, e := manifest(v)
		if e != nil {
			return e
		}
		r, e := readJSON(rPath)
		if e != nil {
			return e
		}
		if e = review(r, m); e != nil {
			return e
		}
		for n, h := range obj(m["candidate_files"]) {
			expected[n] = str(h)
			deleted[n] = contains(m["deleted_files"], n)
		}
	}
	bad := []any{}
	for _, v := range sortedKeys(expected) {
		n := str(v)
		b, exists, e := content(root, n, true)
		if e != nil {
			return e
		}
		if deleted[n] && exists || !deleted[n] && (!exists || hash(b) != expected[n]) {
			bad = append(bad, n)
		}
	}
	status, code := "pass", "published-notes-reviewed"
	if len(bad) > 0 {
		status = "blocked"
		code = "published-note-drift"
	}
	if e = json.NewEncoder(out).Encode(map[string]any{"status": status, "code": code, "checked_files": len(expected), "mismatches": bad}); e != nil {
		return e
	}
	if len(bad) > 0 {
		return fail(code)
	}
	return nil
}
