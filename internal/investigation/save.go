package investigation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

func optionalDigest(b []byte) string {
	if b == nil {
		return "absent"
	}
	return digest(b)
}
func saveCase(root string, o options) (any, error) {
	if e := o.required("id", "public-candidate", "expected-public-sha256", "private-root", "source"); e != nil {
		return nil, e
	}
	if e := eventInput(o.get("source")); e != nil {
		return nil, e
	}
	if o.get("private-candidate") != "" && o.get("delete-private") == "true" {
		return nil, errors.New("private-candidate and delete-private are mutually exclusive")
	}
	public, e := readText(o.get("public-candidate"))
	if e != nil {
		return nil, e
	}
	if secretPattern.MatchString(public) || localPattern.MatchString(public) {
		return nil, errors.New("public candidate contains credential or absolute local path")
	}
	var private []byte
	if p := o.get("private-candidate"); p != "" {
		s, e := readText(p)
		if e != nil {
			return nil, e
		}
		if e = privateSafe(s, o.get("id")); e != nil {
			return nil, e
		}
		private = []byte(s)
	}
	pr, e := filepath.Abs(o.get("private-root"))
	if e != nil {
		return nil, e
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(pr))
	if e != nil {
		return nil, e
	}
	if filepath.Base(pr) != ".investigations-private" || parent != filepath.Dir(root) {
		return nil, errors.New("private root must share the vault root")
	}
	pr = filepath.Join(parent, ".investigations-private")
	if e = noSymlink(pr, true); e != nil {
		return nil, e
	}
	for _, v := range append(append([]string{}, o["target"]...), o["private-target"]...) {
		if !registerID.MatchString(v) {
			return nil, errors.New("save targets must be register IDs")
		}
	}
	who, e := identity(root)
	if e != nil {
		return nil, e
	}
	ts, e := eventTime(o)
	if e != nil {
		return nil, e
	}
	o.defaultValue("expected-private-sha256", "absent")
	return mutationGate(root, func() (any, error) {
		r, e := locate(root, o.get("id"))
		if e != nil {
			return nil, e
		}
		if digest([]byte(r.text)) != o.get("expected-public-sha256") {
			return nil, errors.New("stale public snapshot")
		}
		privatePath := filepath.Join(pr, o.get("id"), "private.md")
		if e = noSymlink(filepath.Dir(privatePath), true); e != nil {
			return nil, e
		}
		before, e := currentBytes(privatePath)
		if e != nil {
			return nil, e
		}
		if optionalDigest(before) != o.get("expected-private-sha256") {
			return nil, errors.New("stale private snapshot")
		}
		publicChanged := public != r.text
		deletePrivate := o.get("delete-private") == "true"
		privateChanged := (private != nil && !sameBytes(private, before)) || (deletePrivate && before != nil)
		if !publicChanged && !privateChanged {
			return map[string]any{"status": "unchanged", "id": o.get("id"), "public_sha256": digest([]byte(r.text)), "private_sha256": optionalDigest(before)}, nil
		}
		if publicChanged && len(o["target"]) == 0 {
			return nil, errors.New("material public write requires affected register ID")
		}
		if privateChanged && len(o["private-target"]) == 0 {
			return nil, errors.New("material private write requires affected register ID")
		}
		f, e := frontmatter(public)
		if e != nil {
			return nil, e
		}
		if f["id"] != o.get("id") {
			return nil, errors.New("candidate ID does not match")
		}
		for _, k := range []string{"artifact-integrity", "status", "blocked-on", "closure-outcome", "resume-to"} {
			if !reflect.DeepEqual(f[k], r.fields[k]) {
				return nil, fmt.Errorf("%s cannot change through save", k)
			}
		}
		exports, e := exportIDs(filepath.Dir(r.path), o.get("id"))
		if e != nil {
			return nil, e
		}
		for _, v := range append(append([]string{}, o["target"]...), o["private-target"]...) {
			if !declares(public, v) && !contains(exports, v) {
				return nil, errors.New("save target absent from public candidate")
			}
		}
		if publicChanged || (deletePrivate && before != nil) {
			public, e = setField(public, "updated-at", ts)
			if e != nil {
				return nil, e
			}
			targets := o["target"]
			verb := "Updated registers"
			es := isSpanish(public)
			if es {
				verb = "Registros actualizados"
			}
			if deletePrivate {
				targets = o["private-target"]
				verb = "Removed private overlay linked to"
				if es {
					verb = "Complemento privado eliminado para"
				}
			}
			refs := []string{}
			for _, v := range targets {
				refs = append(refs, "`"+v+"`")
			}
			public, e = appendHistory(public, attributed(ts, verb+" "+strings.Join(refs, ", "), who, o.get("source"), es))
			if e != nil {
				return nil, e
			}
		}
		if private != nil && !sameBytes(private, before) {
			s, e := setField(string(private), "updated-at", ts)
			if e != nil {
				return nil, e
			}
			es := isSpanish(s)
			verb := "Updated private context for "
			if es {
				verb = "Contexto privado actualizado para "
			}
			refs := []string{}
			for _, v := range o["private-target"] {
				refs = append(refs, "`"+v+"`")
			}
			s, e = appendHistory(s, attributed(ts, verb+strings.Join(refs, ", "), who, o.get("source"), es))
			if e != nil {
				return nil, e
			}
			private = []byte(s)
		}
		candidate := r
		candidate.text = public
		candidate.fields, _ = frontmatter(public)
		all, e := validateVault(root)
		if e != nil {
			return nil, e
		}
		prefix := filepath.Base(filepath.Dir(r.path)) + "/investigation.md: "
		errs := []string{}
		for _, v := range all {
			if !strings.HasPrefix(v, prefix) {
				errs = append(errs, v)
			}
		}
		errs = append(errs, validateRecord(candidate)...)
		rs, _, e := records(root)
		if e != nil {
			return nil, e
		}
		for _, other := range rs {
			if other.path != r.path && other.get("dedupe-key") == candidate.get("dedupe-key") {
				errs = append(errs, "duplicate dedupe-key")
			}
		}

		candidateIDs := map[string]bool{}
		candidateLineage := map[string]bool{}
		for _, other := range rs {
			if other.path == r.path {
				other = candidate
			}
			candidateIDs[other.get("id")] = true
		}
		for _, other := range rs {
			if other.path == r.path {
				other = candidate
			}
			for _, id := range other.lineage() {
				if candidateIDs[id] || candidateLineage[id] {
					errs = append(errs, "conflicting consolidated-from identity")
				}
				candidateLineage[id] = true
			}
		}
		if len(errs) > 0 {
			return nil, fmt.Errorf("save validation failed: %s", strings.Join(errs, "; "))
		}
		rel, e := filepath.Rel(filepath.Dir(root), r.path)
		if e != nil {
			return nil, e
		}
		changes := []fileChange{{filepath.ToSlash(rel), []byte(r.text), []byte(public)}}
		if privateChanged {
			after := private
			if !deletePrivate && private == nil {
				after = before
			}
			rel, e = filepath.Rel(filepath.Dir(root), privatePath)
			if e != nil {
				return nil, e
			}
			changes = append(changes, fileChange{filepath.ToSlash(rel), before, after})
		}
		if e = commitChanges(root, changes); e != nil {
			return nil, e
		}
		if deletePrivate {
			_ = os.Remove(filepath.Dir(privatePath))
		}
		return map[string]any{"status": "saved", "id": o.get("id"), "public_sha256": digest([]byte(public)), "private": private != nil, "private_deleted": deletePrivate, "warnings": []string{}}, nil
	})
}
