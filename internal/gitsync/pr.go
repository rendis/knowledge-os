package gitsync

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"knowledge-os/internal/config"
)

type prRun struct {
	BaseBranch string `json:"base_branch"`
	Base       string `json:"base"`
	Case       string `json:"case,omitempty"`

	dir, ref string
}

func prLocation(o opts) (prRun, error) {
	if !slugRE.MatchString(o.name) {
		return prRun{}, errors.New("--name must be a lowercase slug")
	}
	dir, err := git(o.vault, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return prRun{}, err
	}
	return prRun{dir: filepath.Join(dir, "kos-sync", o.name), ref: "refs/heads/sync/" + o.name}, nil
}

func (r prRun) draft() string { return filepath.Join(r.dir, "draft") }

func upstream(vault, branch string) (string, error) {
	ref := "refs/remotes/origin/" + branch
	if _, err := git(vault, "fetch", "--no-tags", "--no-write-fetch-head", "origin", "+refs/heads/"+branch+":"+ref); err != nil {
		return "", err
	}
	return git(vault, "rev-parse", "--verify", ref+"^{commit}")
}

func startPR(o opts, out io.Writer) error {
	r, err := prLocation(o)
	if err != nil {
		return err
	}
	if o.base == "" {
		o.base = "main"
	}
	if !config.ValidBranch(o.base) || current(o.vault) != o.base {
		return fmt.Errorf("PR sync requires the vault checkout on its base branch %q", o.base)
	}
	if o.head != "" || o.message != "" || o.allowNoChange {
		return errors.New("unsupported option for sync start")
	}
	r.BaseBranch, r.Case = o.base, o.caseID
	r.Base, err = upstream(o.vault, r.BaseBranch)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.dir), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(r.dir, 0o700); err != nil {
		return fmt.Errorf("create sync run %s: %w (use sync status --name %s to resume)", o.name, err, o.name)
	}
	ready := false
	defer func() {
		if !ready {
			_ = os.RemoveAll(r.dir)
		}
	}()
	if err := os.Mkdir(r.draft(), 0o700); err != nil {
		return err
	}
	if err := exportCommit(o.vault, r.Base, r.draft()); err != nil {
		return err
	}
	var imported []string
	if r.Case != "" {
		imported, err = importPendingCase(o, r)
		if err != nil {
			return err
		}
	}
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(r.dir, "run.json"), body, 0o600); err != nil {
		return err
	}
	if _, err := git(o.vault, "update-ref", "--no-deref", r.ref, r.Base, strings.Repeat("0", len(r.Base))); err != nil {
		return err
	}
	ready = true
	return emit(out, map[string]any{"branch": "sync/" + o.name, "base": r.BaseBranch, "base_commit": r.Base, "head": r.Base,
		"draft": r.draft(), "imported": imported, "next": "edit the draft, then sync commit --name " + o.name + " with explicit --path entries"})
}

func runPR(cmd string, o opts, out io.Writer) error {
	r, err := prLocation(o)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join(r.dir, "run.json"))
	if err != nil {
		return fmt.Errorf("read PR sync run %s: %w", o.name, err)
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	if !config.ValidBranch(r.BaseBranch) || len(r.Base) != 40 && len(r.Base) != 64 || strings.Trim(r.Base, "0123456789abcdef") != "" {
		return errors.New("invalid PR sync run base")
	}
	if o.base != "" || o.allowNoChange {
		return errors.New("a named PR run uses its pinned base and requires the normal review gates")
	}
	if (cmd != "review" && o.head != "") || (cmd != "commit" && o.message != "") {
		return errors.New("--head is for review; --message is for commit")
	}
	head, err := git(o.vault, "rev-parse", "--verify", r.ref+"^{commit}")
	if err != nil {
		return err
	}
	if cmd == "review" && o.head != head {
		return fmt.Errorf("review must name the exact candidate --head %s; inspect its diff before recording a verdict", head)
	}
	o.base, o.head = r.Base, head
	if _, err := git(o.vault, "merge-base", "--is-ancestor", r.Base, head); err != nil {
		return fmt.Errorf("candidate does not descend from pinned base %s: %w", r.Base, err)
	}
	paths, err := diffPaths(o.vault, r.Base, head)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if !r.allows(path, true) {
			return fmt.Errorf("candidate changes an out-of-scope file: %s", path)
		}
	}
	switch cmd {
	case "status":
		res, err := status(o)
		if err != nil {
			return err
		}
		res["branch"], res["base"], res["base_commit"], res["head"], res["draft"], res["paths"] =
			"sync/"+o.name, r.BaseBranch, r.Base, head, r.draft(), paths
		return emit(out, res)
	case "commit":
		return checkpointPR(o, r, out)
	case "acknowledge":
		if r.Case != "" {
			return errors.New("a case-only run cannot acknowledge a repository")
		}
		o.vault = r.draft()
		return acknowledge(o, out)
	case "review":
		msg, res, path, body, err := reviewCommit(o)
		if err != nil {
			return err
		}
		newHead, err := appendCandidate(o, r, msg, func(index string) error {
			return stageBytes(o.vault, index, path, "100644", body)
		})
		if err != nil {
			return err
		}
		res["head"] = newHead
		return emit(out, res)
	case "verify", "finish":
		return verifyPR(cmd, o, r, paths, out)
	default:
		return fmt.Errorf("sync %s does not take a named PR run", cmd)
	}
}

