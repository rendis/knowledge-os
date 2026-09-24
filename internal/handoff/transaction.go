package handoff

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/gofrs/flock"
)

const transactionName = ".APPLY.transaction"
const nativeJournalFormat = "vaultctl-handoff-transaction-1"

type transactionImage struct {
	Path        string `json:"path"`
	Before      []byte `json:"before"`
	Desired     []byte `json:"desired"`
	BeforeHash  string `json:"before_hash"`
	DesiredHash string `json:"desired_hash"`
	Mode        uint32 `json:"mode"`
}
type transactionJournal struct {
	Format             string             `json:"format"`
	Token              string             `json:"token"`
	Images             []transactionImage `json:"images"`
	CreatedDirectories []string           `json:"created_directories"`
}

// WithStoreLock serializes planning checks, recovery and application. The lock
// file must never be unlinked: other processes may already hold its inode.
func WithStoreLock(root string, fn func() error) error {
	if err := safeDirectory(root); err != nil {
		return err
	}
	store := filepath.Join(root, storeName)
	if err := os.Mkdir(store, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := safeDirectory(store); err != nil {
		return err
	}
	path := filepath.Join(store, ".ACTIVE.lock")
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return errors.New("unsafe handoff lock")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lock := flock.New(path)
	ok, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("handoff store is locked; retry later")
	}
	defer lock.Close()
	return fn()
}

func ApplyTransaction(root, token string, writes map[string][]byte) error {
	return WithStoreLock(root, func() error { return applyTransactionLocked(root, token, writes, nil) })
}
func Recover(root, token string) error {
	return WithStoreLock(root, func() error { return recoverLocked(root, token) })
}

