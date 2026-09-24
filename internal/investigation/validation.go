package investigation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var allowedValues = map[string][]string{
	"purpose":          {"knowledge", "development", "mixed", "undecided"},
	"vault-outcome":    {"not-evaluated", "none", "deferred-until-production", "candidate-for-audit", "documented"},
	"learning-outcome": {"not-evaluated", "no-learning", "already-covered", "insufficient-evidence", "candidate", "documented"},
	"status":           {"investigating", "blocked", "closed"},
}

func validateRecord(r record) []string {
	out := []string{}
	add := func(s string) { out = append(out, filepath.Base(filepath.Dir(r.path))+"/investigation.md: "+s) }
	for _, k := range requiredFields {
		v := r.fields[k]
		empty := v == nil || v == ""
		if a, ok := v.([]string); ok {
			empty = len(a) == 0
		}
		if empty {
			add("missing " + k)
		}
	}
	for _, k := range []string{"created-at", "updated-at"} {
		if v := r.get(k); v != "" && !timestamp(v) {
			add(k + " must be an ISO-8601 timestamp with offset")
		}
	}
	if e := historyDates(r.text); e != nil {
		add(e.Error())
	}
	if filepath.Base(filepath.Dir(r.path)) != r.get("id") {
		add("directory does not match id")
	}
	if !idPattern.MatchString(r.get("id")) {
		add("invalid id")
	}
	if k := r.get("dedupe-key"); k != "" && !keyPattern.MatchString(k) {
		add("invalid dedupe-key")
	}
	for k, values := range allowedValues {
		if s := r.get(k); s != "" && !contains(values, s) {
			add("invalid " + k)
		}
	}
	status, blocked, closure := r.get("status"), r.get("blocked-on"), r.get("closure-outcome")
	if _, ok := r.fields["resume-to"]; ok {
		add("resume-to is legacy lifecycle metadata")
	}
	if status == "blocked" && blocked == "" {
		add("blocked status requires blocked-on")
	}
	if status != "blocked" && blocked != "" {
		add("blocked-on is allowed only while blocked")
	}
	if status == "closed" && !contains([]string{"completed", "abandoned"}, closure) {
		add("closed status requires a valid closure-outcome")
	}
	if status != "closed" && closure != "" {
		add("closure-outcome is allowed only while closed")
	}
	if status == "closed" && closure == "completed" {
		if r.get("purpose") == "undecided" {
			add("a completed case requires a resolved purpose")
		}
		if r.get("vault-outcome") == "not-evaluated" {
			add("a completed case requires an evaluated vault-outcome")
		}
	}
	if r.get("purpose") == "development" || r.get("purpose") == "mixed" {
		s := section(r.text, requiredSections[1])
		if !regexp.MustCompile(`(?m)^### (?:Current productive state|Estado productivo actual)\s*$`).MatchString(s) {
			add("missing Current productive state subsection")
		}
		if !regexp.MustCompile(`(?m)^### (?:Future/proposed state|Estado futuro/propuesto|Estado futuro o propuesto)\s*$`).MatchString(s) {
			add("missing Future/proposed state subsection")
		}
	}
	h := headings(r.text)
	last := -1
	missing := []string{}
	order := false
	for _, a := range requiredSections {
		pos := -1
		for i, x := range h {
			if contains(a, x) {
				pos = i
				break
			}
		}
		if pos < 0 {
			missing = append(missing, a[0])
		} else {
			if pos < last {
				order = true
			}
			last = pos
		}
	}
	if len(missing) > 0 {
		add("missing sections " + strings.Join(missing, ", "))
	} else if order {
		add("required sections are out of order")
	}
	for _, s := range validateHandoffs(r.text) {
		add(s)
	}
	if e := publicSafe(filepath.Dir(r.path)); e != nil {
		add(e.Error())
	}
	integrity := r.get("artifact-integrity")
	if integrity != "" && integrity != "sha256-v1" {
		add("invalid artifact-integrity")
	}
	_, entries, e := Snapshot(filepath.Dir(r.path))
	if e != nil {
		add(e.Error())
	} else {
		for _, s := range artifactErrors(entries, r.text, integrity == "sha256-v1", nil) {
			add(s)
		}
		if integrity == "sha256-v1" {
			for _, s := range largeErrors(filepath.Dir(r.path), entries, r.text, nil) {
				add(s)
			}
		}
	}
	archive := filepath.Join(filepath.Dir(r.path), "artifacts", "consolidated")
	dirs, e := os.ReadDir(archive)
	if e == nil {
		for _, d := range dirs {
			if d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
				for _, s := range validateArchive(filepath.Join(archive, d.Name())) {
					add(s)
				}
			}
		}
	}
	return out
}

var artifactUnit = regexp.MustCompile(`^(A-[0-9]{3,})(?:$|[-.])`)
var artifactHeader = regexp.MustCompile("^(?:-\\s+|###\\s+)`?(A-[0-9]{3,})`?(?:$|[\\s:—(])")

