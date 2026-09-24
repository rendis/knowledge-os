// Package investigation manages portable investigation records and reviewed
// artifacts, with compare-and-swap writes and recoverable filesystem transactions.
package investigation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var ErrUnknownCommand = errors.New("unknown investigation command")

// Entry binds a path, entry kind, permission bits and (for files) bytes.
type Entry struct {
	Mode   uint32 `json:"mode"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
	Type   string `json:"type"`
}

// Snapshot is compatible with investigation-case.py case_tree_snapshot hashes,
// including Python's ASCII-escaped canonical JSON. Files are streamed so artifact
// size does not determine memory use. Symlinks and special files are rejected.
func Snapshot(dir string) (string, []Entry, error) {
	info, err := os.Lstat(dir)
	if err != nil {
		return "", nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", nil, errors.New("case snapshot requires a real directory")
	}
	var entries = make([]Entry, 0)
	err = filepath.WalkDir(dir, func(path string, de fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if !utf8.ValidString(rel) {
			return errors.New("case path is not valid UTF-8")
		}
		st, err := de.Info()
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("case snapshot contains a symlink: %s", rel)
		}
		e := Entry{Path: filepath.ToSlash(rel), Mode: permissionBits(st.Mode())}
		if st.IsDir() {
			e.Type = "directory"
		} else {
			if !st.Mode().IsRegular() {
				return fmt.Errorf("case snapshot contains a special file: %s", rel)
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			opened, err := f.Stat()
			if err != nil {
				f.Close()
				return err
			}
			if !os.SameFile(st, opened) {
				f.Close()
				return fmt.Errorf("case changed while reading: %s", rel)
			}
			h := sha256.New()
			_, copyErr := io.Copy(h, f)
			end, statErr := f.Stat()
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if statErr != nil {
				return statErr
			}
			if closeErr != nil {
				return closeErr
			}
			if opened.Size() != end.Size() || opened.ModTime() != end.ModTime() || opened.Mode() != end.Mode() {
				return fmt.Errorf("case changed while reading: %s", rel)
			}
			e.Type = "file"
			e.SHA256 = hex.EncodeToString(h.Sum(nil))
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	// pathlib sorts by path components, not by a slash-joined string.
	sort.Slice(entries, func(i, j int) bool { return pathLess(entries[i].Path, entries[j].Path) })
	b, err := pythonCanonical(entries, true, false)
	if err != nil {
		return "", nil, err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), entries, nil
}

func permissionBits(m fs.FileMode) uint32 {
	v := uint32(m.Perm())
	if m&os.ModeSetuid != 0 {
		v |= 04000
	}
	if m&os.ModeSetgid != 0 {
		v |= 02000
	}
	if m&os.ModeSticky != 0 {
		v |= 01000
	}
	return v
}

func pathLess(a, b string) bool {
	x, y := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}

// Run exposes only operations whose guarantees are implemented. It accepts
// --root before or after the verb, matching the future unified CLI convention.
func Run(args []string, out io.Writer) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			verb := ""
			for _, v := range args {
				if _, ok := commandOptions[v]; ok {
					verb = v
					break
				}
			}
			return help(out, verb)
		}
	}
	o := options{}
	verb := ""
	valued := map[string]bool{}
	for _, k := range []string{"root", "case-dir", "id", "to", "reason", "source", "blocked-on", "expected-public-sha256", "timestamp", "decision", "limitations", "evidence", "title", "objective", "dedupe-key", "purpose", "vault-outcome", "learning-outcome", "request-summary", "source-ref", "source-type", "requester-role", "export-intent", "visibility", "public-candidate", "private-root", "private-candidate", "expected-private-sha256", "target", "private-target", "candidate-dir", "expected-tree-sha256", "expected-candidate-tree-sha256", "retain-local", "observation", "expected-story-sha256", "dependency-review", "absorption-review", "summary", "snapshot-commit", "destination", "canonical", "retire", "expected-canonical-sha256", "expected-retire-sha256"} {
		valued[k] = true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") {
			k := strings.TrimPrefix(a, "--")
			if k == "delete-private" || k == "authorized" {
				o[k] = []string{"true"}
				continue
			}
			if !valued[k] {
				return fmt.Errorf("unknown option %s", strconv.Quote(a))
			}
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			i++
			o[k] = append(o[k], args[i])
		} else {
			if verb != "" {
				return errors.New("unexpected positional argument")
			}
			verb = a
		}
	}
	if !contains([]string{"snapshot", "load", "list", "validate", "transition", "close", "open", "save", "save-resources", "publish", "bind", "retire", "consolidate"}, verb) {
		return fmt.Errorf("%w: %s", ErrUnknownCommand, verb)
	}
	allowed := " " + commandOptions[verb] + " "
	for k, values := range o {
		if k != "root" && !strings.Contains(allowed, " "+k+" ") {
			return fmt.Errorf("--%s is not valid for %s", k, verb)
		}
		if len(values) > 1 && !contains([]string{"source-ref", "target", "private-target", "evidence", "destination", "retain-local"}, k) {
			return fmt.Errorf("--%s must be supplied once", k)
		}
	}
	root := o.get("root")
	if root != "" {
		var err error
		root, err = filepath.Abs(root)
		if err != nil {
			return err
		}
		if verb == "open" {
			if filepath.Base(root) != "investigations" {
				return errors.New("root must be named investigations")
			}
			if err = os.Mkdir(root, 0755); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
		}
		st, err := os.Lstat(root)
		if err != nil {
			return err
		}
		if filepath.Base(root) != "investigations" || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("root must be a real investigations directory")
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return err
		}
	}
	if verb == "snapshot" {
		if o.get("case-dir") == "" {
			return errors.New("snapshot requires --case-dir")
		}
		hash, entries, err := Snapshot(o.get("case-dir"))
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(map[string]any{"status": "snapshotted", "tree_sha256": hash, "files": entries})
	}
	if root == "" {
		return errors.New("--root is required")
	}
	var result any
	var err error
	if verb == "consolidate" {
		result, err = consolidateCase(root, o)
	} else if verb == "retire" {
		result, err = retireCase(root, o)
	} else if verb == "bind" {
		result, err = bindCase(root, o)
	} else if verb == "publish" {
		result, err = publishCase(root, o)
	} else if verb == "save-resources" {
		result, err = saveResources(root, o)
	} else if verb == "save" {
		result, err = saveCase(root, o)
	} else if verb == "open" {
		result, err = openCase(root, o)
	} else if verb == "transition" || verb == "close" {
		result, err = lifecycle(root, verb, o)
	} else {
		result, err = stableRead(root, func() (any, error) {
			switch verb {
			case "list":
				return listCases(root)
			case "validate":
				return validationResult(root)
			default:
				return loadCase(root, o.get("id"))
			}
		})
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}