func transactionTarget(root, rel string) (string, error) {
	if rel == "" || strings.Contains(rel, "\\") || filepath.IsAbs(rel) || filepath.ToSlash(filepath.Clean(rel)) != rel {
		return "", errors.New("invalid transaction path")
	}
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if part == "." || part == ".." || part == "" {
			return "", errors.New("invalid transaction path")
		}
	}
	allowed := rel == ".gitignore" || rel == "AGENTS.md" || rel == "CLAUDE.md" || rel == storeName+"/ACTIVE.yaml"
	if len(parts) >= 3 && parts[0] == storeName && familyPattern.MatchString(parts[1]) {
		allowed = true
	}
	if !allowed {
		return "", fmt.Errorf("unmanaged transaction path: %s", rel)
	}
	current := root
	if err := safeDirectory(current); err != nil {
		return "", err
	}
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		if err := safeDirectory(current); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return "", fmt.Errorf("unsafe transaction target: %s", rel)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}
func imageHash(raw []byte) string {
	if raw == nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func optionalImage(path string) ([]byte, error) {
	raw, err := readFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return raw, err
}
func syncTransactionDir(path string) error {
	// Windows does not expose directory fsync through os.File.Sync.
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func writeTransactionFile(path string, raw []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".handoff-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode.Perm()); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return syncTransactionDir(filepath.Dir(path))
}

func applyTransactionLocked(root, token string, writes map[string][]byte, verify func() error) error {
	if !hashPattern.MatchString(token) {
		return errors.New("invalid transaction token")
	}
	if err := noTransaction(filepath.Join(root, storeName)); err != nil {
		return err
	}
	if len(writes) == 0 {
		if verify != nil {
			return verify()
		}
		return nil
	}
	if len(writes) > 128 {
		return errors.New("too many transaction files")
	}
	journal := transactionJournal{Format: nativeJournalFormat, Token: token}
	paths := make([]string, 0, len(writes))
	for path := range writes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	// ACTIVE is the last visible authority change.
	sort.SliceStable(paths, func(i, j int) bool {
		return paths[i] != storeName+"/ACTIVE.yaml" && paths[j] == storeName+"/ACTIVE.yaml"
	})
	for _, rel := range paths {
		path, err := transactionTarget(root, rel)
		if err != nil {
			return err
		}
		before, err := optionalImage(path)
		if err != nil {
			return err
		}
		desired := writes[rel]
		if desired == nil || len(desired) > maxFileBytes {
			return errors.New("invalid desired file image")
		}
		mode := os.FileMode(0600)
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
		journal.Images = append(journal.Images, transactionImage{rel, before, desired, imageHash(before), imageHash(desired), uint32(mode)})
	}
	dirs := map[string]bool{}
	for _, im := range journal.Images {
		for dir := filepath.Dir(im.Path); dir != "."; dir = filepath.Dir(dir) {
			if _, err := os.Lstat(filepath.Join(root, dir)); errors.Is(err, os.ErrNotExist) {
				dirs[filepath.ToSlash(dir)] = true
			} else if err != nil {
				return err
			}
		}
	}
	for dir := range dirs {
		journal.CreatedDirectories = append(journal.CreatedDirectories, dir)
	}
	sort.Slice(journal.CreatedDirectories, func(i, j int) bool { return len(journal.CreatedDirectories[i]) > len(journal.CreatedDirectories[j]) })
	store := filepath.Join(root, storeName)
	stage, err := os.MkdirTemp(store, ".native-transaction-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	raw, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	if err = writeTransactionFile(filepath.Join(stage, "transaction.json"), raw, 0600); err != nil {
		return err
	}
	txn := filepath.Join(store, transactionName)
	if err = os.Rename(stage, txn); err != nil {
		return err
	}
	if err = syncTransactionDir(store); err != nil {
		return err
	}
	for _, im := range journal.Images {
		path, e := transactionTarget(root, im.Path)
		if e == nil {
			var current []byte
			current, e = optionalImage(path)
			if e == nil && imageHash(current) != im.BeforeHash {
				e = errors.New("planned content changed before transaction write")
			}
		}
		if e == nil {
			e = writeTransactionFile(path, im.Desired, os.FileMode(im.Mode))
		}
		if e != nil {
			return fmt.Errorf("handoff apply interrupted; recover exact token: %w", e)
		}
	}
	if verify != nil {
		if err = verify(); err != nil {
			recoveryErr := recoverLocked(root, token)
			if recoveryErr != nil {
				return fmt.Errorf("verification failed: %v; rollback failed: %w", err, recoveryErr)
			}
			return err
		}
	}
	if err = writeTransactionFile(filepath.Join(txn, "COMMITTED"), []byte("committed\n"), 0600); err != nil {
		return err
	}
	return retireTransaction(store, txn)
}

// Move the completed journal out of the pending location before deletion. A
// process exit during RemoveAll must never leave a half-deleted pending journal.
func retireTransaction(store, txn string) error {
	tomb, err := os.MkdirTemp(store, ".native-completed-")
	if err != nil {
		return err
	}
	if err = os.Remove(tomb); err != nil {
		return err
	}
	if err = os.Rename(txn, tomb); err != nil {
		return err
	}
	if err = syncTransactionDir(store); err != nil {
		return err
	}
	return os.RemoveAll(tomb)
}

func loadNativeJournal(txn string) (transactionJournal, error) {
	var j transactionJournal
	if err := safeDirectory(txn); err != nil {
		return j, err
	}
	path := filepath.Join(txn, "transaction.json")
	st, err := os.Lstat(path)
	if err != nil {
		return j, err
	}
	if !st.Mode().IsRegular() || st.Size() > 400<<20 {
		return j, errors.New("unsafe or oversized transaction journal")
	}
	f, err := os.Open(path)
	if err != nil {
		return j, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(st, opened) {
		return j, errors.New("journal changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 400<<20))
	if err != nil {
		return j, err
	}
	var format struct {
		Format string `json:"format"`
	}
	if err = json.Unmarshal(raw, &format); err != nil {
		return j, err
	}
	if format.Format != nativeJournalFormat {
		if format.Format != "" {
			return j, errors.New("unknown handoff journal format")
		}
		return loadLegacyJournal(txn, raw)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&j); err != nil {
		return j, err
	}
	if !hashPattern.MatchString(j.Token) || len(j.Images) == 0 || len(j.Images) > 128 {
		return j, errors.New("invalid transaction journal")
	}
	seen := map[string]bool{}
	for _, im := range j.Images {
		if seen[im.Path] || im.Desired == nil || len(im.Before) > maxFileBytes || len(im.Desired) > maxFileBytes || imageHash(im.Before) != im.BeforeHash || imageHash(im.Desired) != im.DesiredHash || im.Mode > 0777 {
			return j, errors.New("invalid transaction image")
		}
		seen[im.Path] = true
	}
	for _, dir := range j.CreatedDirectories {
		// Only ancestors of declared images may be removed, and only when empty.
		valid := false
		for _, im := range j.Images {
			if strings.HasPrefix(im.Path, dir+"/") && dir != storeName {
				valid = true
			}
		}
		if !valid {
			return j, errors.New("invalid transaction directory")
		}
	}
	return j, nil
}

func recoverLocked(root, token string) error {
	store := filepath.Join(root, storeName)
	txn := filepath.Join(store, transactionName)
	if err := cleanupLegacyPrepare(store, token); err != nil {
		return err
	}
	if _, err := os.Lstat(txn); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	// Python cleanup could be interrupted after deleting metadata but before
	// deleting COMMITTED. No rollback is needed after this durable marker.
	if err := safeDirectory(txn); err != nil {
		return err
	}
	if marker, err := optionalImage(filepath.Join(txn, "COMMITTED")); err != nil {
		return err
	} else if string(marker) == "committed\n" {
		if _, err := os.Lstat(filepath.Join(txn, "transaction.json")); errors.Is(err, os.ErrNotExist) {
			entries, err := os.ReadDir(txn)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() || (entry.Name() != "COMMITTED" && !regexp.MustCompile(`^[0-9]{4}\.bak$`).MatchString(entry.Name())) {
					return errors.New("unexpected committed legacy journal content")
				}
			}
			return retireTransaction(store, txn)
		} else if err != nil {
			return err
		}
	}
	j, err := loadNativeJournal(txn)
	if err != nil {
		return err
	}
	if token != j.Token {
		return errors.New("only the exact interrupted plan token may recover this worktree")
	}
	if marker, err := optionalImage(filepath.Join(txn, "COMMITTED")); err != nil {
		return err
	} else if marker != nil {
		if string(marker) != "committed\n" {
			return errors.New("invalid commit marker")
		}
		return retireTransaction(store, txn)
	}
	check := func(im transactionImage) (string, error) {
		path, err := transactionTarget(root, im.Path)
		if err != nil {
			return "", err
		}
		current, err := optionalImage(path)
		if err != nil {
			return "", err
		}
		hash := imageHash(current)
		if hash != im.BeforeHash && (im.DesiredHash == "" || hash != im.DesiredHash) {
			return "", fmt.Errorf("recovery conflict; preserve modified file: %s", im.Path)
		}
		return path, nil
	}
	for _, im := range j.Images {
		if _, err = check(im); err != nil {
			return err
		}
	}
	for _, im := range j.Images {
		path, err := check(im)
		if err != nil {
			return err
		}
		if im.Before != nil {
			err = writeTransactionFile(path, im.Before, os.FileMode(im.Mode))
		} else {
			err = os.Remove(path)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			if err == nil {
				if _, e := os.Stat(filepath.Dir(path)); e == nil {
					err = syncTransactionDir(filepath.Dir(path))
				} else if !errors.Is(e, os.ErrNotExist) {
					err = e
				}
			}
		}
		if err != nil {
			return err
		}
	}
	for _, dir := range j.CreatedDirectories {
		path := filepath.Join(root, filepath.FromSlash(dir))
		if err := safeDirectory(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("recovery preserved directory with unexpected content: %w", err)
		}
		if err := syncTransactionDir(filepath.Dir(path)); err != nil {
			return err
		}
	}
	return retireTransaction(store, txn)
}

// Python schema-1 journals store backup images and a hash of the prospective
// write (recorded before the write). Convert these to the same rollback model.
func loadLegacyJournal(txn string, raw []byte) (transactionJournal, error) {
	var legacy struct {
		Schema        int    `json:"schema-version"`
		Token         string `json:"plan-token"`
		Bundle        string `json:"bundle-fingerprint"`
		ID            string `json:"handoff-id"`
		Family        string `json:"family"`
		FamilyExisted *bool  `json:"family-existed"`
		Paths         []struct {
			Path      string  `json:"path"`
			Existed   *bool   `json:"existed"`
			Backup    *string `json:"backup"`
			Postimage *string `json:"postimage-sha256"`
		} `json:"paths"`
	}
	var j transactionJournal
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&legacy); err != nil {
		return j, fmt.Errorf("invalid legacy journal: %w", err)
	}
	if legacy.Schema != 1 || !hashPattern.MatchString(legacy.Token) || !hashPattern.MatchString(legacy.Bundle) || !hashPattern.MatchString(legacy.ID) || !familyPattern.MatchString(legacy.Family) || legacy.FamilyExisted == nil || len(legacy.Paths) == 0 || len(legacy.Paths) > 128 {
		return j, errors.New("invalid legacy journal identity")
	}
	// Null is meaningful, but an absent field signals damaged metadata.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return j, err
	}
	var records []map[string]json.RawMessage
	if err := json.Unmarshal(fields["paths"], &records); err != nil {
		return j, err
	}
	marker, markerErr := optionalImage(filepath.Join(txn, "COMMITTED"))
	if markerErr != nil {
		return j, markerErr
	}
	committed := string(marker) == "committed\n"
	j.Format = "legacy-python-1"
	j.Token = legacy.Token
	root := filepath.Dir(filepath.Dir(txn))
	seen := map[string]bool{}
	backups := map[string]bool{}
	dirs := map[string]bool{}
	prefix := storeName + "/" + legacy.Family + "/"
	for i, record := range legacy.Paths {
		if len(records[i]) != 4 || record.Existed == nil || seen[record.Path] {
			return j, errors.New("invalid legacy path record")
		}
		seen[record.Path] = true
		if record.Path != ".gitignore" && record.Path != "AGENTS.md" && record.Path != "CLAUDE.md" && record.Path != storeName+"/ACTIVE.yaml" && !strings.HasPrefix(record.Path, prefix) {
			return j, errors.New("invalid legacy family path")
		}
		path, err := transactionTarget(root, record.Path)
		if err != nil {
			return j, err
		}
		im := transactionImage{Path: record.Path, Mode: 0600}
		if *record.Existed != (record.Backup != nil) {
			return j, errors.New("legacy backup state mismatch")
		}
		if !*legacy.FamilyExisted && strings.HasPrefix(record.Path, prefix) && *record.Existed {
			return j, errors.New("new legacy family contains existing file")
		}
		if record.Backup != nil {
			if !regexp.MustCompile(`^[0-9]{4}\.bak$`).MatchString(*record.Backup) || backups[*record.Backup] {
				return j, errors.New("invalid legacy backup")
			}
			backups[*record.Backup] = true
			im.Before, err = readFile(filepath.Join(txn, *record.Backup))
			if committed && errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			if err != nil {
				return j, err
			}
			im.BeforeHash = imageHash(im.Before)
		}
		if record.Postimage != nil {
			if !hashPattern.MatchString(*record.Postimage) {
				return j, errors.New("invalid legacy postimage")
			}
			im.DesiredHash = *record.Postimage
		}
		if st, err := os.Stat(path); err == nil {
			im.Mode = uint32(st.Mode().Perm())
		}
		j.Images = append(j.Images, im)
		if !*legacy.FamilyExisted && strings.HasPrefix(record.Path, prefix) {
			for dir := filepath.ToSlash(filepath.Dir(record.Path)); dir != storeName; dir = filepath.ToSlash(filepath.Dir(dir)) {
				dirs[dir] = true
			}
		}
	}
	sort.SliceStable(j.Images, func(i, k int) bool {
		return j.Images[i].Path != storeName+"/ACTIVE.yaml" && j.Images[k].Path == storeName+"/ACTIVE.yaml"
	})
	if !*legacy.FamilyExisted && !committed {
		familyPath := filepath.Join(root, storeName, legacy.Family)
		err := filepath.WalkDir(familyPath, func(path string, entry os.DirEntry, err error) error {
			if errors.Is(err, os.ErrNotExist) && path == familyPath {
				return nil
			}
			if err != nil {
				return err
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			rel = filepath.ToSlash(rel)
			if entry.Type()&os.ModeSymlink != 0 || (entry.IsDir() && !dirs[rel]) || (!entry.IsDir() && !seen[rel]) {
				return fmt.Errorf("legacy recovery conflict; unexpected family content: %s", rel)
			}
			return nil
		})
		if err != nil {
			return j, err
		}
		for dir := range dirs {
			j.CreatedDirectories = append(j.CreatedDirectories, dir)
		}
		sort.Slice(j.CreatedDirectories, func(i, k int) bool { return len(j.CreatedDirectories[i]) > len(j.CreatedDirectories[k]) })
	}
	return j, nil
}

func cleanupLegacyPrepare(store, token string) error {
	prepare := filepath.Join(store, ".APPLY.transaction.prepare")
	if err := safeDirectory(prepare); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	// Preparation has not published a journal and cannot have mutated targets.
	// Restrict cleanup to the Python writer's own snapshot files.
	entries, err := os.ReadDir(prepare)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("unsafe legacy preparation file")
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || (name != "transaction.json" && !regexp.MustCompile(`^[0-9]{4}\.bak$`).MatchString(name)) {
			return errors.New("unexpected legacy preparation content")
		}
	}
	if raw, err := optionalImage(filepath.Join(prepare, "transaction.json")); err != nil {
		return err
	} else if raw != nil {
		j, err := loadLegacyJournal(prepare, raw)
		if err != nil {
			return err
		}
		if j.Token != token {
			return errors.New("legacy preparation token mismatch")
		}
	}
	return retireTransaction(store, prepare)
}
