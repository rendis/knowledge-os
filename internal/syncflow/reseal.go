package syncflow

import (
	"os"
	"reflect"
)

func authority(g, u map[string]any) any {
	if u["unit_type"] == "write-group" {
		for _, v := range arr(g["write_groups"]) {
			gr := obj(v)
			if gr["group_id"] == u["unit_id"] {
				c := clone(gr)
				delete(c, "group_id")
				return c
			}
		}
		return nil
	}
	a := []any{}
	for _, v := range arr(g["acknowledgements"]) {
		if contains(u["repositories"], obj(v)["repository"]) {
			a = append(a, v)
		}
	}
	return a
}
func sameAuthority(a, b, g, h map[string]any) bool {
	return a["unit_type"] == b["unit_type"] && reflect.DeepEqual(a["repositories"], b["repositories"]) && reflect.DeepEqual(a["nodes"], b["nodes"]) && reflect.DeepEqual(authority(g, a), authority(h, b))
}
func rebind(root, id string, previous, fresh map[string]any, gd string) (map[string]any, error) {
	old := []string{"active", id, "units", str(previous["unit_id"])}
	projection, e := readDigest(root, previous["projection_digest"], append(old, "projection.json")...)
	if e != nil {
		return nil, e
	}
	p, e := statePath(root, append(old, "unit.patch")...)
	if e != nil {
		return nil, e
	}
	patch, e := os.ReadFile(p)
	if e != nil || hash(patch) != previous["patch_digest"] {
		return nil, fail("unit-checkpoint-invalid")
	}
	b, e := canonical(previous)
	if e != nil {
		return nil, e
	}
	v, e := decode(b)
	if e != nil {
		return nil, e
	}
	u := obj(v)
	u["unit_id"] = fresh["unit_id"]
	u["gate_digest"] = gd
	u["receipt_digest"] = ""
	projection["unit_id"] = fresh["unit_id"]
	projection["gate_digest"] = gd
	pd, e := digest(projection)
	if e != nil {
		return nil, e
	}
	u["projection_digest"] = pd
	target := []string{"active", id, "units", str(fresh["unit_id"])}
	if e = atomicState(root, append(target, "projection.json"), projection); e != nil {
		return nil, e
	}
	if e = atomicStateBytes(root, append(target, "unit.patch"), patch); e != nil {
		return nil, e
	}
	if previous["note_review_digest"] != "" {
		r, e := readDigest(root, previous["note_review_digest"], append(old, "note-review.json")...)
		if e != nil {
			return nil, e
		}
		r["unit_id"] = fresh["unit_id"]
		r["gate_digest"] = gd
		r["projection_digest"] = pd
		d, _ := digest(r)
		u["note_review_digest"] = d
		if e = atomicState(root, append(target, "note-review.json"), r); e != nil {
			return nil, e
		}
	} else {
		u["note_review_digest"] = ""
	}
	return u, nil
}
