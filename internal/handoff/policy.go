package handoff

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed assets/*.md
var policyAssets embed.FS

const managedBegin = `<!-- knowledge-os:managed:start id="development handoff" -->`
const managedEnd = `<!-- knowledge-os:managed:end id="development handoff" -->`
const updatesName = "implementation-updates.md"

func asset(name string) []byte {
	b, err := policyAssets.ReadFile("assets/" + name)
	if err != nil {
		panic(err)
	}
	return b
}
func startAsset() []byte   { return asset("start.md") }
func updatesAsset() []byte { return asset(updatesName) }

func policyOptional(path string) ([]byte, error) {
	b, err := readFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}
func decodeManaged(b []byte) (text string, bom bool, newline string, err error) {
	bom = bytes.HasPrefix(b, []byte{239, 187, 191})
	b = bytes.TrimPrefix(b, []byte{239, 187, 191})
	if !utf8.Valid(b) {
		return "", false, "", errors.New("managed file must be UTF-8 text")
	}
	text = string(b)
	newline = "\n"
	crlf := strings.Count(text, "\r\n")
	if crlf > strings.Count(text, "\n")-crlf {
		newline = "\r\n"
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return
}
func encodeManaged(text string, bom bool, newline string) []byte {
	b := []byte(strings.ReplaceAll(text, "\n", newline))
	if bom {
		b = append([]byte{239, 187, 191}, b...)
	}
	return b
}
func rootInstructionFiles(target string) ([]string, error) {
	info, err := os.Lstat(filepath.Join(target, "AGENTS.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("AGENTS.md must be a physical regular file")
	}
	return []string{"AGENTS.md"}, nil
}
func joinManaged(block, after string) string {
	if strings.TrimSpace(after) == "" {
		return block + after
	}
	n := len(after) - len(strings.TrimLeft(after, "\n"))
	if n < 2 {
		return block + strings.Repeat("\n", 2-n) + after
	}
	return block + after
}
func insertionOffset(text string) int {
	lines := strings.SplitAfter(text, "\n")
	i := 0
	if len(lines) > 0 && strings.TrimSuffix(lines[0], "\n") == "---" {
		for j := 1; j < len(lines); j++ {
			if strings.TrimSuffix(lines[j], "\n") == "---" {
				i = j + 1
				break
			}
		}
	}
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && strings.HasPrefix(lines[i], "# ") {
		i++
	}
	return len(strings.Join(lines[:i], ""))
}
func prepareInstructionFile(path string) ([]byte, error) {
	raw, e := policyOptional(path)
	if e != nil {
		return nil, e
	}
	text, bom, newline, e := decodeManaged(raw)
	if e != nil {
		return nil, e
	}
	if strings.Contains(text, "<!-- BEGIN MANAGED: System A-System B DEVELOPMENT HANDOFF -->") || strings.Contains(text, "<!-- END MANAGED: System A-System B DEVELOPMENT HANDOFF -->") {
		return nil, errors.New("unsupported managed block version")
	}
	beginCount, endCount := strings.Count(text, managedBegin), strings.Count(text, managedEnd)
	block := strings.Trim(string(asset("agents-managed-block.md")), "\n")
	if beginCount == 0 && endCount == 0 {
		offset := insertionOffset(text)
		before, after := text[:offset], text[offset:]
		if before != "" && !strings.HasSuffix(before, "\n") {
			before += "\n"
		}
		if before != "" && !strings.HasSuffix(before, "\n\n") {
			before += "\n"
		}
		text = before + joinManaged(block, after)
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
	} else {
		begin, end := strings.Index(text, managedBegin), strings.Index(text, managedEnd)
		if beginCount != 1 || endCount != 1 || end < begin {
			return nil, errors.New("invalid managed block")
		}
		text = text[:begin] + joinManaged(block, text[end+len(managedEnd):])
	}
	desired := encodeManaged(text, bom, newline)
	if len(desired) > 32768 {
		return nil, errors.New("instruction file too large")
	}
	return desired, nil
}
func prepareInstructions(target string) (map[string][]byte, error) {
	override, e := policyOptional(filepath.Join(target, "AGENTS.override.md"))
	if e != nil {
		return nil, e
	}
	if len(bytes.TrimSpace(override)) != 0 {
		return nil, errors.New("non-empty AGENTS.override.md hides managed instructions")
	}
	names, e := rootInstructionFiles(target)
	if e != nil {
		return nil, e
	}
	desired := map[string][]byte{}
	for _, name := range names {
		b, e := prepareInstructionFile(filepath.Join(target, name))
		if e != nil {
			return nil, e
		}
		desired[name] = b
	}
	return desired, nil
}
func prepareIgnore(target string) ([]byte, error) {
	raw, e := policyOptional(filepath.Join(target, ".gitignore"))
	if e != nil {
		return nil, e
	}
	text, bom, newline, e := decodeManaged(raw)
	if e != nil {
		return nil, e
	}
	found := false
	for _, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "!") && strings.Contains(s, storeName) {
			return nil, errors.New("gitignore negation conflicts with local handoff store")
		}
		if line == "/"+storeName+"/" {
			found = true
		}
	}
	if found {
		return raw, nil
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return encodeManaged(text+"/"+storeName+"/\n", bom, newline), nil
}

var updateHeading = regexp.MustCompile(`^## UPD-([0-9]{3,}) — (.+)$`)
var updateField = regexp.MustCompile(`^- ([A-Za-z][A-Za-z ]+):\s*(.*)$`)
var relatedUpdate = regexp.MustCompile(`\bUPD-([0-9]{3,})\b`)
var updateRequired = []string{"Recorded at", "Source or trigger", "Initial definition affected", "Update", "Status", "Reason or agreement", "Impact", "Evidence", "Related entries"}

// Timestamp accepts the ISO-8601 timezone forms used by Python's fromisoformat,
// including space separators and basic dates/times, without permitting local time.
var updateWeekDate = regexp.MustCompile(`^([0-9]{4})-?W([0-9]{2})(?:-?([1-7]))?`)

func updateTimestamp(s string) bool {
	if match := updateWeekDate.FindStringSubmatch(s); match != nil {
		year, _ := strconv.Atoi(match[1])
		week, _ := strconv.Atoi(match[2])
		day := 1
		if match[3] != "" {
			day, _ = strconv.Atoi(match[3])
		}
		jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
		weekday := (int(jan4.Weekday()) + 6) % 7
		date := jan4.AddDate(0, 0, -weekday+(week-1)*7+day-1)
		y, w := date.ISOWeek()
		if y != year || w != week {
			return false
		}
		s = date.Format("2006-01-02") + s[len(match[0]):]
	}
	s = strings.ReplaceAll(s, ",", ".")
	// fromisoformat permits one arbitrary Unicode date/time separator.
	dateEnd := 0
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		dateEnd = 10
	} else if len(s) >= 8 && s[4] != '-' && s[4] != 'W' {
		dateEnd = 8
	}
	if dateEnd > 0 && len(s) > dateEnd {
		_, n := utf8.DecodeRuneInString(s[dateEnd:])
		s = s[:dateEnd] + "T" + s[dateEnd+n:]
	}
	if len([]rune(s)) > 64 {
		return false
	}
	for _, date := range []string{"2006-01-02", "20060102"} {
		for _, sep := range []string{"T", " ", "t"} {
			for _, clock := range []string{"15:04:05", "15:04", "15", "150405", "1504"} {
				for _, zone := range []string{"Z07:00", "Z0700", "Z07:00:00", "Z07"} {
					if _, e := time.Parse(date+sep+clock+zone, s); e == nil {
						return true
					}
				}
			}
		}
	}
	return false
}
func validateUpdates(path string) (map[string]any, error) {
	raw, e := policyOptional(path)
	if e != nil {
		return nil, e
	}
	if raw == nil {
		return nil, errors.New("implementation updates missing")
	}
	if len(raw) > maxFileBytes {
		return nil, errors.New("implementation updates exceeds byte limit")
	}
	text, _, _, e := decodeManaged(raw)
	if e != nil {
		return nil, e
	}
	if strings.ContainsRune(text, 0) {
		return nil, errors.New("implementation updates contains NUL")
	}
	if e = scanSecrets(raw); e != nil {
		return nil, e
	}
	lines := strings.Split(text, "\n")
	first := ""
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			first = line
			break
		}
	}
	if first != "# Implementation updates" {
		return nil, errors.New("implementation updates must start with canonical H1")
	}
	type heading struct {
		line, number int
		title        string
	}
	headings := []heading{}
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		m := updateHeading.FindStringSubmatch(line)
		if m == nil {
			return nil, errors.New("every H2 must be a canonical UPD entry")
		}
		n, e := strconv.Atoi(m[1])
		if e != nil {
			return nil, e
		}
		headings = append(headings, heading{i, n, strings.TrimSpace(m[2])})
	}
	for i, h := range headings {
		if h.number != i+1 || h.title == "" {
			return nil, errors.New("update IDs must be contiguous from UPD-001")
		}
		end := len(lines)
		if i+1 < len(headings) {
			end = headings[i+1].line
		}
		fields := map[string]string{}
		order := []string{}
		for _, line := range lines[h.line+1 : end] {
			if !strings.HasPrefix(line, "- ") {
				continue
			}
			m := updateField.FindStringSubmatch(line)
			if m == nil {
				return nil, errors.New("entry bullet must use canonical label")
			}
			key, value := m[1], strings.TrimSpace(m[2])
			allowed := key == "Analysis"
			for _, k := range updateRequired {
				allowed = allowed || k == key
			}
			if !allowed || fields[key] != "" || value == "" {
				return nil, errors.New("unknown, empty or duplicate update field")
			}
			fields[key] = value
			order = append(order, key)
		}
		expected := append([]string{}, updateRequired...)
		if _, ok := fields["Analysis"]; ok {
			expected = append(expected, "Analysis")
		}
		if strings.Join(order, "\x00") != strings.Join(expected, "\x00") {
			return nil, errors.New("update fields must follow canonical order")
		}
		if !updateTimestamp(fields["Recorded at"]) {
			return nil, errors.New("Recorded at must include ISO-8601 UTC offset")
		}
		switch fields["Status"] {
		case "proposed", "agreed", "implemented", "rejected", "superseded":
		default:
			return nil, errors.New("invalid update status")
		}
		related := fields["Related entries"]
		ids := relatedUpdate.FindAllStringSubmatch(related, -1)
		if !strings.EqualFold(related, "none") && len(ids) == 0 {
			return nil, errors.New("related entries must be none or earlier UPD IDs")
		}
		for _, id := range ids {
			n, e := strconv.Atoi(id[1])
			if e != nil || n >= h.number {
				return nil, errors.New("related entries may reference only earlier updates")
			}
		}
		switch strings.ToLower(fields["Analysis"]) {
		case "none", "n/a", "not applicable", "no analysis":
			return nil, errors.New("omit Analysis when no analysis occurred")
		}
	}
	var latest any
	if len(headings) > 0 {
		latest = fmt.Sprintf("UPD-%03d", headings[len(headings)-1].number)
	}
	return map[string]any{"path": updatesName, "entries": len(headings), "latest": latest}, nil
}
func prepareUpdates(familyPath string) (string, map[string]any, error) {
	path := filepath.Join(familyPath, updatesName)
	raw, e := policyOptional(path)
	if e != nil {
		return "", nil, e
	}
	if raw == nil {
		return "create", map[string]any{"path": updatesName, "entries": 0, "latest": nil}, nil
	}
	summary, e := validateUpdates(path)
	return "none", summary, e
}