func unitID(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] != "artifacts" {
		return ""
	}
	m := artifactUnit.FindStringSubmatch(parts[1])
	if m == nil {
		return ""
	}
	return m[1]
}
func artifactEntry(text, id string) string {
	if id == "" {
		return ""
	}
	lines := strings.Split(section(text, requiredSections[2]), "\n")
	start := -1
	for i, l := range lines {
		m := artifactHeader.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		if start >= 0 {
			return strings.Join(lines[start:i], "\n")
		}
		if m[1] == id {
			start = i
		}
	}
	if start >= 0 {
		return strings.Join(lines[start:], "\n")
	}
	return ""
}
func artifactErrors(entries []Entry, text string, strict bool, targets map[string]bool) []string {
	out := []string{}
	for _, item := range entries {
		if item.Type != "file" || !strings.HasPrefix(item.Path, "artifacts/") {
			continue
		}
		unit := unitID(item.Path)
		if unit == "" {
			if strict && targets == nil {
				out = append(out, item.Path+": artifact is outside an A-NNN unit")
			}
			continue
		}
		if targets != nil && !targets[unit] {
			continue
		}
		entry := artifactEntry(text, unit)
		if !strict && !regexp.MustCompile(regexp.QuoteMeta(item.Path)+".{0,24}SHA-256\\s+`?[a-fA-F0-9]{64}").MatchString(entry) {
			continue
		}
		if entry == "" {
			out = append(out, item.Path+": missing "+unit+" register entry")
			continue
		}
		match := false
		for _, l := range strings.Split(entry, "\n") {
			match = match || (strings.Contains(l, item.Path) && strings.Contains(l, item.SHA256))
		}
		if !match {
			out = append(out, item.Path+": "+unit+" must record path and SHA-256 on one line")
		}
	}
	return out
}

var largeApproval = regexp.MustCompile("\\blarge-artifact-approval:\\s*user:[^\\s`]+")

func largeErrors(dir string, entries []Entry, text string, targets map[string]bool) []string {
	out := []string{}
	for _, item := range entries {
		if item.Type != "file" || !strings.HasPrefix(item.Path, "artifacts/") {
			continue
		}
		unit := unitID(item.Path)
		if targets != nil && !targets[unit] {
			continue
		}
		st, e := os.Stat(filepath.Join(dir, filepath.FromSlash(item.Path)))
		if e != nil {
			out = append(out, e.Error())
			continue
		}
		if st.Size() <= 10*1024*1024 {
			continue
		}
		approved := false
		for _, l := range strings.Split(artifactEntry(text, unit), "\n") {
			approved = approved || (strings.Contains(l, item.Path) && largeApproval.MatchString(l))
		}
		if !approved {
			out = append(out, item.Path+": exceeds 10 MiB without large-artifact-approval: user:<source>")
		}
	}
	return out
}
func validateArchive(dir string) []string {
	b, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return []string{dir + ": missing manifest.json"}
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return []string{dir + ": invalid manifest"}
	}
	var files []map[string]string
	if json.Unmarshal(m["files"], &files) != nil || files == nil {
		return []string{dir + ": files must be a list"}
	}
	out := []string{}
	for _, item := range files {
		if len(item) != 2 || item["path"] == "" || item["sha256"] == "" {
			out = append(out, dir+": invalid file entry")
			continue
		}
		p := filepath.Join(dir, item["path"])
		resolved, e := filepath.EvalSymlinks(p)
		if e != nil {
			out = append(out, dir+": missing archived file "+item["path"])
			continue
		}
		rel, e := filepath.Rel(dir, resolved)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			out = append(out, dir+": archived path escapes "+item["path"])
			continue
		}
		b, e := os.ReadFile(resolved)
		if e != nil || digest(b) != item["sha256"] {
			out = append(out, dir+": hash mismatch "+item["path"])
		}
	}
	return out
}
func validateVault(root string) ([]string, error) {
	rs, _, e := records(root)
	if e != nil {
		return nil, e
	}
	ret, e := retired(root)
	if e != nil {
		return nil, e
	}
	if e = retirementConflicts(rs, ret); e != nil {
		return nil, e
	}
	out := []string{}
	for _, store := range []string{root, filepath.Join(filepath.Dir(root), ".investigations")} {
		dirs, e := os.ReadDir(store)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return nil, e
		}
		for _, d := range dirs {
			n := d.Name()
			if (strings.HasPrefix(n, ".open-") || strings.HasPrefix(n, ".retire-") || strings.HasPrefix(n, ".publish-")) && strings.HasSuffix(n, ".tmp") {
				out = append(out, "incomplete staging directory: "+n)
			}
		}
	}
	ids, keys, lineage := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, r := range rs {
		out = append(out, validateRecord(r)...)
		id, key := r.get("id"), r.get("dedupe-key")
		if ids[id] {
			out = append(out, "duplicate id: "+id)
		}
		ids[id] = true
		if key != "" && keys[key] {
			out = append(out, "duplicate dedupe-key: "+key)
		}
		keys[key] = true
	}
	for _, r := range rs {
		for _, id := range r.lineage() {
			if lineage[id] {
				out = append(out, "duplicate consolidated-from id: "+id)
			}
			lineage[id] = true
			if ids[id] {
				out = append(out, "consolidated-from id still has an active directory: "+id)
			}
		}
	}
	return out, nil
}
func validationResult(root string) (any, error) {
	errs, e := validateVault(root)
	if e != nil {
		return nil, e
	}
	if len(errs) > 0 {
		return map[string]any{"status": "invalid", "errors": errs, "warnings": []string{}}, fmt.Errorf("investigation validation failed: %s", strings.Join(errs, "; "))
	}
	rs, _, e := records(root)
	if e != nil {
		return nil, e
	}
	return map[string]any{"status": "valid", "cases": len(rs), "warnings": []string{}}, nil
}