func validPRPath(path string) bool {
	if !fs.ValidPath(path) || path == "." || !filepath.IsLocal(filepath.FromSlash(path)) || strings.ContainsAny(path, "\\:\r\n\t\x00") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}

func (r prRun) allows(path string, review bool) bool {
	if !validPRPath(path) {
		return false
	}
	if review && strings.HasPrefix(path, reviewsDir+"/") && strings.HasSuffix(path, ".json") {
		return true
	}
	if r.Case != "" {
		return strings.HasPrefix(path, "investigations/"+r.Case+"/")
	}
	return knowledgePath.MatchString(path) || strings.HasPrefix(path, "90-Meta/discovery/")
}

func checkpointPR(o opts, r prRun, out io.Writer) error {
	if len(o.paths) == 0 || strings.TrimSpace(o.message) == "" {
		return errors.New("sync commit needs --message and at least one explicit --path")
	}
	// Only sync review can create a review checkpoint.
	if strings.Contains(o.message, "Knowledge-Review:") || strings.Contains(o.message, "Knowledge-Digest:") || strings.Contains(o.message, "Knowledge-Base:") {
		return errors.New("review trailers are reserved for sync review")
	}
	for _, path := range o.paths {
		if !r.allows(path, false) {
			return fmt.Errorf("cannot checkpoint path %q in this run", path)
		}
	}
	root, err := os.OpenRoot(r.draft())
	if err != nil {
		return err
	}
	defer root.Close()
	head, err := appendCandidate(o, r, o.message, func(index string) error {
		for _, path := range o.paths {
			body, mode, err := readRegular(root, path)
			if errors.Is(err, fs.ErrNotExist) {
				_, err = indexGit(o.vault, index, nil, "update-index", "--force-remove", "--", path)
			} else if err == nil {
				err = stageBytes(o.vault, index, path, mode, body)
			}
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return emit(out, map[string]any{"head": head, "paths": o.paths})
}

func readRegular(root *os.Root, path string) ([]byte, string, error) {
	if !validPRPath(path) {
		return nil, "", fmt.Errorf("invalid file path %q", path)
	}
	parts := strings.Split(path, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return nil, "", err
		}
		if i != len(parts)-1 && info.IsDir() {
			continue
		}
		if i != len(parts)-1 || !info.Mode().IsRegular() {
			return nil, "", fmt.Errorf("path is not a regular file: %s", path)
		}
		mode := "100644"
		if info.Mode()&0o100 != 0 {
			mode = "100755"
		}
		b, err := root.ReadFile(path)
		return b, mode, err
	}
	return nil, "", errors.New("empty path")
}

func indexGit(vault, index string, input []byte, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", vault}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(b)), nil
}

func stageBytes(vault, index, path, mode string, body []byte) error {
	blob, err := indexGit(vault, index, body, "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	_, err = indexGit(vault, index, nil, "update-index", "--add", "--cacheinfo", mode, blob, path)
	return err
}

func prepareIndex(vault, dir, revision string) (string, func(), error) {
	scratch, err := os.MkdirTemp(dir, "index-")
	if err != nil {
		return "", nil, err
	}
	clean := func() { _ = os.RemoveAll(scratch) }
	index := filepath.Join(scratch, "index")
	if _, err := indexGit(vault, index, nil, "read-tree", revision); err != nil {
		clean()
		return "", nil, err
	}
	return index, clean, nil
}

func appendCandidate(o opts, r prRun, message string, stage func(string) error) (string, error) {
	index, clean, err := prepareIndex(o.vault, r.dir, o.head)
	if err != nil {
		return "", err
	}
	defer clean()
	if err := stage(index); err != nil {
		return "", err
	}
	tree, err := indexGit(o.vault, index, nil, "write-tree")
	if err != nil {
		return "", err
	}
	parentTree, err := git(o.vault, "rev-parse", o.head+"^{tree}")
	if err != nil {
		return "", err
	}
	if tree == parentTree {
		return "", errors.New("the selected files did not change the candidate")
	}
	head, err := indexGit(o.vault, index, []byte(message+"\n"), "commit-tree", tree, "-p", o.head, "-F", "-")
	if err != nil {
		return "", err
	}
	_, err = git(o.vault, "update-ref", "--no-deref", r.ref, head, o.head)
	return head, err
}

func verifyPR(cmd string, o opts, r prRun, paths []string, out io.Writer) error {
	revision := o.head
	base := r.Base
	if cmd == "finish" {
		var err error
		base, err = upstream(o.vault, r.BaseBranch)
		if err != nil {
			return err
		}
		if r.Case == "" {
			if err := checkOverlap(o.vault, paths); err != nil {
				return err
			}
		}
		if base != r.Base {
			revision, err = combinedTree(o, r, base, paths)
			if err != nil {
				return err
			}
		}
	}
	tree, clean, err := committedTree(o.vault, revision)
	if err != nil {
		return err
	}
	defer clean()
	if err := bindSnapshotSources(o.vault, tree); err != nil {
		return err
	}
	o.tree, o.treeBase = tree, base
	res, err := verify(o)
	if err != nil {
		return err
	}
	if res["ok"] != true {
		if err := emit(out, res); err != nil {
			return err
		}
		return errors.New("sync verification failed")
	}
	if cmd == "finish" {
		head, err := git(o.vault, "rev-parse", "--verify", r.ref)
		if err != nil || head != o.head {
			return errors.New("the candidate changed during verification; rerun sync finish")
		}
		res["base"], res["base_commit"], res["ready_for_pr"] = r.BaseBranch, r.Base, true
		now, err := git(o.vault, "rev-parse", "--verify", "refs/remotes/origin/"+r.BaseBranch)
		if err != nil || now != base {
			return errors.New("origin base changed during verification; rerun sync finish")
		}
		res["verified_against"], res["push_refspec"] = base, o.head+":"+r.ref
	}
	return emit(out, res)
}

// Build the would-be merged tree for checks without changing the reviewed PR head.
func combinedTree(o opts, r prRun, base string, paths []string) (string, error) {
	if _, err := git(o.vault, "merge-base", "--is-ancestor", r.Base, base); err != nil {
		return "", fmt.Errorf("origin/%s no longer descends from %s; start a new run and get a new review", r.BaseBranch, r.Base)
	}
	upstreamPaths, err := diffPaths(o.vault, r.Base, base)
	if err != nil {
		return "", err
	}
	for _, incoming := range upstreamPaths {
		for _, path := range paths {
			if incoming == path || strings.HasPrefix(path, incoming+"/") || strings.HasPrefix(incoming, path+"/") {
				return "", fmt.Errorf("origin/%s changed PR path %s; reapply it in a new run and get a new review", r.BaseBranch, path)
			}
		}
	}
	patch, err := gitOutput(o.vault, "diff", "--binary", "--no-ext-diff", "--no-textconv", "--no-renames", "--src-prefix=a/", "--dst-prefix=b/", r.Base, o.head, "--")
	if err != nil {
		return "", err
	}
	index, clean, err := prepareIndex(o.vault, r.dir, base)
	if err != nil {
		return "", err
	}
	defer clean()
	if _, err := indexGit(o.vault, index, []byte(patch), "apply", "--cached", "--whitespace=nowarn", "-"); err != nil {
		return "", err
	}
	return indexGit(o.vault, index, nil, "write-tree")
}

func checkOverlap(vault string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := []string{"--no-optional-locks", "--literal-pathspecs", "status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all", "--"}
	status, err := gitOutput(vault, append(args, paths...)...)
	if err != nil {
		return err
	}
	if status != "" {
		entries := strings.Split(strings.TrimSuffix(status, "\x00"), "\x00")
		for i := range entries {
			if len(entries[i]) < 4 {
				return errors.New("invalid Git status entry")
			}
			entries[i] = entries[i][3:]
		}
		return fmt.Errorf("pending local edits overlap PR paths: %s", strings.Join(entries, ", "))
	}
	return nil
}

func importPendingCase(o opts, r prRun) ([]string, error) {
	prefix := "investigations/" + r.Case + "/"
	if strings.Contains(r.Case, "/") || !validPRPath(prefix+"investigation.md") {
		return nil, errors.New("--case must identify an existing published investigation")
	}
	if _, err := git(o.vault, "cat-file", "-e", r.Base+":"+prefix+"investigation.md"); err != nil {
		return nil, errors.New("--case must identify a case present in the published base")
	}
	if _, err := git(o.vault, "--literal-pathspecs", "diff", "--quiet", r.Base, "HEAD", "--", prefix); err != nil {
		return nil, fmt.Errorf("local case %s differs from origin/%s; reconcile its published base before importing edits", r.Case, r.BaseBranch)
	}
	status, err := gitOutput(o.vault, "--no-optional-locks", "--literal-pathspecs", "status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all", "--", prefix)
	if err != nil {
		return nil, err
	}
	if status == "" {
		return nil, fmt.Errorf("case %s has no pending files", r.Case)
	}
	src, err := os.OpenRoot(o.vault)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	dst, err := os.OpenRoot(r.draft())
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	var imported []string
	for _, entry := range strings.Split(strings.TrimSuffix(status, "\x00"), "\x00") {
		if len(entry) < 4 {
			return nil, errors.New("invalid Git status entry")
		}
		path := entry[3:]
		if !r.allows(path, false) {
			return nil, fmt.Errorf("invalid case path %q", path)
		}
		body, mode, err := readRegular(src, path)
		if errors.Is(err, fs.ErrNotExist) {
			err = dst.Remove(path)
			if errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
		} else if err == nil {
			perm := fs.FileMode(0o644)
			if mode == "100755" {
				perm = 0o755
			}
			if err = dst.MkdirAll(filepath.Dir(path), 0o755); err == nil {
				err = dst.WriteFile(path, body, perm)
			}
			if err == nil {
				err = dst.Chmod(path, perm)
			}
		}
		if err != nil {
			return nil, err
		}
		imported = append(imported, path)
	}
	return imported, nil
}

// Source bindings come from the machine, while notes and instance.yaml come from Git.
// Use exact source roots so the live vault cannot be discovered as its own source.
func bindSnapshotSources(vault, dir string) error {
	checkouts, err := config.Checkouts(vault)
	if err != nil {
		return err
	}
	rawURL, _ := git(vault, "config", "--get", "remote.origin.url")
	vaultID, _ := config.RemoteIdentity(rawURL)
	roots := []string{}
	for _, path := range checkouts {
		if path == vault {
			continue
		}
		if vaultID != "" {
			rawURL, _ := git(path, "config", "--get", "remote.origin.url")
			id, _ := config.RemoteIdentity(rawURL)
			if id == vaultID {
				continue
			}
		}
		roots = append(roots, path)
	}
	body, err := json.Marshal(map[string]any{"version": 1, "workspace": map[string]any{"repository_roots": roots}})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, config.WorkspaceFile), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(body)
	return errors.Join(err, f.Close())
}

func committedTree(vault, revision string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "kos-sync-check-")
	if err != nil {
		return "", nil, err
	}
	clean := func() { _ = os.RemoveAll(dir) }
	if err := exportCommit(vault, revision, dir); err != nil {
		clean()
		return "", nil, err
	}
	return dir, clean, nil
}

// Read committed blobs verbatim. Checkout filters and archive export attributes must not
// change which bytes the verifier sees. The destination is a private, empty directory.
func exportCommit(vault, revision, dir string) error {
	tree, err := gitOutput(vault, "ls-tree", "--full-tree", "-rz", revision)
	if err != nil {
		return err
	}
	type entry struct {
		path, oid string
		mode      fs.FileMode
	}
	var entries []entry
	var input strings.Builder
	for _, record := range strings.Split(strings.TrimSuffix(tree, "\x00"), "\x00") {
		if record == "" {
			continue
		}
		header, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(header)
		if !ok || len(fields) != 3 || !validPRPath(path) {
			return fmt.Errorf("invalid Git tree path %q", path)
		}
		mode := fs.FileMode(0o644)
		switch fields[0] {
		case "100644":
		case "100755":
			mode = 0o755
		case "120000":
			mode = fs.ModeSymlink
		default:
			return fmt.Errorf("unsupported Git mode %s for PR snapshot path %s", fields[0], path)
		}
		entries = append(entries, entry{path, fields[2], mode})
		fmt.Fprintln(&input, fields[2])
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var links []snapshotLink
	cmd := exec.Command("git", "-C", vault, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(input.String())
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	reader := bufio.NewReader(pipe)
	extractErr := func() error {
		for _, e := range entries {
			header, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			fields := strings.Fields(header)
			if len(fields) != 3 || fields[0] != e.oid || fields[1] != "blob" {
				return fmt.Errorf("invalid Git blob response for %s", e.path)
			}
			size, err := strconv.ParseInt(fields[2], 10, 64)
			if err != nil || size < 0 {
				return fmt.Errorf("invalid Git blob size for %s", e.path)
			}
			path := filepath.FromSlash(e.path)
			if err := root.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if e.mode == fs.ModeSymlink {
				// A Git blob can be much larger than a filesystem link target.
				if size > 64<<10 {
					return fmt.Errorf("PR snapshot symlink %s has an oversized target", e.path)
				}
				target := make([]byte, int(size))
				if _, err := io.ReadFull(reader, target); err != nil {
					return err
				}
				links = append(links, snapshotLink{path: path, target: string(target)})
			} else {
				f, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, e.mode)
				if err != nil {
					return err
				}
				_, err = io.CopyN(f, reader, size)
				if err := errors.Join(err, f.Close()); err != nil {
					return err
				}
			}
			if sep, err := reader.ReadByte(); err != nil || sep != '\n' {
				return fmt.Errorf("incomplete Git blob for %s", e.path)
			}
		}
		_, err := io.Copy(io.Discard, reader)
		return err
	}()
	if extractErr != nil {
		_ = cmd.Process.Kill()
	}
	if err := errors.Join(extractErr, cmd.Wait()); err != nil {
		return err
	}
	return installSnapshotLinks(root, links)
}

type snapshotLink struct {
	path, target string
}

func installSnapshotLinks(root *os.Root, links []snapshotLink) error {
	// Resolve targets after the regular tree exists. Defer chains until their targets
	// are in place, so directory links are also created correctly on Windows.
	for len(links) > 0 {
		pending := links[:0]
		var missing error
		for _, link := range links {
			if link.target == "" || os.IsPathSeparator(link.target[0]) || filepath.VolumeName(link.target) != "" || strings.ContainsRune(link.target, 0) {
				return fmt.Errorf("PR snapshot symlink %s must have a relative target", link.path)
			}
			// Keep parent components for os.Root to resolve with symlink awareness.
			dest := filepath.Dir(link.path) + string(filepath.Separator) + link.target
			if _, err := root.Stat(dest); err != nil {
				problem := fmt.Errorf("PR snapshot symlink %s must resolve inside the snapshot: %w", link.path, err)
				if !errors.Is(err, fs.ErrNotExist) {
					return problem
				}
				pending = append(pending, link)
				if missing == nil {
					missing = problem
				}
				continue
			}
			if err := root.Symlink(filepath.FromSlash(link.target), link.path); err != nil {
				return fmt.Errorf("create PR snapshot symlink %s: %w", link.path, err)
			}
		}
		if len(pending) == len(links) {
			return missing
		}
		links = pending
	}
	return nil
}
