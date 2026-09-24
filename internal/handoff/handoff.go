// Package handoff inspects the persisted handoff registry without changing it.
// Inspection checks byte integrity; it does not authorize publication or closure.
package handoff

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
	"golang.org/x/text/unicode/norm"
)

const storeName = ".knowledge-os-handoffs"
const maxFileBytes = 1 << 20
const maxStoredFileBytes = 64 << 20 // indented revision patches can exceed document size

var familyPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*--[a-z0-9]+(?:-[a-z0-9]+)*$`)
var revisionPattern = regexp.MustCompile(`^v[0-9]{4}$`)
var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Entry struct {
	ID          string `yaml:"handoff-id" json:"handoff_id"`
	Family      string `yaml:"family" json:"family"`
	Revision    string `yaml:"revision" json:"revision"`
	Manifest    string `yaml:"manifest" json:"manifest"`
	ActivatedAt string `yaml:"activated-at" json:"activated_at"`
	State       string `yaml:"state" json:"state"`
}

type activeRegistry struct {
	Schema        int     `yaml:"schema-version"`
	Investigation string  `yaml:"investigation-id"`
	Entries       []Entry `yaml:"handoffs"`
	ID            string  `yaml:"handoff-id"`
	Family        string  `yaml:"family"`
	Revision      string  `yaml:"revision"`
	Manifest      string  `yaml:"manifest"`
	ActivatedAt   string  `yaml:"activated-at"`
}

type fileRecord struct {
	SHA         string `yaml:"sha256"`
	SemanticSHA string `yaml:"semantic-sha256,omitempty"`
}

type manifest struct {
	Schema     int     `yaml:"schema-version"`
	ID         string  `yaml:"handoff-id"`
	Family     string  `yaml:"family"`
	Revision   string  `yaml:"revision"`
	Previous   *string `yaml:"previous"`
	CreatedAt  string  `yaml:"created-at"`
	UpdatedAt  string  `yaml:"updated-at"`
	Repository struct {
		Remote string `yaml:"remote"`
	} `yaml:"repository"`
	WorkItem map[string]any `yaml:"work-item"`
	Source   struct {
		Investigation string `yaml:"investigation-id"`
		UpdatedAt     string `yaml:"investigation-updated-at"`
		Story         string `yaml:"story-id"`
	} `yaml:"source"`
	Files   map[string]fileRecord `yaml:"files"`
	History struct {
		Path string `yaml:"path"`
		SHA  string `yaml:"sha256"`
	} `yaml:"history"`
}

type historyEvent struct {
	Schema      int      `yaml:"schema-version"`
	ID          string   `yaml:"handoff-id"`
	Revision    string   `yaml:"revision"`
	Previous    *string  `yaml:"previous"`
	PreviousSHA *string  `yaml:"previous-event-sha256"`
	CreatedAt   string   `yaml:"created-at"`
	Reason      string   `yaml:"reason"`
	Changed     []any    `yaml:"changed"`
	Unchanged   []string `yaml:"unchanged"`
}

type Result struct {
	Status        string   `json:"status"`
	Scope         string   `json:"scope"`
	Worktree      string   `json:"worktree"`
	Investigation string   `json:"investigation_id,omitempty"`
	Handoffs      []Entry  `json:"handoffs"`
	Unchecked     []string `json:"unchecked"`
}

// Run exposes handoff lifecycle operations. Mutations require the exact current
// plan token; semantic review remains the responsibility of the caller.
func Run(args []string, out io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "plan", "apply", "validate", "resolve-branch", "set-state":
			return familyRun(args, out)
		}
	}
	if len(args) > 0 && (args[0] == "plan-worktree" || args[0] == "create-worktree" || args[0] == "plan-handoff" || args[0] == "prepare-handoff") {
		return worktreeRun(args, out)
	}
	if len(args) == 0 || args[0] != "inspect" {
		return errors.New("handoff requires inspect, plan-worktree, create-worktree, plan-handoff, prepare-handoff, plan, apply, validate, resolve-branch or set-state")
	}
	fs := flag.NewFlagSet("handoff inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("worktree-path", "", "worktree containing the handoff store")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *root == "" || fs.NArg() != 0 {
		return errors.New("usage: handoff inspect --worktree-path PATH")
	}
	result, err := Inspect(*root)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}

// Inspect reads schema-1/2 ACTIVE wrappers and schema-2 handoff families. A
// successful result is not equivalent to the full development-handoff validator.
func Inspect(root string) (Result, error) {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Result{}, err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Result{}, err
	}
	if !info.IsDir() {
		return Result{}, errors.New("worktree must be a directory")
	}
	result := Result{Status: "inactive", Scope: "stored-byte-integrity", Worktree: resolved, Handoffs: []Entry{}, Unchecked: []string{"repository-and-branch-binding", "managed-instructions", "implementation-update-semantics", "closure-eligibility"}}
	store := filepath.Join(resolved, storeName)
	if _, err = os.Lstat(store); errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return Result{}, err
	}
	if err = safeDirectory(store); err != nil {
		return Result{}, err
	}
	if err = noTransaction(store); err != nil {
		return Result{}, err
	}
	raw, err := readFile(filepath.Join(store, "ACTIVE.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return Result{}, err
	}
	var active activeRegistry
	if err = decode(raw, &active); err != nil {
		return Result{}, fmt.Errorf("ACTIVE.yaml: %w", err)
	}
	if active.Schema == 1 {
		if active.Investigation != "" || active.Entries != nil {
			return Result{}, errors.New("legacy ACTIVE.yaml has schema-2 fields")
		}
		active.Entries = []Entry{{ID: active.ID, Family: active.Family, Revision: active.Revision, Manifest: active.Manifest, ActivatedAt: active.ActivatedAt, State: "active"}}
	} else if active.Schema == 2 {
		if active.ID != "" || active.Family != "" || active.Revision != "" || active.Manifest != "" || active.ActivatedAt != "" || strings.TrimSpace(active.Investigation) == "" {
			return Result{}, errors.New("invalid schema-2 ACTIVE.yaml")
		}
	} else {
		return Result{}, errors.New("unsupported ACTIVE.yaml schema")
	}
	if len(active.Entries) == 0 {
		return Result{}, errors.New("ACTIVE.yaml must have at least one handoff")
	}
	ids, families := map[string]bool{}, map[string]bool{}
	for _, entry := range active.Entries {
		if err = validEntry(entry); err != nil {
			return Result{}, err
		}
		if ids[entry.ID] || families[entry.Family] {
			return Result{}, errors.New("duplicate active handoff identity")
		}
		ids[entry.ID], families[entry.Family] = true, true
		inv, err := inspectFamily(store, entry)
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", entry.Family, err)
		}
		if active.Schema == 1 {
			active.Investigation = inv
		}
		if inv != active.Investigation {
			return Result{}, errors.New("handoff investigation differs from ACTIVE.yaml")
		}
	}
	// Refuse an observed concurrent update instead of returning a mixed registry.
	last, err := readFile(filepath.Join(store, "ACTIVE.yaml"))
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(raw, last) {
		return Result{}, errors.New("handoff registry changed during inspection; retry")
	}
	if err = noTransaction(store); err != nil {
		return Result{}, err
	}
	result.Status = "integrity-checked"
	result.Investigation = active.Investigation
	result.Handoffs = active.Entries
	return result, nil
}

func validEntry(e Entry) error {
	if !hashPattern.MatchString(e.ID) || !familyPattern.MatchString(e.Family) || !revisionPattern.MatchString(e.Revision) || e.Revision == "v0000" || e.Manifest != e.Family+"/handoff.yaml" {
		return errors.New("non-canonical active handoff identity or manifest path")
	}
	if e.State != "active" && e.State != "ready-for-production" && e.State != "production" {
		return errors.New("invalid active handoff state")
	}
	if !updateTimestamp(e.ActivatedAt) {
		return errors.New("invalid handoff activation timestamp")
	}
	return nil
}

func inspectFamily(store string, e Entry) (string, error) {
	dir := filepath.Join(store, e.Family)
	if err := safeDirectory(dir); err != nil {
		return "", err
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	expected := map[string]bool{"handoff.yaml": false, "START.md": false, "context.md": false, "work-item.md": false, "scope.md": false, "history": false}
	for _, item := range items {
		if _, ok := expected[item.Name()]; ok {
			expected[item.Name()] = true
		} else if item.Name() != "implementation-updates.md" {
			return "", errors.New("unmanaged handoff family entry")
		}
		if item.Type()&os.ModeSymlink != 0 {
			return "", errors.New("symlink in handoff family")
		}
	}
	for name, present := range expected {
		if !present {
			return "", fmt.Errorf("missing %s", name)
		}
	}
	raw, err := readFile(filepath.Join(dir, "handoff.yaml"))
	if err != nil {
		return "", err
	}
	var m manifest
	if err = decode(raw, &m); err != nil {
		return "", err
	}
	if m.Schema != 2 || m.ID != e.ID || m.Family != e.Family || m.Revision != e.Revision || strings.TrimSpace(m.Source.Investigation) == "" {
		return "", errors.New("manifest identity or schema mismatch")
	}
	if !updateTimestamp(m.Source.UpdatedAt) || !regexp.MustCompile(`^S-[0-9]{3,}$`).MatchString(m.Source.Story) {
		return "", errors.New("invalid manifest source identity")
	}
	if err := validateStoredWorkItem(m.WorkItem); err != nil {
		return "", err
	}
	if len(m.Files) != 4 {
		return "", errors.New("manifest must index exactly four documents")
	}
	for _, name := range []string{"START.md", "context.md", "work-item.md", "scope.md"} {
		record, ok := m.Files[name]
		if !ok || !hashPattern.MatchString(record.SHA) {
			return "", fmt.Errorf("invalid file digest: %s", name)
		}
		if name != "START.md" && !hashPattern.MatchString(record.SemanticSHA) {
			return "", fmt.Errorf("invalid semantic digest: %s", name)
		}
		data, err := readFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		if digest(data) != record.SHA {
			return "", fmt.Errorf("file digest mismatch: %s", name)
		}
		if name != "START.md" {
			if !utf8.Valid(data) || digest(semanticBytes(data)) != record.SemanticSHA {
				return "", fmt.Errorf("semantic digest mismatch: %s", name)
			}
		}
	}
	if m.History.Path != "history/"+e.Revision+".md" || !hashPattern.MatchString(m.History.SHA) {
		return "", errors.New("invalid history pointer")
	}
	if err = checkHistory(dir, e, m.History.SHA); err != nil {
		return "", err
	}
	return m.Source.Investigation, nil
}

func checkHistory(dir string, e Entry, expectedHash string) error {
	history := filepath.Join(dir, "history")
	if err := safeDirectory(history); err != nil {
		return err
	}
	n, _ := strconv.Atoi(e.Revision[1:])
	items, err := os.ReadDir(history)
	if err != nil {
		return err
	}
	if len(items) != n {
		return errors.New("non-contiguous or extra history events")
	}
	for i := n; i >= 1; i-- {
		revision := fmt.Sprintf("v%04d", i)
		raw, err := readFile(filepath.Join(history, revision+".md"))
		if err != nil {
			return err
		}
		if digest(raw) != expectedHash {
			return errors.New("broken history hash chain")
		}
		normalized := strings.ReplaceAll(strings.TrimPrefix(string(raw), "\ufeff"), "\r\n", "\n")
		if !strings.HasPrefix(normalized, "---\n") {
			return errors.New("missing history frontmatter")
		}
		end := strings.Index(normalized[4:], "\n---\n")
		if end < 0 {
			return errors.New("unterminated history frontmatter")
		}
		var event historyEvent
		if err = decode([]byte(normalized[4:4+end]), &event); err != nil {
			return err
		}
		if event.Schema != 1 || event.ID != e.ID || event.Revision != revision || event.Changed == nil || event.Unchanged == nil {
			return errors.New("history identity mismatch")
		}
		if i == 1 {
			if event.Previous != nil || event.PreviousSHA != nil {
				return errors.New("baseline history does not terminate chain")
			}
		} else {
			if event.Previous == nil || *event.Previous != fmt.Sprintf("v%04d", i-1) || event.PreviousSHA == nil || !hashPattern.MatchString(*event.PreviousSHA) {
				return errors.New("invalid previous history pointer")
			}
			expectedHash = *event.PreviousSHA
		}
	}
	return nil
}

func noTransaction(store string) error {
	for _, name := range []string{".APPLY.transaction", ".APPLY.transaction.prepare"} {
		if _, err := os.Lstat(filepath.Join(store, name)); err == nil {
			return errors.New("pending handoff transaction; recover with the existing lifecycle tool before inspecting")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func safeDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("unsafe directory: %s", path)
	}
	return nil
}

func readFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, errors.New("file changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxStoredFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxStoredFileBytes {
		return nil, fmt.Errorf("file exceeds %d bytes: %s", maxStoredFileBytes, path)
	}
	return raw, nil
}

func decode(raw []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple YAML documents are not allowed")
		}
		return err
	}
	return nil
}

func digest(raw []byte) string { value := sha256.Sum256(raw); return hex.EncodeToString(value[:]) }

func semanticBytes(raw []byte) []byte {
	s := norm.NFC.String(strings.TrimPrefix(string(raw), "\ufeff"))
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
