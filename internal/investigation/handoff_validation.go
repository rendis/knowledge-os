package investigation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/text/cases"
	"io"
	"net/url"
	"os/exec"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var dhFields = []string{"Story ID", "Tracker ID", "Provider", "Tracker URL", "Work item reference", "Repository remote", "Branch", "Handoff ID", "Family", "Revision", "Materialized at"}
var dhKeys = []string{"dh", "story-id", "tracker-id", "provider", "tracker-url", "work-item-reference", "repository-remote", "branch", "handoff-id", "family", "revision", "materialized-at"}

const dhMarker = "knowledge-os:development-handoff-binding"

var dhLinesRE = regexp.MustCompile(`\r\n|[\n\r\v\f\x1c\x1d\x1e\x{85}\x{2028}\x{2029}]`)
var dhHeadingRE = regexp.MustCompile(`^### (DH-([0-9]{3,})) — (.+)$`)
var dhFieldRE = regexp.MustCompile(`^- ([A-Za-z ]+): (.+)$`)
var dhRevisionRE = regexp.MustCompile(`^v[0-9]{4}$`)
var dhSlugRE = regexp.MustCompile(`[a-z0-9]+`)
var dhMarkerRE = regexp.MustCompile(`^  <!-- knowledge-os:development-handoff-binding (\{.*\}) -->$`)

type dhEntry struct {
	id, number, title string
	fields            map[string]string
}

