package handoff

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func ValidateFamilyRepository(vault, remote, path string) (map[string]any, error) {
	return validateFamilyRepository(vault, remote, path, false)
}
func validateFamilyRepository(vault, remote, path string, skipTransaction bool) (map[string]any, error) {
	target, normalized, branch, err := resolveTarget(vault, remote, path)
	if err != nil {
		return nil, err
	}
	desired, err := prepareInstructions(target)
	if err != nil {
		return nil, err
	}
	for rel, want := range desired {
		got, e := optionalFile(filepath.Join(target, rel))
		if e != nil {
			return nil, e
		}
		if !bytes.Equal(got, want) {
			return nil, errors.New("managed handoff instructions are outdated")
		}
	}
	ignore, err := prepareIgnore(target)
	if err != nil {
		return nil, err
	}
	current, err := optionalFile(filepath.Join(target, ".gitignore"))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(ignore, current) {
		return nil, errors.New("handoff ignore rule is missing")
	}
	if err = verifyIgnored(target); err != nil {
		return nil, err
	}
	a, err := registry(target, skipTransaction)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"status": "inactive", "target": map[string]string{"path": target, "remote": normalized, "branch": branch}}
	if len(a.Entries) == 0 {
		return result, nil
	}
	results := []map[string]any{}
	for i, e := range a.Entries {
		dir := filepath.Join(target, storeName, e.Family)
		m, err := loadManifest(filepath.Join(dir, "handoff.yaml"))
		if err != nil {
			return nil, err
		}
		if m.Repository.Remote != normalized {
			return nil, errors.New("manifest repository identity differs from worktree")
		}
		if i == 0 {
			_, _, token := identity(m.WorkItem, normalized)
			if !strings.Contains(branch, "/") || !strings.HasPrefix(branch[strings.LastIndex(branch, "/")+1:], token+"-") {
				return nil, errors.New("worktree branch differs from anchor handoff")
			}
		}
		updates, err := validateUpdates(filepath.Join(dir, "implementation-updates.md"))
		if err != nil {
			return nil, err
		}
		fingerprint, err := ClosureFingerprint(target, e.Family)
		if err != nil {
			return nil, err
		}
		results = append(results, map[string]any{"handoff_id": e.ID, "family": e.Family, "revision": e.Revision, "state": e.State, "materialized_at": m.UpdatedAt, "manifest": filepath.Join(dir, "handoff.yaml"), "history": m.History.Path, "implementation_updates": updates, "closure_fingerprint": fingerprint})
	}
	result["status"] = "valid"
	result["investigation_id"] = a.Investigation
	result["handoffs"] = results
	return result, nil
}

func verifyIgnored(target string) error {
	if _, err := gitCall(target, "check-ignore", "--quiet", "--", storeName+"/ACTIVE.yaml"); err != nil {
		return errors.New("handoff store is not effectively ignored")
	}
	tracked, err := gitCall(target, "ls-files", "--", storeName)
	if err != nil {
		return err
	}
	if tracked != "" {
		return errors.New("handoff store contains tracked paths")
	}
	return nil
}

