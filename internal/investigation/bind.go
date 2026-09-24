package investigation

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// bindCase records a reviewed observation. It deliberately never reads a worktree.
func bindCase(root string, o options) (any, error) {
	if e := o.required("id", "observation", "expected-public-sha256", "expected-story-sha256"); e != nil {
		return nil, e
	}
	raw, e := readText(o.get("observation"))
	if e != nil {
		return nil, fmt.Errorf("binding_observation_invalid: %w", e)
	}
	observation, dups, e := dhDecode(raw)
	if e != nil || len(dups) > 0 || len(observation) != len(dhFields) {
		return nil, errors.New("binding_observation_invalid: expected exact binding fields excluding dh")
	}
	fields := map[string]string{}
	for i, key := range dhKeys[1:] {
		v, ok := observation[key]
		if !ok || v == "" || strings.ContainsAny(v, "\r\n") || secretPattern.MatchString(v) {
			return nil, errors.New("binding_observation_invalid: expected nonempty fields without credentials or newlines")
		}
		fields[dhFields[i]] = v
	}
	who, e := identity(root)
	if e != nil {
		return nil, e
	}
	return mutationGate(root, func() (any, error) {
		r, e := locate(root, o.get("id"))
		if e != nil {
			return nil, e
		}
		original := []byte(r.text)
		if digest(original) != o.get("expected-public-sha256") {
			return nil, errors.New("stale_public_snapshot: investigation changed after binding review")
		}
		problems, e := validateVault(root)
		if e != nil {
			return nil, e
		}
		if len(problems) > 0 {
			return nil, fmt.Errorf("binding_validation_failed: source case is invalid: %s", strings.Join(problems, "; "))
		}
		exports := filepath.Join(filepath.Dir(r.path), "exports")
		if e = noSymlink(exports, true); e != nil {
			return nil, e
		}
		paths, e := filepath.Glob(filepath.Join(exports, "*.md"))
		if e != nil {
			return nil, e
		}
		var stories []string
		var story []byte
		for _, p := range paths {
			s, e := readText(p)
			if e != nil {
				return nil, e
			}
			meta, e := frontmatter(s)
			if e != nil {
				return nil, e
			}
			if meta["story-id"] == fields["Story ID"] {
				if meta["source-investigation"] != o.get("id") {
					return nil, errors.New("binding_story_missing: source case must contain one exact story draft")
				}
				stories = append(stories, p)
				story = []byte(s)
			}
		}
		if len(stories) != 1 {
			return nil, errors.New("binding_story_missing: source case must contain one exact story draft")
		}
		if digest(story) != o.get("expected-story-sha256") {
			return nil, errors.New("stale_story_snapshot: source story changed after binding review")
		}
		lines, count := dhSection(r.text, "Development handoffs", "Handoffs de desarrollo")
		if count != 1 {
			return nil, errors.New("binding_section_missing: source case needs Development handoffs section")
		}
		content := strings.Join(lines, "\n")
		headings := regexp.MustCompile(`(?m)^### (DH-[0-9]{3,}) — .*`).FindAllStringSubmatchIndex(content, -1)
		targetStart, targetEnd := -1, -1
		entryID := fmt.Sprintf("DH-%03d", len(headings)+1)
		var old map[string]string
		for i, h := range headings {
			end := len(content)
			if i+1 < len(headings) {
				end = headings[i+1][0]
			}
			values := map[string]string{}
			for _, line := range dhLinesRE.Split(content[h[0]:end], -1) {
				if m := dhFieldRE.FindStringSubmatch(line); m != nil {
					values[m[1]] = m[2]
				}
			}
			if values["Story ID"] == fields["Story ID"] && values["Repository remote"] == fields["Repository remote"] {
				targetStart, targetEnd = h[0], end
				old = values
				entryID = content[h[2]:h[3]]
			}
		}
		result := func(status string) any {
			return map[string]any{"status": status, "id": o.get("id"), "dh": entryID, "warnings": []string{}}
		}
		if reflect.DeepEqual(old, fields) {
			return result("unchanged"), nil
		}
		expected := 1
		if old != nil {
			for _, k := range dhFields {
				if k != "Revision" && k != "Materialized at" && old[k] != fields[k] {
					return nil, errors.New("binding_identity_mismatch: immutable binding coordinates changed")
				}
			}
			n, e := strconv.Atoi(strings.TrimPrefix(old["Revision"], "v"))
			if e != nil {
				return nil, errors.New("binding_revision_invalid")
			}
			expected = n + 1
		}
		if fields["Revision"] != fmt.Sprintf("v%04d", expected) {
			return nil, errors.New("binding_revision_invalid: binding requires the exact next revision")
		}
		remote := fields["Repository remote"]
		parts := strings.Split(remote, "/")
		title := fields["Tracker ID"] + ":" + fields["Work item reference"] + " / " + parts[len(parts)-1]
		block := "### " + entryID + " — " + title + "\n\n"
		for _, k := range dhFields {
			block += "- " + k + ": " + fields[k] + "\n"
		}
		block += "\n"
		if targetStart >= 0 {
			content = content[:targetStart] + block + content[targetEnd:]
		} else {
			content = strings.TrimRightFunc(content, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }) + "\n\n" + block
		}
		sectionRE := regexp.MustCompile(`(?m)^## (?:Development handoffs|Handoffs de desarrollo)[ \t]*\r?$`)
		section := sectionRE.FindStringIndex(r.text)
		if section == nil {
			return nil, errors.New("binding_section_missing")
		}
		start := section[1]
		if start < len(r.text) && r.text[start] == '\n' {
			start++
		}
		end := len(r.text)
		if next := regexp.MustCompile(`(?m)^## `).FindStringIndex(r.text[start:]); next != nil {
			end = start + next[0]
		}
		updated := r.text[:start] + "\n" + strings.TrimSpace(content) + "\n\n" + r.text[end:]
		action, recorder, source := "Bound", "recorded by", "source"
		if isSpanish(updated) {
			action, recorder, source = "Vinculado", "registrado por", "fuente"
		}
		event := fmt.Sprintf("- %s — %s development handoff `%s`; story `%s`; work item `%s:%s`; repository `%s`; branch `%s`; handoff `%s`; revision `%s`; %s %s; %s: validated handoff observation.\n  <!-- %s %s -->\n", fields["Materialized at"], action, entryID, fields["Story ID"], fields["Tracker ID"], fields["Work item reference"], remote, fields["Branch"], fields["Handoff ID"], fields["Revision"], recorder, who, source, dhMarker, dhJSON(dhBinding(dhEntry{id: entryID, fields: fields})))
		history := regexp.MustCompile(`(?m)^## (?:History|Historial)\s*$`).FindStringIndex(updated)
		if history == nil {
			return nil, errors.New("binding_validation_failed: History section missing")
		}
		position := strings.Index(updated[history[1]:], "\n## ")
		if position < 0 {
			position = len(updated)
		} else {
			position += history[1]
		}
		updated = strings.TrimRight(updated[:position], " \t\r\n") + "\n\n" + event + "\n" + updated[position:]
		candidate := r
		candidate.text = updated
		candidate.fields, e = frontmatter(updated)
		if e != nil {
			return nil, e
		}
		if problems := validateRecord(candidate); len(problems) > 0 {
			return nil, fmt.Errorf("binding_validation_failed: %s", strings.Join(problems, "; "))
		}
		// Recheck the reviewed draft immediately before exposing the observation.
		latest, e := readText(stories[0])
		if e != nil {
			return nil, e
		}
		if digest([]byte(latest)) != o.get("expected-story-sha256") {
			return nil, errors.New("stale_story_snapshot: source story changed before commit")
		}
		relative, e := filepath.Rel(filepath.Dir(root), r.path)
		if e != nil {
			return nil, e
		}
		if e = commitChanges(root, []fileChange{{Path: filepath.ToSlash(relative), Before: original, After: []byte(updated)}}); e != nil {
			return nil, e
		}
		return result("bound"), nil
	})
}