func dhSection(text string, names ...string) ([]string, int) {
	lines := dhLinesRE.Split(text, -1)
	start, count := 0, 0
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			for _, name := range names {
				if strings.TrimSpace(line[3:]) == name {
					start = i + 1
					count++
				}
			}
		}
	}
	if count != 1 {
		return nil, count
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return lines[start:end], count
}
func dhComplete(f map[string]string) bool {
	for _, key := range dhFields {
		if _, ok := f[key]; !ok {
			return false
		}
	}
	return true
}
func dhBranch(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && utf8.RuneCountInString(s) <= 255 && exec.Command("git", "check-ref-format", "--branch", s).Run() == nil
}
func dhSlug(s string) string {
	return strings.Join(dhSlugRE.FindAllString(cases.Fold().String(s), -1), "-")
}
func dhTimestamp(s string) (time.Time, bool) {
	// ISO timestamps emitted by the lifecycle carry an explicit UTC offset.
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02T15:04:05.999999999-0700", "2006-01-02 15:04:05.999999999-0700", "2006-01-02T15:04Z07:00", "2006-01-02 15:04Z07:00"} {
		if t, e := time.Parse(layout, s); e == nil {
			_, offset := t.Zone()
			if t.Year() >= 1 && t.Year() <= 9999 && offset > -86400 && offset < 86400 {
				return t, true
			}
		}
	}
	return time.Time{}, false
}
func dhTrackerURL(s string) string {
	u, e := url.Parse(s)
	if e != nil || !strings.EqualFold(u.Scheme, "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	host := cases.Fold().String(u.Hostname())
	if p := u.Port(); p != "" {
		n, e := strconv.Atoi(p)
		if e != nil || n < 0 || n > 65535 {
			return ""
		}
		if n != 0 {
			host += fmt.Sprintf(":%d", n)
		}
	}
	path := u.RawPath
	if path == "" {
		path = u.Path
	}
	return "https://" + host + strings.TrimRight(path, "/")
}
func dhBinding(e dhEntry) map[string]string {
	m := map[string]string{"dh": e.id}
	for i, f := range dhFields {
		m[dhKeys[i+1]] = e.fields[f]
	}
	return m
}

// Python-compatible compact JSON retains Unicode, HTML characters and U+2028/2029.
func dhJSON(m map[string]string) string {
	var b strings.Builder
	quote := func(s string) {
		b.WriteByte('"')
		for _, r := range s {
			switch r {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if r < 32 {
					fmt.Fprintf(&b, `\u%04x`, r)
				} else {
					b.WriteRune(r)
				}
			}
		}
		b.WriteByte('"')
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		quote(k)
		b.WriteByte(':')
		quote(m[k])
	}
	b.WriteByte('}')
	return b.String()
}

func validateHandoffs(text string) []string {
	lines, count := dhSection(text, "Development handoffs", "Handoffs de desarrollo")
	if count == 0 {
		return nil
	}
	if count != 1 {
		return []string{"Development handoffs section must appear exactly once"}
	}
	var errs []string
	var entries []dhEntry
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		h := dhHeadingRE.FindStringSubmatch(lines[i])
		if h == nil {
			errs = append(errs, "Development handoffs contains content outside an exact DH-NNN entry")
			break
		}
		i++
		e := dhEntry{h[1], h[2], strings.TrimSpace(h[3]), map[string]string{}}
		var order []string
		for i < len(lines) && !strings.HasPrefix(lines[i], "### ") {
			line := lines[i]
			i++
			if strings.TrimSpace(line) == "" {
				continue
			}
			m := dhFieldRE.FindStringSubmatch(line)
			if m == nil {
				errs = append(errs, e.id+" contains invalid content")
				continue
			}
			known := false
			for _, f := range dhFields {
				if m[1] == f {
					known = true
					break
				}
			}
			if !known {
				errs = append(errs, e.id+" contains unknown field "+m[1])
				continue
			}
			if _, ok := e.fields[m[1]]; ok {
				errs = append(errs, e.id+" repeats field "+m[1])
				continue
			}
			e.fields[m[1]] = strings.TrimSpace(m[2])
			order = append(order, m[1])
		}
		var missing []string
		for _, f := range dhFields {
			if e.fields[f] == "" {
				missing = append(missing, f)
			}
		}
		if len(missing) > 0 {
			errs = append(errs, e.id+" is missing fields "+strings.Join(missing, ", "))
		} else if !reflect.DeepEqual(order, dhFields) {
			errs = append(errs, e.id+" fields are out of order")
		}
		entries = append(entries, e)
	}
	ids := map[string]bool{}
	duplicate, contiguous := false, true
	for i, e := range entries {
		if ids[e.id] {
			duplicate = true
		}
		ids[e.id] = true
		n, err := strconv.Atoi(e.number)
		if err != nil || n != i+1 {
			contiguous = false
		}
	}
	if duplicate {
		errs = append(errs, "Development handoffs contains duplicate DH-NNN identifiers")
	}
	if !contiguous {
		errs = append(errs, "Development handoff identifiers must be contiguous from DH-001")
	}
	targets := map[[2]string]bool{}
	branches := map[string]string{}
	handoffIDs := map[string]bool{}
	for _, e := range entries {
		f := e.fields
		if !dhComplete(f) {
			continue
		}
		bad := func(cond bool, label string) {
			if cond {
				errs = append(errs, e.id+label)
			}
		}
		bad(!regexp.MustCompile(`^S-[0-9]{3,}$`).MatchString(f["Story ID"]), " has invalid Story ID")
		u := dhTrackerURL(f["Tracker URL"])
		bad(u == "" || u != f["Tracker URL"], " has invalid canonical Tracker URL")
		for _, name := range []string{"Tracker ID", "Provider"} {
			bad(!regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(f[name]), " has invalid "+name)
		}
		bad(f["Work item reference"] == "" || utf8.RuneCountInString(f["Work item reference"]) > 200, " has invalid Work item reference")
		remote := f["Repository remote"]
		parts := strings.Split(remote, "/")
		invalid := remote != cases.Fold().String(remote) || strings.Contains(remote, "://") || strings.Contains(remote, `\`) || strings.HasSuffix(remote, ".git") || len(parts) < 2 || strings.ContainsFunc(remote, unicode.IsSpace)
		for _, p := range parts {
			if p == "" || p == "." || p == ".." {
				invalid = true
			}
		}
		bad(invalid, " has invalid normalized Repository remote")
		bad(!dhBranch(f["Branch"]), " has invalid Branch")
		bad(!regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(f["Handoff ID"]), " has invalid Handoff ID")
		bad(!regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*--[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(f["Family"]), " has invalid Family")
		bad(!dhRevisionRE.MatchString(f["Revision"]), " has invalid Revision")
		_, ok := dhTimestamp(f["Materialized at"])
		bad(!ok, " has invalid Materialized at timestamp")
		repo := parts[len(parts)-1]
		bad(e.title != f["Tracker ID"]+":"+f["Work item reference"]+" / "+repo, " heading does not match work item and repository")
		sum := sha256.Sum256([]byte(strings.Join([]string{f["Tracker ID"], f["Work item reference"], remote}, "|")))
		id := hex.EncodeToString(sum[:])
		readable := dhSlug(f["Tracker ID"] + "-" + f["Work item reference"])
		if len(readable) > 80 {
			readable = readable[:80]
		}
		family := strings.TrimRight(readable, "-") + "-" + id[:10] + "--" + dhSlug(repo)
		bad(f["Family"] != family, " Family does not match work item and repository")
		bad(f["Handoff ID"] != id, " Handoff ID does not match its identity")
		target := [2]string{f["Story ID"], remote}
		bad(targets[target], " duplicates a story-and-repository target")
		targets[target] = true
		prev, exists := branches[f["Branch"]]
		bad(exists && prev != remote, " reuses a Branch for a different Repository remote")
		if !exists {
			branches[f["Branch"]] = remote
		}
		bad(handoffIDs[f["Handoff ID"]], " reuses a Handoff ID")
		handoffIDs[f["Handoff ID"]] = true
	}
	return append(errs, dhHistory(text, entries)...)
}

// Decode JSON tokens rather than into a map so duplicate object keys cannot be hidden.
func dhDecode(raw string) (map[string]string, []string, error) {
	d := json.NewDecoder(strings.NewReader(raw))
	duplicates := map[string]bool{}
	var read func() (any, error)
	read = func() (any, error) {
		t, e := d.Token()
		if e != nil {
			return nil, e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return t, nil
		}
		switch delim {
		case '{':
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				key, ok := k.(string)
				if !ok {
					return nil, fmt.Errorf("invalid key")
				}
				v, e := read()
				if e != nil {
					return nil, e
				}
				if _, exists := m[key]; exists {
					duplicates[key] = true
				}
				m[key] = v
			}
			_, e = d.Token()
			return m, e
		case '[':
			var a []any
			for d.More() {
				v, e := read()
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			_, e = d.Token()
			return a, e
		}
		return nil, fmt.Errorf("invalid delimiter")
	}
	v, e := read()
	if e == nil {
		_, tail := d.Token()
		if tail != io.EOF {
			e = fmt.Errorf("trailing JSON")
		}
	}
	if e != nil {
		return nil, nil, e
	}
	var dup []string
	for k := range duplicates {
		dup = append(dup, k)
	}
	sort.Strings(dup)
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, dup, nil
	}
	result := map[string]string{}
	for k, v := range obj {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, dup, nil
		}
		result[k] = s
	}
	return result, dup, nil
}

func dhHistory(text string, entries []dhEntry) []string {
	lines, count := dhSection(text, "History", "Historial")
	if count == 0 {
		if strings.Contains(text, dhMarker) {
			return []string{"Development handoff binding markers must appear only in History"}
		}
		return nil
	}
	if count != 1 {
		return []string{"History section must appear exactly once"}
	}
	var errs []string
	if strings.Count(text, dhMarker) != strings.Count(strings.Join(lines, "\n"), dhMarker) {
		errs = append(errs, "Development handoff binding markers must appear only in History")
	}
	type marker struct {
		fields map[string]string
		event  string
	}
	var markers []marker
	event := ""
	hasBinding := false
	for _, line := range lines {
		if strings.HasPrefix(line, "- ") {
			event = line
			hasBinding = false
			continue
		}
		if !strings.Contains(line, dhMarker) {
			if line != "" {
				r, _ := utf8.DecodeRuneInString(line)
				if !unicode.IsSpace(r) {
					event = ""
					hasBinding = false
				}
			}
			continue
		}
		if event == "" {
			errs = append(errs, "Development handoff binding must belong to one human-readable History event")
		} else if hasBinding {
			errs = append(errs, "A History event must contain at most one development handoff binding")
		} else {
			hasBinding = true
		}
		match := dhMarkerRE.FindStringSubmatch(line)
		if match == nil {
			errs = append(errs, "History contains an invalid development handoff binding marker")
			continue
		}
		value, duplicates, err := dhDecode(match[1])
		if err != nil {
			errs = append(errs, "History contains invalid development handoff binding JSON")
			continue
		}
		if len(duplicates) > 0 {
			errs = append(errs, "History development handoff binding contains duplicate keys: "+strings.Join(duplicates, ", "))
			continue
		}
		valid := len(value) == len(dhKeys)
		for _, key := range dhKeys {
			if value[key] == "" {
				valid = false
			}
		}
		if !valid {
			errs = append(errs, "History development handoff binding has invalid fields")
			continue
		}
		if match[1] != dhJSON(value) {
			errs = append(errs, "History development handoff binding must use canonical compact JSON")
		}
		markers = append(markers, marker{value, event})
	}
	entryFields := map[string]dhEntry{}
	for _, e := range entries {
		if dhComplete(e.fields) {
			entryFields[e.id] = e
		}
	}
	revisions := map[string][]int{}
	times := map[string][]time.Time{}
	current := map[string]int{}
	for _, m := range markers {
		v := m.fields
		id := v["dh"]
		e, ok := entryFields[id]
		if !ok {
			errs = append(errs, "History binding references unknown "+id)
			continue
		}
		stable := true
		binding := dhBinding(e)
		for _, k := range []string{"story-id", "tracker-id", "provider", "tracker-url", "work-item-reference", "repository-remote", "handoff-id", "family"} {
			if v[k] != binding[k] {
				stable = false
			}
		}
		if !stable {
			errs = append(errs, "History binding for "+id+" changes its stable identity")
			continue
		}
		if !dhRevisionRE.MatchString(v["revision"]) || !dhRevisionRE.MatchString(e.fields["Revision"]) {
			errs = append(errs, "History binding for "+id+" has invalid Revision")
			continue
		}
		rev, _ := strconv.Atoi(v["revision"][1:])
		now, _ := strconv.Atoi(e.fields["Revision"][1:])
		if rev < 1 || rev > now {
			errs = append(errs, "History binding for "+id+" exceeds its current Revision")
			continue
		}
		if !dhBranch(v["branch"]) {
			errs = append(errs, "History binding for "+id+" has invalid Branch")
			continue
		}
		timestamp, ok := dhTimestamp(v["materialized-at"])
		if !ok {
			errs = append(errs, "History binding for "+id+" has invalid Materialized at")
			continue
		}
		if m.event != "" {
			prefix := "- " + v["materialized-at"] + " — "
			target := "development handoff `" + id + "`"
			if !strings.HasPrefix(m.event, prefix) {
				errs = append(errs, "History event for "+id+" must start with its Materialized at")
			} else {
				action, _, found := strings.Cut(strings.TrimPrefix(m.event, prefix), " "+target+";")
				if !found || !strings.ContainsFunc(action, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) {
					errs = append(errs, "History event for "+id+" must name an action before its development handoff target")
				}
			}
			fragments := [][2]string{{id, target}, {v["story-id"], "story `" + v["story-id"] + "`"}, {v["work-item-reference"], "work item `" + v["tracker-id"] + ":" + v["work-item-reference"] + "`"}, {v["repository-remote"], "repository `" + v["repository-remote"] + "`"}, {v["branch"], "branch `" + v["branch"] + "`"}, {v["handoff-id"], "handoff `" + v["handoff-id"] + "`"}, {v["revision"], "revision `" + v["revision"] + "`"}}
			var missing []string
			for _, p := range fragments {
				if !strings.Contains(m.event, p[1]) {
					missing = append(missing, p[0])
				}
			}
			if len(missing) > 0 {
				errs = append(errs, "History event for "+id+" does not name its exact binding target: "+strings.Join(missing, ", "))
			}
		}
		revisions[id] = append(revisions[id], rev)
		times[id] = append(times[id], timestamp)
		if reflect.DeepEqual(v, binding) {
			current[id]++
		}
	}
	// Use source entry order for deterministic diagnostics.
	seen := map[string]bool{}
	for _, entry := range entries {
		e, ok := entryFields[entry.id]
		if !ok || seen[e.id] {
			continue
		}
		seen[e.id] = true
		id := e.id
		if !dhRevisionRE.MatchString(e.fields["Revision"]) {
			continue
		}
		n, _ := strconv.Atoi(e.fields["Revision"][1:])
		observed := revisions[id]
		ordered := append([]int{}, observed...)
		sort.Ints(ordered)
		coverage := len(ordered) == n
		for i, r := range ordered {
			if r != i+1 {
				coverage = false
			}
		}
		if !coverage {
			errs = append(errs, "History bindings for "+id+" must cover each Revision from v0001")
		} else {
			inOrder := true
			for i, r := range observed {
				if r != i+1 {
					inOrder = false
				}
			}
			if !inOrder {
				errs = append(errs, "History bindings for "+id+" must appear in Revision order from v0001")
			} else {
				for i := 1; i < len(times[id]); i++ {
					if times[id][i].Before(times[id][i-1]) {
						errs = append(errs, "History bindings for "+id+" Materialized at timestamps must preserve Revision chronology")
						break
					}
				}
			}
		}
		if current[id] != 1 {
			errs = append(errs, "History must contain exactly one current binding for "+id)
		}
	}
	return errs
}