func ClosureFingerprint(target, family string) (string, error) {
	if !familyPattern.MatchString(family) {
		return "", errors.New("invalid handoff family")
	}
	head := commit(target, "HEAD")
	if head == "" {
		return "", errors.New("HEAD unavailable")
	}
	status, err := gitRaw(target, "status", "--porcelain=v1", "-z", "--branch", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return "", err
	}
	diff, err := gitDigest(target, "diff", "--no-ext-diff", "--binary", "--submodule=diff", "HEAD", "--")
	if err != nil {
		return "", err
	}
	paths, err := gitRaw(target, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	untracked := map[string]any{}
	if len(paths) > 0 {
		entries := strings.Split(strings.TrimSuffix(string(paths), "\x00"), "\x00")
		if len(entries) > 10000 {
			return "", errors.New("too many untracked paths for closure snapshot")
		}
		for _, rel := range entries {
			if filepath.IsAbs(rel) || strings.Contains(rel, "../") || rel == ".." {
				return "", errors.New("unsafe Git path")
			}
			p := filepath.Join(target, filepath.FromSlash(rel))
			info, e := os.Lstat(p)
			if e != nil {
				return "", e
			}
			if info.Mode()&os.ModeSymlink != 0 {
				link, e := os.Readlink(p)
				if e != nil {
					return "", e
				}
				untracked[rel] = map[string]string{"symlink": link}
			} else {
				data, e := fileDigest(p)
				if e != nil {
					return "", e
				}
				untracked[rel] = data
			}
		}
	}
	state, err := stateFingerprint(target, storeName+"/"+family, storeName+"/ACTIVE.yaml")
	if err != nil {
		return "", err
	}
	after, err := gitRaw(target, "status", "--porcelain=v1", "-z", "--branch", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return "", err
	}
	if !bytes.Equal(after, status) || commit(target, "HEAD") != head {
		return "", errors.New("repository changed during closure snapshot")
	}
	raw, _ := json.Marshal(map[string]any{"schema": 1, "state": state, "head": head, "status": digest(status), "diff": diff, "untracked": untracked})
	return digest(raw), nil
}

func gitDigest(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	h := sha256.New()
	cmd.Stdout = h
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("closure requires regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, opened) {
		return "", errors.New("closure file changed while opening")
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return "", err
	}
	after, err := f.Stat()
	if err != nil {
		return "", err
	}
	if opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return "", errors.New("closure file changed while hashing")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func SetState(vault, remote, path, id, state, closure, approved string) (map[string]any, error) {
	var output map[string]any
	action := func() error {
		if state != "active" && state != "ready-for-production" && state != "production" {
			return errors.New("unsupported handoff state")
		}
		target, normalized, branch, err := resolveTarget(vault, remote, path)
		if err != nil {
			return err
		}
		a, err := registry(target, false)
		if err != nil {
			return err
		}
		index := -1
		for i, e := range a.Entries {
			if e.ID == id {
				index = i
			}
		}
		if index < 0 {
			return errors.New("handoff not found")
		}
		entry := a.Entries[index]
		// Validate all stored records and branch binding, permitting policy refresh.
		for i, e := range a.Entries {
			m, err := loadManifest(filepath.Join(target, storeName, e.Family, "handoff.yaml"))
			if err != nil {
				return err
			}
			if m.Repository.Remote != normalized {
				return errors.New("manifest repository mismatch")
			}
			if _, err = validateUpdates(filepath.Join(target, storeName, e.Family, "implementation-updates.md")); err != nil {
				return err
			}
			if i == 0 {
				_, _, token := identity(m.WorkItem, normalized)
				if !strings.Contains(branch, "/") || !strings.HasPrefix(branch[strings.LastIndex(branch, "/")+1:], token+"-") {
					return errors.New("branch identity mismatch")
				}
			}
		}
		if state == "ready-for-production" && entry.State == "production" {
			return errors.New("ready-for-production requires an active handoff")
		}
		if state == "production" && entry.State != "ready-for-production" && entry.State != "production" {
			return errors.New("production requires ready-for-production")
		}
		if state == "ready-for-production" && entry.State == "active" {
			observed, e := ClosureFingerprint(target, entry.Family)
			if e != nil {
				return e
			}
			if !hashPattern.MatchString(closure) || subtle.ConstantTimeCompare([]byte(observed), []byte(closure)) != 1 {
				return errors.New("current closure fingerprint is required")
			}
		} else if state != "ready-for-production" && closure != "" {
			return errors.New("only ready-for-production accepts closure fingerprint")
		}
		desired, err := prepareInstructions(target)
		if err != nil {
			return err
		}
		writes := map[string][]byte{}
		policy := map[string]string{}
		for rel, data := range desired {
			policy[rel] = digest(data)
			current, e := optionalFile(filepath.Join(target, rel))
			if e != nil {
				return e
			}
			if !bytes.Equal(current, data) {
				writes[rel] = data
			}
		}
		change := "noop"
		if len(writes) > 0 {
			change = "refresh-policy"
		}
		if entry.State != state {
			change = "set-state"
			a.Entries[index].State = state
			if state == "active" {
				a.Entries[index].ActivatedAt = time.Now().UTC().Format(time.RFC3339)
			}
			raw, e := marshalRegistry(a)
			if e != nil {
				return e
			}
			writes[storeName+"/ACTIVE.yaml"] = raw
		}
		fingerprint, err := stateFingerprint(target, "AGENTS.md", "CLAUDE.md", "AGENTS.override.md", storeName)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"operation": "set-state", "target": target, "remote": normalized, "id": id, "state": state, "closure": closure, "snapshot": fingerprint, "policy": policy})
		token := digest(payload)
		output = map[string]any{"status": "planned", "operation": "set-state", "handoff_id": id, "state": state, "action": change, "target": map[string]string{"path": target, "remote": normalized, "branch": branch}, "plan_token": token}
		if approved == "" {
			return nil
		}
		if subtle.ConstantTimeCompare([]byte(approved), []byte(token)) != 1 {
			return errors.New("plan_stale: state inputs changed")
		}
		if change == "noop" {
			output["status"] = "unchanged"
			return nil
		}
		if err = applyTransactionLocked(target, approved, writes, nil); err != nil {
			return err
		}
		output["status"] = "state-updated"
		return nil
	}
	var err error
	if approved != "" {
		target, _, _, e := resolveTarget(vault, remote, path)
		if e != nil {
			return nil, e
		}
		err = WithStoreLock(target, func() error {
			if e := recoverLocked(path, approved); e != nil {
				return e
			}
			return action()
		})
	} else {
		err = action()
	}
	return output, err
}
